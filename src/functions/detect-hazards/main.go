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

type HazardDetectionRequest struct {
	Image string `json:"image"`
}

type HazardDetectionResponse struct {
	SpeechText string `json:"speechText"`
	Severity   string `json:"severity"`
}

type HazardDetection struct {
	Hazards       []Hazard `json:"hazards"`
	Severity      string   `json:"severity"`
	SafeDirection string   `json:"safe_direction"`
}

type Hazard struct {
	Position    string `json:"position"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// DetectHazards is the Cloud Function entry point
func DetectHazards(w http.ResponseWriter, r *http.Request) {
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

	logName := "detect-hazards"
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
	var req HazardDetectionRequest
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
		ResponseMIMEType: "application/json",
	}
	model.SetMaxOutputTokens(1024)

	prompt := `

	You are a navigation assistant for blind users. Your task is to analyze an image and identify any potential hazards for a blind person walking in the scene, paying special attention to objects that are directly in front of the user and centered in their field of view. This includes, but is not limited to, advertisement screens, other fixed objects, and moving objects. Your goal is to guide the user toward the safest, most comfortable, and most natural path, considering the surrounding environment and pedestrian flow.

	# Follow these rules for hazard classification:
	
	## Position-Based Categories:
	[FRONT]: 0-3 steps ahead. HIGH severity if centered, MEDIUM severity if not centered. Requires immediate attention. Direct impact on path. [LEFT/RIGHT]: Side areas. MEDIUM severity. Important for orientation. May escalate based on context.
	
	## Hazard Categories:
	### Path Obstructions:
	- HIGH Severity: Blocking fixed obstacles, fast-moving objects, construction barriers, complete path blockages, objects that are directly in front of the user and centered.
	- MEDIUM Severity: Partial blockages, slow-moving objects, temporary obstacles, side path obstacles, objects that are in front of the user but not centered.
	### Ground Conditions:
	- HIGH Severity: Open holes/manholes, missing pavement, ice patches, steep slopes (>15°).
	- MEDIUM Severity: Uneven surfaces, minor cracks, wet surfaces, moderate slopes (8-15°), stair steps.
	### Environmental Hazards:
	- HIGH Severity: Complete darkness, sudden light changes, major flooding, heavy snow coverage.
	- MEDIUM Severity: Partial shadows, light rain, wet patches, gradual light changes.
	### Proximity Hazards:
	- HIGH Severity: Unmarked drop-offs, traffic zones, water bodies, platform edges.
	- MEDIUM Severity: Marked curbs, pedestrian crossings, protected edges, side barriers, handrails.
	
	# Output Format: Return a JSON object with the following structure: 
	
	{ 
		"hazards": 
		[ 
			{ 
				"position": "[FRONT/LEFT/RIGHT]", 
				"type": "[Hazard Category]", 
				"severity": "[HIGH/MEDIUM]", 
				"description": "[Detailed description of the hazard for TTS]" 
				
			}, 
			// ... more hazards ], 
		"severity": [IF found any HIGH in hazards, then HIGH else MEDIUM, but if empty then LOW], 
		"safe_direction": "[Recommended direction for the user: LEFT, RIGHT, STRAIGHT, 'Move slightly to the [LEFT/RIGHT] to [avoid [shortened name of object in FRONT/ OPPOSITE DIRECTION or follow the pedestrian [FLOW/SIGN] ] - you can add CAUTION as prefix]], 'STOP', 'Crosswalk in front of you. Please find assistance.', 'CAUTION, Crosswalk in front of you. Proceed with caution.', 'STOP. Wait for pedestrian light.', 'Please find assistance to navigate the stairs', or a combination of these with a context with [CAUTION/STOP/SLOW] prefix if needed" 
	}
	
	 Criteria: There is no [STOP/SLOW/CAUTIOUS] in the final safe_direction. If MEDIUM then SLOW or CAUTION
	
	# Instructions: 
	Analyze the provided image. Identify all hazards present in the image based on the above classification system. For each identified hazard, create a hazard object with the correct position, type, severity, and a detailed description suitable for Text-to-Speech output. Prioritize hazards that are closer to the user's path and those that are more unpredictable or unstable. Provide detailed descriptions of each hazard, including its location relative to the user's path and the nature of the obstacle. If the hazard is a ground condition with medium severity, start the description with 'CAUTION,' followed by the detailed description. For example, 'CAUTION, Wet surface' or 'CAUTION, Uneven surface ahead.' For high-severity ground conditions, do not use the 'CAUTION' prefix.
	
	You can return only top 3 hazards
	
	## Crosswalk Handling: 
	If a crosswalk is detected directly [FRONT CENTERED] in front of the user
	
	### Pedestrian Crossing Check:
	Check if people are actively crossing the crosswalk.
	If people are crossing, set "safe_direction" to "CAUTION, Crosswalk in front of you. Proceed with caution." and skip the pedestrian light check.
	Pedestrian Light Detection: If no people are crossing, then check for the presence of a pedestrian traffic light.
	If a pedestrian light is GREEN, set "safe_direction" to "CAUTION, Crosswalk in front of you. Proceed with caution."
	If a pedestrian light is RED, set "safe_direction" to "STOP. Wait for pedestrian light."
	If NO pedestrian light is detected, set "safe_direction" to "Crosswalk in front of you. Please find assistance."
	If the crosswalk is in the front but not centered, ignore the crosswalk.
	
	## Stair Handling: 
	If stair steps are detected as a [FRONT] ground condition:
	1. **Flow Analysis:**
		 - Check for both UP and DOWN pedestrian flows
		 - Note which side (LEFT/RIGHT) people are going DOWN
		 - Note which side (LEFT/RIGHT) people are going UP
		 - If pedestrian flow exists, always follow the matching direction (DOWN flow for going down, UP flow for going up)
	
	2. **Direction-Specific Rules:**
		 For going DOWN stairs:
		 - If people going DOWN on LEFT: "CAUTION, Move to the left handrail and follow the pedestrian flow to go down the stairs."
		 - If people going DOWN on RIGHT: "CAUTION, Move to the right handrail and follow the pedestrian flow to go down the stairs."
		 - If no DOWN flow visible: "CAUTION, Move to the left handrail to go down the stairs." (default to left side)
		 - If no handrail visible: "STOP. Please find assistance to navigate down the stairs."
	
		 For going UP stairs:
		 - If people going UP on LEFT: "CAUTION, Move to the left handrail and follow the pedestrian flow to go up the stairs."
		 - If people going UP on RIGHT: "CAUTION, Move to the right handrail and follow the pedestrian flow to go up the stairs."
		 - If no UP flow visible: "CAUTION, Move to the right handrail to go up the stairs." (default to right side)
		 - If no handrail visible: "STOP. Please find assistance to navigate up the stairs."
	
	3. **Priority Rules:**
		 - Always prioritize matching the flow direction (DOWN flow for descending, UP flow for ascending)
		 - Keep to the same side as others going in your direction
		 - If flows are visible on both sides, follow conventional pattern (DOWN on left, UP on right)
		 - Default to requesting assistance if flow patterns are unclear or conflicting
	
	4. **Hazard Reporting:**
		 - Report both UP and DOWN flows as separate hazards when present
		 - Include flow direction and side in hazard descriptions
		 - Mark all stair-related hazards as MEDIUM severity
	
	
	If there is no crosswalk in front of the user, and no stairs, but there are other hazards, prioritize guiding the user to follow the natural flow of pedestrian traffic when present. When selecting a safe direction, prioritize guiding the user towards a clear and unobstructed path.
	
	## Escalator Handling:
	For escalators detected as [FRONT] path condition:
	CAUTION. Escalator ahead. Please find assistance
	
	## Elevator Handling:
	For elevators detected in [FRONT]:
	### Door States:
	
	Open: "STRAIGHT, [LEFT/RIGHT/FRONT] Elevator doors open. Move forward to enter."
	Closed: "STOP, Elevator ahead. Wait for elevator"
	Crowded: "SLOW, Crowded elevator. Wait for next or find assistance."
	
	### Location Guidance:
	
	Clear path: "STRAIGHT, Elevator entrance [X] steps forward."
	Obstructed: "SLOW, Move [slightly left/right] to reach elevator."
	Multiple elevators: "STOP, Multiple elevators. Please find assistance."
	Out of service: "STOP, Elevator out of service. Find assistance for alternate route."
	
	## Platform Priority Rules:
	
	Prioritize elevator over escalator when both present
	Default to assistance requests in unclear situations
	Consider crowd density in guidance
	Maintain right-side preference for handrails
	Include directional context for escalators
	
	## Safety Emphasis:
	
	Always mention handrail usage for moving platforms
	Provide clear waiting instructions
	Include crowd awareness
	Default to assistance in complex scenarios
	Treat stationary escalators as stairs
	
	# General Guidance:
	## Primary Rules
	When faced with obstacles on both sides: Guide user away from the most significant obstacle (FIND Pedestrian FLOW OR SIGN) Default to following pedestrian flow if it's safer Use "Move slightly to [LEFT/RIGHT]" + [shortened reason] Adjust movement magnitude based on obstacle severity/proximity
	
	## Movement Instructions
	For pedestrian [flow/sign]: Use "SLOW, Move slightly to the [LEFT/RIGHT] to follow the pedestrian [flow/sign]" + [shorten reason e.g. blocking object on the [OPPOSITE DIRECTION]] Prioritize this guidance when it provides a safe path
	For clear paths: Use "Walk straight, but be aware of obstacles on the [LEFT/RIGHT]"
	Vehicle Obstruction Protocol
	When vehicle blocks path [FRONT]: Prioritize following pedestrian flow if present This guidance takes precedence over other directions Focus on safest path around vehicle
	Safety Priorities
	For HIGH severity hazards (non-crosswalk): Prioritize "STOP" command immediately
	
	## Default/Unclear Situations
	If image is blurry or no clear hazards: Set "hazards" array to empty Set "safe_direction" to "STRAIGHT" and "severity" to "LOW"
	
	## Movement Scale Guide
	Closer obstacles = more significant sideways movement
	
	More severe obstacles = more significant sideways movement
	
	If severity is HIGH (and not a crosswalk or stairs):
	
	Extract the description of the first HIGH severity hazard.
	Prepend "STOP [shortened description]. " to the safe_direction. Shorten the description to be concise (e.g., "Open hole ahead", "[FRONT AND CENTERED] Fast moving vehicle, "Construction ahead").
	If severity is MEDIUM and there is a moving object or crosswalk or stairs in the hazards: Prepend "CAUTION, " to the safe_direction.
	If severity is MEDIUM and there is a ground hazard in the hazards: Prepend "SLOW, [shortened description] " to the safe_direction.
	Otherwise: Do not add any prefix.
	
	Example If found stairs:
	{
	"hazards": [
	{
	"position": "FRONT",
	"type": "Ground Conditions",
	"severity": "MEDIUM",
	"description": "Stair steps going down ahead."
	},
	{
	"position": "RIGHT",
	"type": "Proximity Hazard",
	"severity": "MEDIUM",
	"description": "People going down stairs on the RIGHT."
	}
	
	],
	"severity": "MEDIUM",
	"safe_direction": "SLOW, Move to the RIGHT handrail and follow the pedestrian flow to down the stairs."
	}
	
	
	Example If not found stairs:
	{
	"hazards": [
	{
	"position": "LEFT",
	"type": "Path Obstructions",
	"severity": "MEDIUM",
	"description": "A row of parked scooters is blocking the left side of the path."
	},
	{
	"position": "RIGHT",
	"type": "Path Obstructions",
	"severity": "MEDIUM",
	"description": "Stanchions and ropes are on the right side of the path."
	},
	{
	"position": "FRONT",
	"type": "Ground Conditions",
	"severity": "MEDIUM",
	"description": "CAUTION, Wet surface."
	},
	{
	"position": "FRONT",
	"type": "Ground Conditions",
	"severity": "HIGH",
	"description": "Open manhole ahead!"
	}
	],
	"severity": "HIGH",
	"safe_direction": "STOP (Open manhole ahead). Move slightly to the right - Construction barriers on the left, be aware of the wet surface"
	}
	
	Example with fast moving object:
	{
	"hazards": [
	{
	"position": "FRONT",
	"type": "Path Obstructions",
	"severity": "HIGH",
	"description": "A fast-moving bicycle is approaching from the front."
	}
	],
	"severity": "HIGH",
	"safe_direction": "STOP,  Fast moving bicycle. Move slightly to the left to avoid the bicycle."
	}
	Example with ground hazard:
	{
	"hazards": [
	{
	"position": "LEFT",
	"type": "Path Obstructions",
	"severity": "MEDIUM",
	"description": "A row of parked bicycles"
	}, 
	{
		"position": "FRONT",
		"type": "Ground Conditions",
		"severity": "MEDIUM",
		"description": "CAUTION, Wet surface."
	}
	],
	"severity": "MEDIUM",
	"safe_direction": "SLOW Wet surface. Move slightly to the left to avoid the bicycle and follow pedestrian flow."
	}	
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

	jsonStr := resp.Candidates[0].Content.Parts[0].(genai.Text)
	var detection HazardDetection
	err = json.Unmarshal([]byte(jsonStr), &detection)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error unmarshaling JSON")
		logger.Printf("Error unmarshaling JSON: %s", err.Error())
		return
	}

	// Return response
	severity := safeguardSeverity(&detection)

	response := HazardDetectionResponse{
		SpeechText: detection.SafeDirection,
		Severity:   severity,
	}

	respondWithJSON(w, http.StatusOK, response)

}

func safeguardSeverity(detection *HazardDetection) string {
	// If original severity is already HIGH, return HIGH
	if detection.Severity == "HIGH" {
		return "HIGH"
	}

	if detection.Severity == "MEDIUM" {
		return "MEDIUM"
	}

	// Convert safe direction to uppercase for case-insensitive comparison
	safeDir := strings.ToUpper(detection.SafeDirection)

	// Check for STOP - always escalates to HIGH
	if strings.HasPrefix(safeDir, "STOP") {
		return "HIGH"
	}

	// Check for CAUTION or SLOW - escalates to MEDIUM
	if strings.HasPrefix(safeDir, "CAUTION") || strings.HasPrefix(safeDir, "SLOW") {
		return "MEDIUM"
	}

	// If no special prefixes, return LOW or original severity
	return "LOW"
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
