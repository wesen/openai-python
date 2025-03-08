#!/usr/bin/env python3
"""
WebSocket server functionality for the Realtime HTTP server.
Handles WebSocket connections and audio processing.
"""
import os
import asyncio
import json
import logging
import base64
from typing import List, Dict, Any

import uvicorn
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from pydantic import BaseModel

# Import OpenAI client
from openai_client import OpenAIRealtimeClient, transcode_audio

# Configure logging
logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO").upper(),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
)
logger = logging.getLogger("realtime-http.websocket")

# Import audio utilities from the realtime example
import sys
sys.path.append(os.path.join(os.path.dirname(__file__), "../realtime"))
# These imports are used by the server even if not directly referenced
from audio_util import SAMPLE_RATE, CHANNELS  # noqa

# Add FFmpeg audio decoder
from audio_decoder import FFmpegAudioDecoder

# Create FastAPI app
app = FastAPI()

# Set up templates and static files
templates = Jinja2Templates(directory=os.path.join(os.path.dirname(__file__), "templates"))
app.mount("/static", StaticFiles(directory=os.path.join(os.path.dirname(__file__), "static")), name="static")

# WebSocket connections manager
class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []
        self.openai_clients: Dict[WebSocket, OpenAIRealtimeClient] = {}
        self.audio_formats: Dict[WebSocket, Dict[str, Any]] = {}
        self.ffmpeg_decoders: Dict[WebSocket, FFmpegAudioDecoder] = {}
        
    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket connected. Active connections: {len(self.active_connections)}")
        
        # Create a new OpenAI client for this WebSocket
        logger.info("Creating new OpenAI Realtime client")
        openai_client = OpenAIRealtimeClient(
            message_handler=lambda message: self.send_message(websocket, message)
        )
        self.openai_clients[websocket] = openai_client
        
        # Connect to OpenAI
        await openai_client.connect()

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
            
            # Disconnect OpenAI client if it exists
            if websocket in self.openai_clients:
                asyncio.create_task(self.openai_clients[websocket].disconnect())
                del self.openai_clients[websocket]
                
            # Clean up FFmpeg decoder if it exists
            if websocket in self.ffmpeg_decoders:
                self.ffmpeg_decoders[websocket].close()
                del self.ffmpeg_decoders[websocket]
                
            # Clean up audio format info
            if websocket in self.audio_formats:
                del self.audio_formats[websocket]
                
            logger.info(f"WebSocket disconnected. Active connections: {len(self.active_connections)}")
        else:
            logger.warning("Attempted to disconnect a WebSocket that wasn't in active connections")

    async def broadcast(self, message: Dict[str, Any]):
        logger.debug(f"Broadcasting message to {len(self.active_connections)} connections: {message}")
        for connection in self.active_connections:
            await connection.send_json(message)
    
    async def send_message(self, websocket: WebSocket, message: Dict[str, Any]) -> bool:
        """Send a message to a specific WebSocket client."""
        if websocket in self.active_connections:
            try:
                await websocket.send_json(message)
                return True
            except Exception as e:
                logger.error(f"Error sending message to WebSocket: {e}", exc_info=True)
                return False
        else:
            logger.warning(f"Attempted to send message to disconnected WebSocket")
            return False

manager = ConnectionManager()

# Text message model
class TextMessage(BaseModel):
    text: str

# Main route to serve the webpage
@app.get("/", response_class=HTMLResponse)
async def get_index(request: Request):
    # Get client info safely
    client_host = request.client.host if request.client else "unknown"
    logger.info(f"Serving index page to {client_host}")
    return templates.TemplateResponse("index.html", {"request": request})

# Debug endpoint to test audio format
@app.get("/debug/audio-format")
async def debug_audio_format():
    logger.info("Debug audio format endpoint accessed")
    
    response_data = {
        "status": "ok", 
        "sample_rate": SAMPLE_RATE,
        "channels": CHANNELS,
        "format": "PCM 16-bit"
    }
    
    return JSONResponse(content=response_data)

# Debug endpoint to test audio processing
@app.get("/debug/audio")
async def debug_audio():
    logger.info("Debug audio endpoint accessed")
    
    # Return information about the audio processing setup
    response_data = {
        "status": "ok",
        "audio_processing": {
            "sample_rate": SAMPLE_RATE,
            "channels": CHANNELS,
            "openai_api_key_set": bool(os.environ.get("OPENAI_API_KEY")),
            "websocket_manager": {
                "active_connections": len(manager.active_connections)
            }
        }
    }
    
    return JSONResponse(content=response_data)

# WebSocket endpoint for real-time audio streaming
@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    # Get client info safely
    client_info = getattr(websocket, 'client', None)
    client_host = getattr(client_info, 'host', 'unknown') if client_info else 'unknown'
    logger.info(f"WebSocket connection request from {client_host}")
    
    await manager.connect(websocket)
    
    # Track accumulated audio data for this connection
    accumulated_audio = bytearray()
    min_audio_size = 4800  # 100ms of audio at 24kHz, 16-bit, mono
    
    try:
        # Listen for messages from the client
        while True:
            logger.debug("Waiting for client message")
            data = await websocket.receive_text()
            message = json.loads(data)
            logger.debug(f"Received message from client: {message['type']}")
            
            if message["type"] == "audio_format":
                # Store the audio format information
                format_info = message.get("format", {})
                logger.info(f"Received audio format information: {format_info}")
                manager.audio_formats[websocket] = format_info
                
                # Create FFmpeg decoder with the appropriate format
                mime_type = format_info.get('mimeType', 'audio/mp4')
                format_name = mime_type.split('/')[-1].split(';')[0]
                
                # Initialize FFmpeg decoder for this client
                if websocket in manager.ffmpeg_decoders:
                    manager.ffmpeg_decoders[websocket].close()
                
                manager.ffmpeg_decoders[websocket] = FFmpegAudioDecoder(
                    input_format=format_name,
                    output_sample_rate=24000,
                    output_channels=1
                )
                
                # Send acknowledgment back to client
                await websocket.send_json({
                    "type": "audio_format_received",
                    "status": "ok"
                })
            
            elif message["type"] == "audio_data":
                # Process audio data with FFmpeg
                try:
                    # Get the audio data
                    audio_data = message.get("data", "")
                    logger.debug(f"Received audio data from client, length: {len(audio_data)}")
                    
                    # Decode base64 to binary
                    if ',' in audio_data:
                        audio_data = audio_data.split(',', 1)[1]
                    binary_data = base64.b64decode(audio_data)
                    
                    # Get the FFmpeg decoder for this client
                    if websocket in manager.ffmpeg_decoders:
                        decoder = manager.ffmpeg_decoders[websocket]
                        
                        # Feed the chunk to FFmpeg
                        decoder.decode_chunk(binary_data)
                        
                        # Check if we have enough data to send to OpenAI
                        if decoder.has_enough_data():
                            # Get decoded PCM data
                            pcm_data = decoder.get_decoded_audio()
                            
                            if pcm_data and len(pcm_data) > 0:
                                # Encode as base64 for OpenAI
                                transcoded_audio = base64.b64encode(pcm_data).decode('utf-8')
                                
                                # Get the OpenAI client for this WebSocket
                                if websocket in manager.openai_clients:
                                    openai_client = manager.openai_clients[websocket]
                                    # Send audio to OpenAI
                                    await openai_client.send_audio(transcoded_audio)
                                    logger.debug(f"Sent {len(pcm_data)} bytes of PCM audio to OpenAI")
                        else:
                            logger.debug("Not enough audio data accumulated yet, waiting for more")
                    else:
                        # Fallback to the old transcoding method if no decoder
                        audio_format = manager.audio_formats.get(websocket, None)
                        if audio_format:
                            transcoded_audio = transcode_audio(audio_data, audio_format)
                            
                            # Accumulate audio data
                            decoded_data = base64.b64decode(transcoded_audio)
                            accumulated_audio.extend(decoded_data)
                            
                            # Only send to OpenAI if we have enough data
                            if len(accumulated_audio) >= min_audio_size:
                                # Encode accumulated data as base64
                                accumulated_base64 = base64.b64encode(accumulated_audio).decode('utf-8')
                                
                                # Get the OpenAI client for this WebSocket
                                if websocket in manager.openai_clients:
                                    openai_client = manager.openai_clients[websocket]
                                    # Send audio to OpenAI
                                    await openai_client.send_audio(accumulated_base64)
                                    logger.debug(f"Sent {len(accumulated_audio)} bytes of accumulated PCM audio to OpenAI")
                                
                                # Clear accumulated data
                                accumulated_audio = bytearray()
                            else:
                                logger.debug(f"Accumulated {len(accumulated_audio)} bytes, need at least {min_audio_size}")
                    
                    # Send acknowledgment back to client
                    await websocket.send_json({
                        "type": "audio_data_received",
                        "status": "ok"
                    })
                except Exception as audio_error:
                    logger.error(f"Error processing audio data: {audio_error}", exc_info=True)
                    # Try to send error message to client
                    try:
                        await websocket.send_json({
                            "type": "error",
                            "message": f"Error processing audio: {str(audio_error)}"
                        })
                    except Exception:
                        pass  # Ignore if we can't send the error
            
            elif message["type"] == "audio_end":
                # Commit the audio buffer when recording stops
                try:
                    logger.info("Audio recording ended, committing buffer")
                    
                    # Get any remaining audio from the FFmpeg decoder
                    if websocket in manager.ffmpeg_decoders:
                        decoder = manager.ffmpeg_decoders[websocket]
                        
                        # Force get all available data, even if it's less than the minimum
                        with decoder.lock:
                            if len(decoder.buffer) > 0:
                                pcm_data = bytes(decoder.buffer)
                                decoder.buffer = bytearray()
                                
                                if pcm_data and len(pcm_data) > 0:
                                    # Encode as base64 for OpenAI
                                    transcoded_audio = base64.b64encode(pcm_data).decode('utf-8')
                                    
                                    # Get the OpenAI client for this WebSocket
                                    if websocket in manager.openai_clients:
                                        openai_client = manager.openai_clients[websocket]
                                        # Send audio to OpenAI
                                        await openai_client.send_audio(transcoded_audio)
                                        logger.debug(f"Sent final {len(pcm_data)} bytes of PCM audio to OpenAI")
                    
                    # Send any remaining accumulated audio
                    if len(accumulated_audio) > 0:
                        # Encode accumulated data as base64
                        accumulated_base64 = base64.b64encode(accumulated_audio).decode('utf-8')
                        
                        # Get the OpenAI client for this WebSocket
                        if websocket in manager.openai_clients:
                            openai_client = manager.openai_clients[websocket]
                            # Send audio to OpenAI
                            await openai_client.send_audio(accumulated_base64)
                            logger.debug(f"Sent final {len(accumulated_audio)} bytes of accumulated PCM audio to OpenAI")
                        
                        # Clear accumulated data
                        accumulated_audio = bytearray()
                    
                    # Get the OpenAI client for this WebSocket
                    if websocket in manager.openai_clients:
                        openai_client = manager.openai_clients[websocket]
                        await openai_client.commit_audio()
                        
                        # Send acknowledgment back to client
                        await websocket.send_json({
                            "type": "audio_end_received",
                            "status": "ok"
                        })
                    else:
                        logger.error("No OpenAI client available for this WebSocket")
                        await websocket.send_json({
                            "type": "error",
                            "message": "No OpenAI client available. Please try reconnecting."
                        })
                except Exception as commit_error:
                    logger.error(f"Error committing audio buffer: {commit_error}", exc_info=True)
                    # Try to send error message to client
                    try:
                        await websocket.send_json({
                            "type": "error",
                            "message": f"Error processing audio: {str(commit_error)}"
                        })
                    except Exception:
                        pass  # Ignore if we can't send the error
            
            elif message["type"] == "text_message":
                # Process text message
                try:
                    text = message.get("text", "")
                    logger.info(f"Received text message: {text}")
                    
                    # Get the OpenAI client for this WebSocket
                    if websocket in manager.openai_clients:
                        openai_client = manager.openai_clients[websocket]
                        
                        # Send text to OpenAI
                        await openai_client.send_text(text)
                        
                        # Send acknowledgment back to client
                        await websocket.send_json({
                            "type": "text_message_received",
                            "status": "ok"
                        })
                    else:
                        logger.error("No OpenAI client available for this WebSocket")
                        await websocket.send_json({
                            "type": "error",
                            "message": "No OpenAI client available. Please try reconnecting."
                        })
                except Exception as text_error:
                    logger.error(f"Error processing text message: {text_error}", exc_info=True)
                    # Try to send error message to client
                    try:
                        await websocket.send_json({
                            "type": "error",
                            "message": f"Error processing text: {str(text_error)}"
                        })
                    except Exception:
                        pass  # Ignore if we can't send the error
    
    except WebSocketDisconnect:
        logger.info(f"WebSocket disconnected from {client_host}")
        manager.disconnect(websocket)
    
    except Exception as e:
        logger.error(f"Error in WebSocket connection: {e}", exc_info=True)
        manager.disconnect(websocket)

# HTTP endpoint for sending text messages
@app.post("/send-text")
async def send_text(message: TextMessage):
    try:
        # This is a simple HTTP endpoint that broadcasts the message to all connected clients
        # It's not used in the current implementation but could be useful for testing
        await manager.broadcast({
            "type": "text_message",
            "text": message.text
        })
        return {"status": "ok"}
    except Exception as e:
        logger.error(f"Error sending text message: {e}", exc_info=True)
        return JSONResponse({"status": "error", "message": str(e)}, status_code=500)

# Run the server
if __name__ == "__main__":
    uvicorn.run("websocket_server:app", host="0.0.0.0", port=8000, reload=False) 