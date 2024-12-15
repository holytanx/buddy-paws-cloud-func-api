import { HttpFunction } from "@google-cloud/functions-framework";
import { handleGetDirection } from "./handler";
import { config } from "../../config";
import { validateApiKey } from "../../shared/middleware/auth-api-key";
import { validateCors } from "../../shared/cors";

export const getDirection: HttpFunction = async (
  req, 
  res
) => {
  const API_KEY = config.apis.functions.getDirection;

  // Handle CORS
  if (validateCors(req, res)) return;

  // Validate Key
  if (!validateApiKey(API_KEY)(req, res)) return;

  // Only allow POST
  if (req.method !== 'POST') {
    res.status(405).json({
      error: 'Method not allowed',
      allowedMethods: ['POST']
    });
    return;
  }

  await handleGetDirection(req, res);
  return;
};