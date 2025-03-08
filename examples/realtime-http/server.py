#!/usr/bin/env python3
"""
HTTP server app that serves a webpage for real-time audio streaming with WebAudio
and provides an endpoint for sending text to the backend.
"""
import os
import asyncio
import json
import logging
from typing import List, Dict, Any, cast, Optional

import base64
import uvicorn
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from pydantic import BaseModel

from openai import AsyncOpenAI
from openai.resources.beta.realtime.realtime import AsyncRealtimeConnection

# Configure logging
logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO").upper(),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
)
logger = logging.getLogger("realtime-http")

# Import audio utilities from the realtime example
import sys
sys.path.append(os.path.join(os.path.dirname(__file__), "../realtime"))
# These imports are used by the server even if not directly referenced
from audio_util import SAMPLE_RATE, CHANNELS  # noqa

# Create FastAPI app
app = FastAPI()

# Set up templates and static files
templates = Jinja2Templates(directory=os.path.join(os.path.dirname(__file__), "templates"))
app.mount("/static", StaticFiles(directory=os.path.join(os.path.dirname(__file__), "static")), name="static")

# OpenAI client
client = AsyncOpenAI()
logger.info(f"AsyncOpenAI client initialized")

# Add FFmpeg audio decoder
import subprocess
import threading
import io

class FFmpegAudioDecoder:
    def __init__(self, input_format: str = 'mp4', output_sample_rate: int = 24000, output_channels: int = 1):
        self.logger = logging.getLogger("ffmpeg-decoder")
        self.input_format = input_format
        self.output_sample_rate = output_sample_rate
        self.output_channels = output_channels
        self.process: Optional[subprocess.Popen] = None
        self.buffer = bytearray()
        self.lock = threading.Lock()
        # Calculate minimum buffer size for 100ms of audio
        # 24000 samples/sec * 2 bytes/sample * 0.1 sec = 4800 bytes
        self.min_buffer_size = int(output_sample_rate * 2 * 0.1)  # 100ms of audio
        self.logger.info(f"Minimum buffer size for 100ms: {self.min_buffer_size} bytes")
        self.start_process()
        
    def start_process(self):
        """Start the FFmpeg process for audio decoding"""
        self.logger.info(f"Starting FFmpeg process: {self.input_format} -> PCM")
        
        self.process = subprocess.Popen([
            'ffmpeg',
            '-hide_banner',
            '-loglevel', 'error',
            '-protocol_whitelist', 'pipe,udp,rtp',
            '-f', self.input_format,
            '-i', 'pipe:0',
            '-f', 's16le',  # 16-bit signed little-endian PCM
            '-ar', str(self.output_sample_rate),
            '-ac', str(self.output_channels),
            'pipe:1'
        ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        
        # Start a thread to continuously read from FFmpeg's stdout
        self.read_thread = threading.Thread(target=self._read_output, daemon=True)
        self.read_thread.start()
    
    def _read_output(self):
        """Continuously read from FFmpeg's stdout"""
        while self.process and self.process.poll() is None:
            try:
                # Read in small chunks to reduce latency
                data = self.process.stdout.read(4096)
                if data:
                    with self.lock:
                        self.buffer.extend(data)
            except Exception as e:
                self.logger.error(f"Error reading from FFmpeg: {e}")
                break
    
    def decode_chunk(self, chunk_data):
        """Feed a chunk of MP4 data to FFmpeg"""
        if not self.process or self.process.poll() is not None:
            self.logger.warning("FFmpeg process not running, restarting...")
            self.start_process()
            
        try:
            # Write the chunk to FFmpeg's stdin
            self.process.stdin.write(chunk_data)
            self.process.stdin.flush()
        except Exception as e:
            self.logger.error(f"Error feeding data to FFmpeg: {e}")
            # Restart the process on error
            self.start_process()
    
    def get_decoded_audio(self, max_size: Optional[int] = None) -> Optional[bytes]:
        """Get decoded PCM audio from the buffer if enough data is available"""
        with self.lock:
            buffer_size = len(self.buffer)
            
            # If buffer is smaller than minimum required size, return None
            if buffer_size < self.min_buffer_size:
                self.logger.debug(f"Buffer too small ({buffer_size} bytes), need at least {self.min_buffer_size} bytes")
                return None
                
            if max_size and buffer_size > max_size:
                # Return a portion of the buffer
                result = bytes(self.buffer[:max_size])
                self.buffer = self.buffer[max_size:]
                self.logger.debug(f"Returning {len(result)} bytes, {len(self.buffer)} bytes left in buffer")
            else:
                # Return the entire buffer
                result = bytes(self.buffer)
                self.buffer = bytearray()
                self.logger.debug(f"Returning entire buffer: {len(result)} bytes")
                
        return result
    
    def has_enough_data(self) -> bool:
        """Check if the buffer has enough data to meet minimum requirements"""
        with self.lock:
            return len(self.buffer) >= self.min_buffer_size
    
    def close(self):
        """Clean up resources"""
        if self.process:
            try:
                self.process.stdin.close()
                self.process.terminate()
                self.process.wait(timeout=2)
            except:
                self.process.kill()
            finally:
                self.process = None

# Audio transcoding function
def transcode_audio(base64_audio: str, audio_format: Dict[str, Any] = None) -> str:
    """
    Transcode audio data to the format expected by OpenAI (mono PCM16 at 24kHz).
    
    Args:
        base64_audio: Base64-encoded audio data from the browser
        audio_format: Dictionary containing audio format information
        
    Returns:
        Base64-encoded audio data in the format expected by OpenAI
    """
    try:
        # The browser sends base64 data that might include a data URL prefix
        # Remove the prefix if it exists
        if ',' in base64_audio:
            base64_audio = base64_audio.split(',', 1)[1]
        
        # Decode base64 to binary
        binary_data = base64.b64decode(base64_audio)
        
        # For debugging
        logger.debug(f"Binary data length: {len(binary_data)} bytes")
        
        # Log audio format information if available
        if audio_format:
            logger.info(f"Audio format: {audio_format}")
            
            # Check if we need to convert the audio format
            mime_type = audio_format.get('mimeType', '')
            sample_rate = audio_format.get('sampleRate', 24000)
            channels = audio_format.get('channels', 1)
            
            if mime_type and mime_type != 'audio/pcm':
                logger.info(f"Converting from {mime_type} to PCM16")
                
                # Use FFmpeg for decoding instead of pydub
                try:
                    # Extract format from mime type
                    format_name = mime_type.split('/')[-1].split(';')[0]
                    if format_name == 'webm':
                        format_name = 'webm'
                    elif format_name == 'ogg':
                        format_name = 'ogg'
                    elif format_name == 'mp4':
                        format_name = 'mp4'
                    else:
                        format_name = 'mp4'  # Default to mp4 if unknown
                    
                    # Create FFmpeg process for one-time conversion
                    ffmpeg_process = subprocess.Popen([
                        'ffmpeg',
                        '-hide_banner',
                        '-loglevel', 'error',
                        '-f', format_name,
                        '-i', 'pipe:0',
                        '-f', 's16le',
                        '-ar', '24000',
                        '-ac', '1',
                        'pipe:1'
                    ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                    
                    # Send data to FFmpeg
                    stdout_data, stderr_data = ffmpeg_process.communicate(input=binary_data)
                    
                    if ffmpeg_process.returncode != 0:
                        logger.error(f"FFmpeg error: {stderr_data.decode('utf-8', errors='ignore')}")
                        # Fall back to returning the original data
                        return base64_audio
                    
                    # Encode the PCM data as base64
                    return base64.b64encode(stdout_data).decode('utf-8')
                    
                except Exception as e:
                    logger.error(f"Error using FFmpeg for transcoding: {e}")
                    # Fall back to returning the original data
                    return base64_audio
                    
        # If no conversion needed or conversion failed, return the original data
        return base64_audio
        
    except Exception as e:
        logger.error(f"Error in transcode_audio: {e}")
        # Return the original data on error
        return base64_audio

# WebSocket connections manager
class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []
        self.openai_connections: Dict[WebSocket, AsyncRealtimeConnection] = {}
        self.connection_tasks: Dict[WebSocket, asyncio.Task[None]] = {}
        self.audio_formats: Dict[WebSocket, Dict[str, Any]] = {}
        self.ffmpeg_decoders: Dict[WebSocket, FFmpegAudioDecoder] = {}
        
    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)
        logger.info(f"WebSocket connected. Active connections: {len(self.active_connections)}")
        
        # Create a new OpenAI connection for this WebSocket
        logger.info("Creating new OpenAI Realtime connection")
        connection_task = asyncio.create_task(self.manage_openai_connection(websocket))
        self.connection_tasks[websocket] = connection_task

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)
            
            # Cancel the connection task if it exists
            if websocket in self.connection_tasks:
                self.connection_tasks[websocket].cancel()
                del self.connection_tasks[websocket]
                
            # Remove OpenAI connection if it exists
            if websocket in self.openai_connections:
                del self.openai_connections[websocket]
                
            logger.info(f"WebSocket disconnected. Active connections: {len(self.active_connections)}")
        else:
            logger.warning("Attempted to disconnect a WebSocket that wasn't in active connections")

    async def broadcast(self, message: Dict[str, Any]):
        logger.debug(f"Broadcasting message to {len(self.active_connections)} connections: {message}")
        for connection in self.active_connections:
            await connection.send_json(message)
    
    async def manage_openai_connection(self, websocket: WebSocket):
        """Manage the OpenAI connection for this WebSocket."""
        try:
            # Use the proper async context manager pattern
            async with client.beta.realtime.connect(model="gpt-4o-realtime-preview") as conn:
                logger.info("OpenAI Realtime connection established")
                
                # Store the connection
                self.openai_connections[websocket] = conn
                
                # Set up server-side VAD (Voice Activity Detection)
                logger.debug("Setting up server-side VAD")
                await conn.session.update(session={"turn_detection": {"type": "server_vad"}})
                
                # Start listening for events from OpenAI
                await self.handle_openai_events(conn, websocket)
        except asyncio.CancelledError:
            logger.info("OpenAI connection task cancelled")
        except Exception as e:
            logger.error(f"Error in OpenAI connection: {e}", exc_info=True)
            try:
                await websocket.send_json({
                    "type": "error",
                    "message": f"OpenAI connection error: {str(e)}"
                })
            except Exception:
                pass  # Ignore if we can't send the error
    
    async def handle_openai_events(self, conn: AsyncRealtimeConnection, websocket: WebSocket):
        """Handle events from OpenAI."""
        last_audio_item_id = None
        accumulated_text = {}
        
        try:
            logger.info("Starting to listen for OpenAI events")
            async for event in conn:
                logger.debug(f"Received OpenAI event: {event.type}")
                
                if event.type == "session.created":
                    # Extract session ID safely from the event
                    logger.info("Session created event received")
                    session_data = event.model_dump() if hasattr(event, 'model_dump') else vars(event)
                    session_id = ""
                    
                    # Navigate the session data structure to find the ID
                    if 'session' in session_data and session_data['session']:
                        session = session_data['session']
                        if isinstance(session, dict) and 'id' in session:
                            session_id = session['id']
                            logger.info(f"Session ID: {session_id}")
                        else:
                            logger.warning("Session object doesn't contain 'id' field")
                    else:
                        logger.warning("Event doesn't contain 'session' field")
                    
                    await websocket.send_json({
                        "type": "session.created", 
                        "session_id": session_id
                    })
                
                elif event.type == "session.updated":
                    # Extract session data safely
                    logger.debug("Session updated event received")
                    session_data = {}
                    if hasattr(event, 'session'):
                        if hasattr(event.session, 'model_dump'):
                            session_data = event.session.model_dump()
                        elif isinstance(event.session, dict):
                            session_data = event.session
                    
                    await websocket.send_json({
                        "type": "session.updated", 
                        "session": session_data
                    })
                
                elif event.type == "response.audio.delta":
                    # Send audio data to the client
                    if event.item_id != last_audio_item_id:
                        logger.debug(f"New audio item: {event.item_id}")
                        last_audio_item_id = event.item_id
                        
                        # Notify client about new audio stream starting
                        try:
                            await websocket.send_json({
                                "type": "audio_stream_start",
                                "item_id": event.item_id
                            })
                        except Exception as e:
                            logger.error(f"Error sending audio stream start notification: {e}", exc_info=True)
                    
                    # Log audio data details
                    audio_data_length = len(event.delta) if hasattr(event, 'delta') else 0
                    logger.debug(f"Sending audio delta to client, data length: {audio_data_length}")
                    
                    # Check if delta is empty or None
                    if not event.delta:
                        logger.warning(f"Empty audio delta received for item {event.item_id}")
                        continue  # Skip sending empty audio data
                    
                    try:
                        # Send audio data to the client
                        await websocket.send_json({
                            "type": "audio_data",
                            "item_id": event.item_id,
                            "data": event.delta
                        })
                        logger.debug(f"Successfully sent audio data of length {audio_data_length}")
                    except Exception as e:
                        logger.error(f"Error sending audio data: {e}", exc_info=True)
                
                elif event.type == "response.audio_transcript.delta":
                    # Accumulate transcript text
                    item_id = event.item_id
                    if item_id not in accumulated_text:
                        accumulated_text[item_id] = ""
                    
                    accumulated_text[item_id] += event.delta
                    
                    # Send the transcript to the client
                    try:
                        await websocket.send_json({
                            "type": "transcript",
                            "item_id": item_id,
                            "text": accumulated_text[item_id]
                        })
                    except Exception as e:
                        logger.error(f"Error sending transcript: {e}", exc_info=True)
                
                elif event.type == "response.text.delta":
                    # Send text delta to the client
                    try:
                        await websocket.send_json({
                            "type": "text_delta",
                            "delta": event.delta
                        })
                    except Exception as e:
                        logger.error(f"Error sending text delta: {e}", exc_info=True)
                
                elif event.type == "response.done":
                    # Response is complete
                    logger.info("Response completed")
                    try:
                        await websocket.send_json({
                            "type": "response.done"
                        })
                    except Exception as e:
                        logger.error(f"Error sending response done notification: {e}", exc_info=True)
                
                elif event.type == "error":
                    # Handle error events - properly extract the nested error message
                    error_message = "Unknown error"
                    
                    # Try to extract the error message from the event
                    try:
                        # Convert the event to a dictionary for easier access
                        event_dict = {}
                        if hasattr(event, 'model_dump'):
                            event_dict = event.model_dump()
                        else:
                            event_dict = vars(event)
                        
                        # Extract error message from the dictionary
                        if 'error' in event_dict and isinstance(event_dict['error'], dict):
                            error_dict = event_dict['error']
                            if 'message' in error_dict:
                                error_message = error_dict['message']
                        elif hasattr(event, 'message'):
                            error_message = str(event.message)
                    except Exception as extract_error:
                        logger.error(f"Error extracting error message: {extract_error}", exc_info=True)
                    
                    logger.error(f"Error event from OpenAI: {error_message}")
                    try:
                        await websocket.send_json({
                            "type": "error",
                            "message": error_message
                        })
                    except Exception as e:
                        logger.error(f"Error sending error notification: {e}", exc_info=True)
        
        except Exception as e:
            logger.error(f"Error handling OpenAI events: {e}", exc_info=True)
            try:
                await websocket.send_json({
                    "type": "error",
                    "message": f"Error handling OpenAI events: {str(e)}"
                })
            except Exception:
                pass  # Ignore if we can't send the error

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
                                
                                # Get the OpenAI connection for this WebSocket
                                if websocket in manager.openai_connections:
                                    conn = manager.openai_connections[websocket]
                                    # Append to the audio buffer
                                    await conn.input_audio_buffer.append(audio=transcoded_audio)
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
                                
                                # Get the OpenAI connection for this WebSocket
                                if websocket in manager.openai_connections:
                                    conn = manager.openai_connections[websocket]
                                    # Append to the audio buffer
                                    await conn.input_audio_buffer.append(audio=accumulated_base64)
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
                                    
                                    # Get the OpenAI connection for this WebSocket
                                    if websocket in manager.openai_connections:
                                        conn = manager.openai_connections[websocket]
                                        # Append to the audio buffer
                                        await conn.input_audio_buffer.append(audio=transcoded_audio)
                                        logger.debug(f"Sent final {len(pcm_data)} bytes of PCM audio to OpenAI")
                    
                    # Send any remaining accumulated audio
                    if len(accumulated_audio) > 0:
                        # Encode accumulated data as base64
                        accumulated_base64 = base64.b64encode(accumulated_audio).decode('utf-8')
                        
                        # Get the OpenAI connection for this WebSocket
                        if websocket in manager.openai_connections:
                            conn = manager.openai_connections[websocket]
                            # Append to the audio buffer
                            await conn.input_audio_buffer.append(audio=accumulated_base64)
                            logger.debug(f"Sent final {len(accumulated_audio)} bytes of accumulated PCM audio to OpenAI")
                        
                        # Clear accumulated data
                        accumulated_audio = bytearray()
                    
                    # Get the OpenAI connection for this WebSocket
                    if websocket in manager.openai_connections:
                        conn = manager.openai_connections[websocket]
                        await conn.input_audio_buffer.commit()
                        await conn.response.create()
                        
                        # Send acknowledgment back to client
                        await websocket.send_json({
                            "type": "audio_end_received",
                            "status": "ok"
                        })
                    else:
                        logger.error("No OpenAI connection available for this WebSocket")
                        await websocket.send_json({
                            "type": "error",
                            "message": "No OpenAI connection available. Please try reconnecting."
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
                    
                    # Get the OpenAI connection for this WebSocket
                    if websocket in manager.openai_connections:
                        conn = manager.openai_connections[websocket]
                        
                        # Send text to OpenAI
                        await conn.input_text.append(text=text)
                        await conn.input_text.commit()
                        await conn.response.create()
                        
                        # Send acknowledgment back to client
                        await websocket.send_json({
                            "type": "text_message_received",
                            "status": "ok"
                        })
                    else:
                        logger.error("No OpenAI connection available for this WebSocket")
                        await websocket.send_json({
                            "type": "error",
                            "message": "No OpenAI connection available. Please try reconnecting."
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
    uvicorn.run("server:app", host="0.0.0.0", port=8000, reload=False) 