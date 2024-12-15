import { Request, Response } from '@google-cloud/functions-framework';

export interface ApiKeyRequest extends Request {
  apiKey?: string;
}

export class ApiKeyError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ApiKeyError';
  }
}

export const validateApiKey = (validApiKey: string) => {
  return async (req: ApiKeyRequest, res: Response) => {
    const providedApiKey = req.headers['x-api-key'];

    if (!providedApiKey || providedApiKey !== validApiKey) {
      return res.status(401).json({ error: 'Invalid API key' });
    }

    return true;
  };
};