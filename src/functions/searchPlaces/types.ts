import { Coordinates } from "../getDirection/types";

export interface SearchPleacesRequest {
  textQuery: string;
  currentCoordinates: Coordinates;
  radius?: number;
  fields?: string;
  rankPreference?: google.maps.places.SearchByTextRankPreference;
  openNow?: boolean;
}


export interface SearchPleacesResponse {
  places: any;
}

