export interface HazardDetectionRequest {
  image: string;
}

export interface HazardDetectionResponse {
  speechText: string;
  severity: string;
}
