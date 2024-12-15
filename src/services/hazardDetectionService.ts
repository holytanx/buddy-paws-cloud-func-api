import { VertexAI, InlineDataPart } from '@google-cloud/vertexai';
import { config } from '../config';
import { logger } from '../shared/utils/logger';

export class HazardDetectionService {
  private vertexAI: VertexAI;
  private model: any;
  private static readonly MAX_IMAGE_SIZE = 4 * 1024 * 1024; // 4MB

  constructor() {
    this.vertexAI = new VertexAI({
      project: config.projectId,
      location: config.location,
    });
    this.model = this.vertexAI.preview.getGenerativeModel({
      model: config.vertex.modelName
    });
  }

  private validateImageData(imageData: string): void {

    // Check if the image data is empty
    if (!imageData) {
      throw new Error('Image data is empty');
    }


    // Check image size
    const sizeInBytes = Buffer.from(imageData, 'base64').length;
    
    if (sizeInBytes > HazardDetectionService.MAX_IMAGE_SIZE) {
      throw new Error(`Image size exceeds maximum limit of ${HazardDetectionService.MAX_IMAGE_SIZE / 1024 / 1024}MB`);
    }
  }

  async detectHazardsInImage(imageData: string): Promise<string> {
    try {

      logger.info('Validating image');
      // Validate image data
      this.validateImageData(imageData);

      const hazardDetectionPrompt = `
        Guide blind. In order: 
        Action (STOP/SLOW/GO) 
        Main hazard + steps (Beware front and side blocking obstacles) 
        Signs if any (If no please tell NO PEDESTRIAN SIGN) 
        Safe path (find a safe way to walk, then tell to WALK SIDEWAYS/TURN (LEFT/RIGHT)) 
        Max 25 words 
        Noted: STOP = Immediate halt needed SLOW = Careful movement needed GO = Clear path ahead 
        Example: 'STOP. Construction barriers 2 steps ahead. Pedestrian sign left. WALK SIDEWAYS LEFT. MED.' End: HIGH/MED/LOW
      `;

      const imagePart = {
        inlineData: {
          data: imageData,
          mimeType: "image/jpeg"
        }
      } as unknown as InlineDataPart;


      logger.info('Sending request to Vertex AI');
      
      const response = await this.model.generateContent({
        contents: [{ 
          role: 'user',
          parts: [{ text: hazardDetectionPrompt }, imagePart]
        }]
      });

      

      if (!response?.response?.candidates?.[0]?.content?.parts) {
        throw new Error('Invalid response format from Vertex AI');
      }

      return response.response.candidates[0].content.parts
        .filter(part => 'text' in part)
        .map(part => (part as { text: string }).text)
        .join(' ');

    } catch (error) {
      logger.error('Error in detectHazardsInImage:', error);
      throw error;
    }
  }
}