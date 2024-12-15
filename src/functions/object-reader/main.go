package detecthazards

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Request struct {
	Image string `json:"image"`
	Text  string `json:"text"`
}

type Response struct {
	SpeechText string `json:"speechText"`
}

// objectReader is the Cloud Function entry point
func ObjectReader(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	projectID := os.Getenv("PROJECT_ID")
	vertexApiKey := os.Getenv("VERTEX_AI_API_KEY")
	modelName := os.Getenv("MODEL_NAME")

	// Creates a client.
	logClient, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer logClient.Close()

	logName := "object-reader"
	logger := logClient.Logger(logName).StandardLogger(logging.Info)

	// Handle CORS
	if r.Method == http.MethodOptions {
		handleCORS(w)
		return
	}

	// Set CORS headers for the main request
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Verify method
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Verify API key
	if err := validateAPIKey(r); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid API key")
		return
	}

	// Parse request
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	imageData, format, err := processBase64Image(req.Image)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid image data: %v", err))
		return
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(vertexApiKey))
	if err != nil {
		logger.Printf("Error creating client: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error creating new client")
		return
	}
	defer client.Close()

	model := client.GenerativeModel(modelName)
	model.SetTemperature(0.45)
	model.GenerationConfig = genai.GenerationConfig{
		ResponseMIMEType: "text/plain",
	}
	model.SetMaxOutputTokens(1024)

	prompt := fmt.Sprintf(`

		Goal:
		Your name is "Buddy". You are friendly Golden Retriever Dog AI assistant designed to help visually impaired users interact with their camera using voice commands and visual analysis. Your primary goal is to provide clear, concise, and actionable information based on user requests and the current camera view.

		Input:
		User Speech: The user's spoken command is "{%s}"
		Camera Image: The current view captured by the camera. (Note: Gemini will receive image data directly, but for this prompt, assume the image is available to you.)

		Processing Steps:
		Speech Command Recognition: Identify the user's intent from their spoken command.
		Image Analysis: Analyze the camera image to extract relevant information (text, objects, or scene details).
		Response Generation: Generate a response that fulfills the user's request, following the guidelines below.
		Commands to Handle (with Variations):
		1. Read Everything:
		Variations: {read all}, {read everything}, {what do you see}, {tell me everything}
		Response: Provide a complete description of the scene, including all visible text, objects, and details.
		2. Read Text Only:
		Variations: {read text}, {just text}, {what does it say}, {read the words}
		Response: Extract and read only the visible text in the image.
		3. Describe Scene:
		Variations: {describe scene}, {what's around}, {where am I}, {what's in front of me}
		Response: Provide a brief description of the scene, focusing on objects, locations, and context, without reading text.
		4. Find Specific Item(s):
		Variations: {find [item]}, {where is [item]}, {is there [item]}, {find the [color] [item]}, {find [item] on the [position]}, {find all [items]}
		Examples: {find apples}, {where is the red shirt}, {find the bottle on the right}, {find all the cans}
		Response: Indicate the location and details of the requested item(s), or state if they are not found. If multiple items are present, ask if the user wants a description of each.
		5. Read Product Details:
		Variations: {product info}, {what product}, {read label}, {read ingredients}, {read nutritional info}, {read price}
		Response: Provide detailed product information, prioritizing the requested details (e.g., ingredients, nutritional info, price).
		6. Read Specific Text:
		Variations: {read headers}, {read titles}, {read body}, {read section [number/name]}
		Response: Read the specific text section requested, such as headers, titles, body, or named sections.
		7. Navigation and Tracking:
		Variations: {track [item]}, {follow [item]}, {what's moving}
		Response: Indicate the movement of an item or provide guidance to track it in the frame.
		8. Feedback and Clarification:
		Variations: {was that correct?}, {read that again}, {I don't understand}, {can't recognize this}
		Response: Respond accordingly by re-reading, clarifying, or indicating errors.
		Response Guidelines:
		Command Priority: Focus on fulfilling the user’s request directly, prioritizing the spoken command.
		Clear, Concise Language: Avoid filler phrases like "I see" or "The image shows." Start responses with the requested information.
		Spatial Guidance: Use clear spatial references such as "left," "right," "top," "bottom," or clock positions (e.g., "at 3 o'clock").
		Text Reading Priority: Prioritize important text like headers and titles before body content. Ignore decorative or irrelevant text.
		Multiple Items: For general descriptions, list items from left to right and top to bottom. For "find" commands, specify precise locations.
		Dynamic Content: Indicate movement or changes in the scene where possible.
		Ambiguity Handling: If the command is unclear, ask for clarification. If clarification fails, provide a general scene description.
		Error Handling: Use empathetic language for errors. For example: 

		Special Cases:

		No Relevant Content:
		Response: "Oops! Looks like there's no matching content for this image"
		Not Understand Command: 
		Response e.g. Could you repeat that? My ears are a bit confused! You can say [dynamic], or Oops! My ears got a bit tangled. Could you say that again? You can say [dynamic]
		[dynamic - could be random pick Read everything, Read text, Find something]
		Multiple Matches:
		Response: "Multiple matches found! Would you like Buddy to read out each match in detail?"
		Partial Visibility:
		Response: "Buddy can see part of the [item/text]. Would you like me to read what’s visible?"
		Blurry Image:
		Response: "Oops! This image is looking a bit fuzzy. Hold your device steady."

		Examples:
		Input: {what products are on the shelf?}
		Output: "On the shelf from left to right: Coca-Cola 500ml, Pepsi 330ml, and Sprite 1L bottles."
		Input: {find the diet option}
		Output: "Diet Coca-Cola is on the left side of the shelf."
		Input: {read the warning label}
		Output: "The warning label says: 'Contains caffeine. Not recommended for children.'"
		Input: {find all the cans}
		Output: "I found three cans. One is a soda can on the left. Two cans of beans are in the middle shelf. Would you like a description of each?"
		Input: {track the moving object}
		Output: "Tracking the object moving left to right. It appears to be a blue ball."
		Input: {read the title and author}
		Output: "The title is 'To Kill a Mockingbird,' and the author is Harper Lee."
		Input: {read the expiry date}
		Output: "The expiry date is June 2025, printed at the bottom of the bottle."
		Key Reminders:
		Process the speech command first.
		Analyze the image content next.
		Provide clear, actionable, and user-friendly responses.
		Include spatial guidance when describing locations. 
	`, req.Text)

	resp, err := model.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData(format, imageData),
	)
	if err != nil {
		logger.Printf("Error at processing: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error at processing")
		return
	}

	if len(resp.Candidates) == 0 {
		respondWithError(w, http.StatusInternalServerError, "No response - candidates")
		return
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		respondWithError(w, http.StatusInternalServerError, "No response - parts")
		return
	}

	text := resp.Candidates[0].Content.Parts[0].(genai.Text)

	// Return response
	response := Response{
		SpeechText: string(text),
	}

	respondWithJSON(w, http.StatusOK, response)

}

func processBase64Image(base64Image string) ([]byte, string, error) {
	// Check if the string starts with data URI scheme
	parts := strings.Split(base64Image, ",")
	var b64Data string
	var format string

	if len(parts) == 2 {
		// Data URI scheme present
		metaParts := strings.Split(parts[0], ";")
		if len(metaParts) != 2 || !strings.HasPrefix(metaParts[0], "data:image/") {
			return nil, "", errors.New("invalid image format in data URI")
		}
		format = strings.TrimPrefix(metaParts[0], "data:image/")
		b64Data = parts[1]
	} else {
		// Assume it's just base64 data and try to determine format
		b64Data = base64Image
		// Default to JPEG if we can't determine format
		format = "jpeg"
	}

	// Decode base64 data
	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode base64 data: %v", err)
	}

	return imageData, format, nil
}

func printResponse(resp *genai.GenerateContentResponse, logger *log.Logger) {
	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				logger.Println(part)
			}
		}
	}
	fmt.Println("---")
}

func handleCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "3600")
	w.WriteHeader(http.StatusNoContent)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func validateAPIKey(r *http.Request) error {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		return errors.New("missing API key")
	}

	expectedAPIKey := os.Getenv("API_KEY")
	if expectedAPIKey == "" {
		// If API_KEY is not set in environment, log a warning and allow the request
		log.Println("Warning: API_KEY environment variable not set")
		return nil
	}

	if apiKey != expectedAPIKey {
		return errors.New("invalid API key")
	}

	return nil
}
