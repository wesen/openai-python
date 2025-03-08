#!/usr/bin/env python3
"""
OpenAI client functionality for the Realtime HTTP server.
Handles communication with OpenAI's Realtime API.
"""
import os
import asyncio
import logging
import base64
from typing import Dict, Any, Optional, Callable, Awaitable

from openai import AsyncOpenAI
from openai.resources.beta.realtime.realtime import AsyncRealtimeConnection

# Configure logging
logger = logging.getLogger("realtime-http.openai")

# OpenAI client
client = AsyncOpenAI()
logger.info(f"AsyncOpenAI client initialized")

class OpenAIRealtimeClient:
    """Manages communication with OpenAI's Realtime API."""
    
    def __init__(self, message_handler: Callable[[Dict[str, Any]], Awaitable[bool]]):
        """
        Initialize the OpenAI Realtime client.
        
        Args:
            message_handler: Callback function to handle messages from OpenAI
        """
        self.message_handler = message_handler
        self.connection: Optional[AsyncRealtimeConnection] = None
        self.connection_task: Optional[asyncio.Task] = None
        
    async def connect(self):
        """Establish a connection to OpenAI's Realtime API."""
        try:
            # Use the proper async context manager pattern
            self.connection = await client.beta.realtime.connect(model="gpt-4o-realtime-preview")
            logger.info("OpenAI Realtime connection established")
            
            # Set up server-side VAD (Voice Activity Detection)
            logger.debug("Setting up server-side VAD")
            await self.connection.session.update(session={
                "turn_detection": {"type": "server_vad"},
                "input_audio_transcription": {
                    "model": "whisper-1"
                }
            })
            
            # Start listening for events from OpenAI
            self.connection_task = asyncio.create_task(self.handle_openai_events())
            return True
        except Exception as e:
            logger.error(f"Error connecting to OpenAI: {e}", exc_info=True)
            await self.message_handler({
                "type": "error",
                "message": f"OpenAI connection error: {str(e)}"
            })
            return False
    
    async def disconnect(self):
        """Disconnect from OpenAI's Realtime API."""
        if self.connection_task:
            self.connection_task.cancel()
            self.connection_task = None
        
        if self.connection:
            await self.connection.close()
            self.connection = None
    
    async def send_audio(self, audio_data: str):
        """
        Send audio data to OpenAI.
        
        Args:
            audio_data: Base64-encoded audio data
        """
        if not self.connection:
            logger.error("Cannot send audio: No OpenAI connection")
            return False
        
        try:
            await self.connection.input_audio_buffer.append(audio=audio_data)
            return True
        except Exception as e:
            logger.error(f"Error sending audio to OpenAI: {e}", exc_info=True)
            return False
    
    async def commit_audio(self):
        """Commit the audio buffer and create a response."""
        if not self.connection:
            logger.error("Cannot commit audio: No OpenAI connection")
            return False
        
        try:
            await self.connection.input_audio_buffer.commit()
            await self.connection.response.create()
            return True
        except Exception as e:
            logger.error(f"Error committing audio to OpenAI: {e}", exc_info=True)
            return False
    
    async def send_text(self, text: str):
        """
        Send a text message to OpenAI.
        
        Args:
            text: Text message to send
        """
        if not self.connection:
            logger.error("Cannot send text: No OpenAI connection")
            return False
        
        try:
            await self.connection.conversation.item.create(
                item={
                    "type": "message",
                    "role": "user",
                    "content": [{"type": "input_text", "text": text}],
                }
            )
            await self.connection.response.create()
            return True
        except Exception as e:
            logger.error(f"Error sending text to OpenAI: {e}", exc_info=True)
            return False
    
    async def handle_openai_events(self):
        """Handle events from OpenAI."""
        last_audio_item_id = None
        accumulated_text = {}
        user_transcripts = {}  # Store user transcripts
        
        try:
            logger.info("Starting to listen for OpenAI events")
            async for event in self.connection:
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
                    
                    await self.message_handler({
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
                    
                    await self.message_handler({
                        "type": "session.updated", 
                        "session": session_data
                    })
                
                elif event.type == "conversation.item.input_audio_transcription.completed":
                    # Handle user transcript
                    try:
                        # Extract transcript from the event
                        event_data = event.model_dump() if hasattr(event, 'model_dump') else vars(event)
                        transcript = event_data.get('transcript', '')
                        item_id = event_data.get('item_id', 'default_transcript_id')
                        
                        if transcript:
                            # Store the transcript
                            user_transcripts[item_id] = transcript
                            
                            # Send the user transcript to the client
                            await self.message_handler({
                                "type": "user_transcript",
                                "item_id": item_id,
                                "text": transcript
                            })
                            logger.info(f"Sent user transcript: {transcript}")
                    except Exception as e:
                        logger.error(f"Error handling user transcript: {e}", exc_info=True)
                
                elif event.type == "response.audio.delta":
                    # Send audio data to the client
                    if event.item_id != last_audio_item_id:
                        logger.debug(f"New audio item: {event.item_id}")
                        last_audio_item_id = event.item_id
                        
                        # Notify client about new audio stream starting
                        try:
                            await self.message_handler({
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
                        await self.message_handler({
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
                        await self.message_handler({
                            "type": "transcript",
                            "item_id": item_id,
                            "text": accumulated_text[item_id]
                        })
                        logger.debug(f"Sent transcript delta for item {item_id}")
                    except Exception as e:
                        logger.error(f"Error sending transcript: {e}", exc_info=True)
                
                elif event.type == "response.text.delta":
                    # Handle text delta
                    try:
                        # Get item ID if available, or use a default
                        item_id = getattr(event, 'item_id', 'default_text_id')
                        
                        # Initialize if this is a new item
                        if item_id not in accumulated_text:
                            accumulated_text[item_id] = ""
                        
                        # Accumulate the text
                        accumulated_text[item_id] += event.delta
                        
                        # Send text delta to the client
                        await self.message_handler({
                            "type": "text_delta",
                            "item_id": item_id,
                            "delta": event.delta,
                            "text": accumulated_text[item_id]  # Send full accumulated text
                        })
                        logger.debug(f"Sent text delta: {event.delta}")
                    except Exception as e:
                        logger.error(f"Error sending text delta: {e}", exc_info=True)
                
                elif event.type == "response.done":
                    # Response is complete
                    logger.info("Response completed")
                    try:
                        await self.message_handler({
                            "type": "response.done"
                        })
                        
                        # Clear accumulated text for the next response
                        accumulated_text = {}
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
                        await self.message_handler({
                            "type": "error",
                            "message": error_message
                        })
                    except Exception as e:
                        logger.error(f"Error sending error notification: {e}", exc_info=True)
        
        except asyncio.CancelledError:
            logger.info("OpenAI connection task cancelled")
        except Exception as e:
            logger.error(f"Error handling OpenAI events: {e}", exc_info=True)
            try:
                await self.message_handler({
                    "type": "error",
                    "message": f"Error handling OpenAI events: {str(e)}"
                })
            except Exception:
                pass  # Ignore if we can't send the error

# Audio transcoding function
def transcode_audio(base64_audio: str, audio_format: Optional[Dict[str, Any]] = None) -> str:
    """
    Transcode audio data to the format expected by OpenAI (mono PCM16 at 24kHz).
    
    Args:
        base64_audio: Base64-encoded audio data
        audio_format: Dictionary with audio format information
        
    Returns:
        Base64-encoded PCM audio data
    """
    # Default to empty dict if None
    if audio_format is None:
        audio_format = {}
    
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
                    import subprocess
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