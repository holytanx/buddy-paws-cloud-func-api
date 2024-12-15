import { logger } from "../shared/utils/logger";
import { config } from "../config";
import { Coordinates } from "../functions/getDirection/types";
import axios from "axios";

export class GetDirectionService {
  private apiKey: string;

  constructor() {
    this.apiKey = config.apis.google.directions;
  }

  async getDirections(origin: Coordinates, destination: Coordinates, mode: string): Promise<any> {
    try {
      
      const url = `https://maps.googleapis.com/maps/api/directions/json`;

      const originParam = encodeURI(`${origin.latitude},${origin.longitude}`);
      const destinationParam = encodeURI(`${destination.latitude},${destination.longitude}`);

      const config = {
        params: {
          destination: destinationParam,
          origin: originParam,
          key: this.apiKey,
          mode: mode,
          language: 'en'
        }
      }

      const response = await axios.get<google.maps.DirectionsResult>(url, config);
      
      return response.data;
    } catch (error) {
      if (axios.isAxiosError(error)) {
        throw new Error(`Directions API error: ${error.response?.data?.message || error.message}`);
      }
      throw error;
    }
  }

}

