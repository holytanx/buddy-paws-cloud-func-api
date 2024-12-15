import { Logging } from '@google-cloud/logging';

class Logger {
  private logging: Logging;
  private logName: string;

  constructor() {
    this.logging = new Logging();
    this.logName = 'cloud-functions-log';
  }

  private async writeLog(severity: string, message: string, data?: any) {
    const log = this.logging.log(this.logName);
    const metadata = {
      resource: {
        type: 'cloud_function',
        labels: {
          function_name: process.env.FUNCTION_TARGET || 'unknown',
          region: process.env.FUNCTION_REGION || 'unknown'
        }
      },
      severity: severity
    };

    const entry = log.entry(metadata, {
      message: message,
      timestamp: new Date(),
      data: data || {}
    });

    try {
      await log.write(entry);
    } catch (error) {
      console.error('Error writing to Cloud Logging:', error);
      // Fallback to console
      console.log(`${severity}: ${message}`, data || '');
    }
  }

  public info(message: string, data?: any) {
    this.writeLog('INFO', message, data);
  }

  public warn(message: string, data?: any) {
    this.writeLog('WARNING', message, data);
  }

  public error(message: string, data?: any) {
    this.writeLog('ERROR', message, data);
  }

  public debug(message: string, data?: any) {
    if (process.env.NODE_ENV !== 'production') {
      this.writeLog('DEBUG', message, data);
    }
  }
}

export const logger = new Logger();

// Usage example:
/*
logger.info('Processing request', { requestId: '123' });
logger.error('Failed to process image', { error: error.message });
logger.debug('Debug data', { someData: 'value' });
*/