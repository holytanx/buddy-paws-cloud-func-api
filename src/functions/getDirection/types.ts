export interface GetDirectionRequest {
  origin: Coordinates;
  destination: Coordinates;
  mode?: string;
}

export interface Coordinates {
  latitude: number;
  longitude: number;
}
