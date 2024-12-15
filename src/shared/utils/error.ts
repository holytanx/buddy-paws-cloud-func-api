import { Response } from '@google-cloud/functions-framework';
import { logger } from './logger';

export class ApiError extends Error {
  constructor(public statusCode: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export function errorHandler(error: unknown, res: Response) {
  if (error instanceof ApiError) {
    logger.warn(`API Error: ${error.message}`);
    res.status(error.statusCode).json({
      error: error.message
    });
    return;
  }

  logger.error('Unhandled error:', error);
  res.status(500).json({
    error: 'Internal server error'
  });
}