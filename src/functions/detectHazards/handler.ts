import { Request, Response } from '@google-cloud/functions-framework';
import { HazardDetectionResponse } from './types';
import { logger } from '../../shared/utils/logger';
import { HazardDetectionService } from '../../services/hazardDetectionService';

export async function handleDetectHazards(req: Request, res: Response) {
  try {
    logger.info('Processing hazard detection request');

    // Validate request body
    if (!req.body) {
      return res.status(400).json({
        error: 'Missing request body',
        timestamp: new Date().toISOString()
      });
    }

    
    // Validate image data
    if (!req.body.image) {
      return res.status(400).json({
        error: 'Missing image data in request body',
        timestamp: new Date().toISOString()
      });
    }

    // Process image for hazards
    const hazardService = new HazardDetectionService();
    
    logger.info('Detecting hazards in image');
    const analysisText = await hazardService.detectHazardsInImage(req.body.image);
    
    if (!analysisText) {
      return res.status(500).json({
        error: 'No analysis result from Vertex AI',
        timestamp: new Date().toISOString()
      });
    }

    // Process hazard results
    const result = HazardResponseProcessor.processHazardAnalysis(analysisText);
    
    logger.info('Successfully processed hazard detection', { severity: result.severity });
    return res.status(200).json(result);

  } catch (error) {
    logger.error('Error processing image for hazards:', {
      error: error instanceof Error ? error.message : 'Unknown error',
      stack: error instanceof Error ? error.stack : undefined
    });
    
    return res.status(500).json({
      error: 'Error processing image for hazard detection',
      details: error instanceof Error ? error.message : 'Unknown error',
      timestamp: new Date().toISOString()
    });
  }
}

export class HazardResponseProcessor {
  static processHazardAnalysis(analysis: string): HazardDetectionResponse {
    const regex = /(HIGH|MED|LOW)[.\s]*$/i;
    
    try {
      const severityMatch = analysis.match(regex);
      const severity = severityMatch ? 
        severityMatch[1].toUpperCase() as HazardDetectionResponse['severity'] : 
        'MED';

      const speechText = analysis
        .replace(regex, '')
        .trim()
        .replace(/\s+/g, ' ');
      
      if (!speechText) {
        logger.warn('Empty speech text after processing');
        return { 
          speechText: 'Unable to analyze image properly', 
          severity: 'MED' 
        };
      }

      return { speechText, severity };
    } catch (error) {
      logger.error('Error processing hazard analysis:', error);
      return { 
        speechText: 'Error processing hazard analysis', 
        severity: 'MED' 
      };
    }
  }
}