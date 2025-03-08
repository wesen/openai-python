#!/usr/bin/env python3
"""
Main server application for the OpenAI Realtime HTTP server.
This file serves as the entry point for the application.
"""
import os
import logging
import uvicorn

# Configure logging
logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO").upper(),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
)
logger = logging.getLogger("realtime-http")

# Import the WebSocket server app
from websocket_server import app

# Run the server
if __name__ == "__main__":
    logger.info("Starting OpenAI Realtime HTTP server")
    uvicorn.run(app, host="0.0.0.0", port=8000, reload=False) 