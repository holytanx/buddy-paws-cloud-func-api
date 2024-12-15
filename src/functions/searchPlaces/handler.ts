import { Request, Response } from '@google-cloud/functions-framework';
import { logger } from '../../shared/utils/logger';
import { SearchPleacesRequest } from './types';
import { SearchPlacesService } from '../../services/searchPlacesService';

export async function handleSearchPlaces(req: Request, res: Response) {
  try {
    logger.info('Processing search places request');

    if (!req.body) {
      return res.status(400).json({
        error: 'Missing request body',
        timestamp: new Date().toISOString()
      });
    }
    

    logger.info(`body: ${JSON.stringify(req.body)}`)

    if (!req.body.textQuery || !req.body.currentCoordinates) {
      return res.status(400).json({
        error: 'Missing textQuery or currentCoordinates in request body',
        timestamp: new Date().toISOString()
      });
    }

    const { textQuery, 
      currentCoordinates, 
      radius = 10000.0, 
      fields = 'places.displayName,places.formattedAddress,places.location', 
      rankPreference = 'DISTANCE', 
    } = req.body as SearchPleacesRequest;



    const searchPlaceService = new SearchPlacesService();
    
    logger.info('Searching places');
    const result = await searchPlaceService.searchPlaces(textQuery, 
      currentCoordinates, 
      radius, 
      fields, 
      rankPreference
    );    
    
    logger.info('Successfully searching places');
    return res.status(200).json({places: result});

  } catch (error) {
    logger.error('Error searching places:', {
      error: error instanceof Error ? error.message : 'Unknown error',
      stack: error instanceof Error ? error.stack : undefined
    });
    
    return res.status(500).json({
      error: 'Error searching places',
      details: error instanceof Error ? error.message : 'Unknown error',
      timestamp: new Date().toISOString()
    });
  }
}
