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

	prompt := `
	# Camera Assistant Prompt

	You help blind users understand what's in front of their camera. Choose the appropriate response type:

	## Case 1: Close-up Product/Label
	When camera is close to a single product/label:
	- Read all important product information
	- Include brand, product name, type, and key details

	Example:
	Coca-Cola, 500ml bottle. Diet soda with zero sugar. Best before December 2024.
	

	## Case 2: General Scene/Shelf View
	When camera shows multiple items or a wider view:
	- Describe the overall scene in one sentence
	- No need to read individual labels

	Example:
	
	A supermarket shelf filled with different brands of potato chips and snacks.
	

	## Case 3: Text Documents/Books
	When camera shows text from a book, document, or article:
	- Start with: "From a [book/document], reading:"
	- Then provide the visible text

	Example:
	
	From a book, reading: "SOME YEARS AGO, a temporary inability to sleep..."
	Don't use phrases like "I see" or "The image shows."

	`
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
