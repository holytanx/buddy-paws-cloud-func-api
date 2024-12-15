import { config } from "../config";
import axios from "axios";
import { SearchPleacesResponse } from "@/functions/searchPlaces/types";
import { Coordinates } from "@/functions/getDirection/types";

export class SearchPlacesService {
  private apiKey: string;

  constructor() {
    this.apiKey = config.apis.google.places;
  }

  async searchPlaces(
    textQuery: string,
    currentCoordinates: Coordinates, 
    radius: number, 
    fields: string, 
    rankPreference: string
  ): Promise<SearchPleacesResponse> {
    try {

      const url = `https://places.googleapis.com/v1/places:searchText`;
      const config = {
        headers: {
          'Content-Type': 'application/json',
          'X-Goog-FieldMask': fields,
          'X-Goog-Api-Key': this.apiKey,
          'language': 'en'
        }
      }
      const body = {
        textQuery: textQuery,
        locationBias: {
          circle: {
            center: {
              latitude: currentCoordinates.latitude,
              longitude: currentCoordinates.longitude
            },
            radius: radius
          }
        },
        rankPreference: rankPreference
      }

      const response = await axios.post<any>(url, 
        body, 
        config
      );

      let places = response.data.places;
      if (places.length > 4) {
        places = places.slice(0, 4);
      }

      // Then get distances for each place
      let destinations = places.map(place => ({
        lat: place.location.latitude,
        lng: place.location.longitude
      }));

      const distanceMatrixUrl = `https://maps.googleapis.com/maps/api/distancematrix/json`;
      const origin = `${currentCoordinates.latitude},${currentCoordinates.longitude}`;
      
      const distanceResponse = await axios.get(distanceMatrixUrl, {
        params: {
          origins: origin,
          destinations: destinations.map(dest => `${dest.lat},${dest.lng}`).join('|'),
          mode: 'walking',
          key: this.apiKey
        }
      });

      // Combine search results with distance information
      return places.map((place, index) => ({
        ...place,
        distance: distanceResponse.data.rows[0].elements[index].distance,
        duration: distanceResponse.data.rows[0].elements[index].duration
      }));
      
      // return response.data;

    } catch (error) {
      if (axios.isAxiosError(error)) {
        throw new Error(`Search Places API error: ${error.response?.data?.message || error.message}`);
      }
      throw error;
    }
  }

}

