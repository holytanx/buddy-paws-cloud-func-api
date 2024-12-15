import { handleDetectHazards } from './handler';
import { HttpFunction } from '@google-cloud/functions-framework';
import { validateApiKey } from '../../shared/middleware/auth-api-key';
import { config } from '../../config';
import { validateCors } from '../../shared/cors';


export const detectHazards: HttpFunction = async (
  req, 
  res
) => {
  const API_KEY = config.apis.functions.detectHazards;


  // Handle CORS
  if (validateCors(req, res)) return;

  // Validate Key
  if (!validateApiKey(API_KEY)(req,res)) return;

  // Only allow POST
  if (req.method !== 'POST') {
    res.status(405).json({
      error: 'Method not allowed',
      allowedMethods: ['POST']
    });
    return;
  }

  await handleDetectHazards(req, res);
  return;
};