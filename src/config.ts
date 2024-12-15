export const config = {
  projectId: process.env.PROJECT_ID || 'ai-for-impact-poc',
  location: process.env.LOCATION || 'asia-southeast1',
  vertex: {
    modelName: process.env.MODEL_NAME || 'gemini-1.5-flash'
  },
  apis: {
    // Cloud Run Function API Keys
    functions: {
      searchPlaces: process.env.SEARCH_PLACES_FUNCTION_API_KEY || 'your-places-api-key',
      getDirection: process.env.GET_DIRECTION_FUNCTION_API_KEY || 'your-directions-api-key',
      detectHazards: process.env.DETECT_HAZARDS_FUNCTION_API_KEY || 'your-vertex-api-key'
    },
    google: {
      places: process.env.GOOGLE_PLACES_API_KEY,
      directions: process.env.GOOGLE_DIRECTIONS_API_KEY,
      vertex: process.env.GOOGLE_VERTEX_API_KEY
    }
  }
};