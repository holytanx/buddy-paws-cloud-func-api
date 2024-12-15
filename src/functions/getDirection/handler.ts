import { Request, Response } from '@google-cloud/functions-framework';
import { GetDirectionService } from '../../services/getDirectionService';
import { logger } from '../../shared/utils/logger';
import { GetDirectionRequest } from './types';

export async function handleGetDirection(req: Request, res: Response) {
  try {
    logger.info('Processing get direction request');

    if (!req.body) {
      return res.status(400).json({
        error: 'Missing request body',
        timestamp: new Date().toISOString()
      });
    }
    

    logger.info(`body: ${JSON.stringify(req.body)}`)

    if (!req.body.destination || !req.body.origin) {
      return res.status(400).json({
        error: 'Missing destination or origin in request body',
        timestamp: new Date().toISOString()
      });
    }

    const { origin, destination, mode = 'walking' } = req.body as GetDirectionRequest;
    logger.info(`origin: ${JSON.stringify(origin)}`)
    logger.info(`destination: ${JSON.stringify(destination)}`)
    logger.info(`mode: ${mode}`)


    const getDirectionService = new GetDirectionService();
    
    logger.info('Getting direction');
    const directions = await getDirectionService.getDirections(origin, destination, mode);
    
  
    logger.info('Successfully get direction');
    return res.status(200).json(directions);

  } catch (error) {
    logger.error('Error getting direction:', {
      error: error instanceof Error ? error.message : 'Unknown error',
      stack: error instanceof Error ? error.stack : undefined
    });
    
    return res.status(500).json({
      error: 'Error getting direction',
      details: error instanceof Error ? error.message : 'Unknown error',
      timestamp: new Date().toISOString()
    });
  }
}
