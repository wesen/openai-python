# OpenAI Realtime HTTP Server Specification

## Overview

The OpenAI Realtime HTTP Server is a full-stack web application that demonstrates real-time audio and text communication with OpenAI's Realtime API. It provides a responsive web interface for users to interact with AI models through both voice and text inputs, with real-time streaming responses.

## Architecture

The application follows a client-server architecture:

1. **Backend**: A FastAPI server that handles WebSocket connections, audio processing, and communication with OpenAI's Realtime API.
2. **Frontend**: A web interface built with HTML, CSS, and vanilla JavaScript that provides audio recording, visualization, and chat functionality.

## Backend Components

### Server (server.py)

The main server application built with FastAPI that handles:

- WebSocket connections for real-time communication
- HTTP endpoints for serving the web interface and handling text messages
- Audio processing and transcoding
- Connection management with OpenAI's Realtime API

#### Key Components:

1. **ConnectionManager**: Manages WebSocket connections and OpenAI Realtime connections
   - `connect(websocket)`: Establishes a WebSocket connection
   - `disconnect(websocket)`: Handles WebSocket disconnection
   - `broadcast(message)`: Sends a message to all connected clients
   - `manage_openai_connection(websocket)`: Creates and manages an OpenAI Realtime connection
   - `handle_openai_events(conn, websocket)`: Processes events from the OpenAI Realtime API

2. **Audio Processing**:
   - `transcode_audio(base64_audio, audio_format)`: Converts audio data between formats

3. **Endpoints**:
   - `GET /`: Serves the main web interface
   - `WebSocket /ws`: WebSocket endpoint for real-time communication
   - `POST /send-text`: HTTP endpoint for sending text messages
   - `GET /debug/audio-format`: Debug endpoint for audio format information
   - `GET /debug/audio`: Debug endpoint for audio testing

### Audio Utilities

#### FFmpegAudioDecoder (audio_decoder.py)

A utility class that uses FFmpeg to decode audio streams:

- `decode_chunk(chunk_data)`: Processes a chunk of encoded audio data
- `get_decoded_audio(max_size)`: Retrieves decoded PCM audio
- `has_enough_data()`: Checks if enough audio data is available

#### Audio Utilities (audio_util.py)

Provides audio processing utilities:

- `audio_to_pcm16_base64(audio_bytes)`: Converts audio to PCM16 format with base64 encoding
- `AudioPlayerAsync`: Asynchronous audio player for streaming audio playback
- `send_audio_worker_sounddevice(connection, should_send, start_send)`: Worker for sending audio to the OpenAI API

### Azure Support (azure_realtime.py)

Provides Azure OpenAI integration for the Realtime API:

- Demonstrates authentication with Azure credentials
- Shows how to use the Azure OpenAI client with the Realtime API

## Frontend Components

### Main Application (main.js)

The core frontend logic that:

- Initializes the application
- Manages WebSocket connections
- Handles user interactions
- Processes messages to and from the server

#### Key Functions:

- `init()`: Initializes the application
- `connectWebSocket()`: Establishes a WebSocket connection to the server
- `handleWebSocketMessage(event)`: Processes incoming WebSocket messages
- `setupEventListeners()`: Sets up event listeners for user interactions
- `sendTextMessage()`: Sends a text message to the server
- `addMessage(role, content)`: Adds a message to the chat interface
- `updateOrCreateMessage(role, content, isDelta, forceUpdate)`: Updates or creates a message in the chat interface
- `showError(message)`: Displays an error message to the user

### Audio Processor (audio-processor.js)

Handles audio recording, playback, and processing:

- `initialize()`: Sets up the audio context and requests microphone permissions
- `startRecording(onDataAvailable)`: Begins audio recording
- `stopRecording()`: Stops audio recording
- `getVisualizationData()`: Provides data for audio visualization
- `playAudio(base64Audio)`: Plays audio from base64-encoded data
- `processAudioQueue()`: Processes the audio playback queue
- `playPCMAudio(base64Audio)`: Plays PCM audio from base64-encoded data
- `fallbackPCMAudio(base64Audio)`: Fallback method for audio playback
- `cleanup()`: Cleans up audio resources

### Audio Visualizer (visualizer.js)

Provides real-time visualization of audio:

- `start()`: Starts the visualization animation
- `stop()`: Stops the visualization animation
- `draw()`: Renders the audio visualization
- `updateForAudio(isActive)`: Updates the visualization based on audio activity

## Communication Protocol

### WebSocket Messages

#### Client to Server:

1. **Audio Data**:
   ```json
   {
     "type": "audio_data",
     "format": "audio/mp4",
     "data": "<base64-encoded-audio>"
   }
   ```

2. **Audio End**:
   ```json
   {
     "type": "audio_end"
   }
   ```

3. **Text Message**:
   ```json
   {
     "type": "text_message",
     "text": "<message-text>"
   }
   ```

#### Server to Client:

1. **Connection Established**:
   ```json
   {
     "type": "connection_established",
     "session_id": "<session-id>"
   }
   ```

2. **Audio Format**:
   ```json
   {
     "type": "audio_format",
     "format": {
       "sample_rate": 24000,
       "channels": 1
     }
   }
   ```

3. **Audio Data**:
   ```json
   {
     "type": "audio_data",
     "data": "<base64-encoded-audio>"
   }
   ```

4. **Text Response**:
   ```json
   {
     "type": "text_response",
     "text": "<response-text>",
     "is_final": false
   }
   ```

5. **Transcript**:
   ```json
   {
     "type": "transcript",
     "text": "<transcribed-text>",
     "is_final": false
   }
   ```

6. **Error**:
   ```json
   {
     "type": "error",
     "message": "<error-message>"
   }
   ```

## Data Flow

1. **Audio Recording Flow**:
   - User clicks "Start Recording"
   - Browser requests microphone access
   - Audio is recorded and sent to the server in chunks via WebSocket
   - Server transcodes the audio and sends it to OpenAI's Realtime API
   - OpenAI processes the audio and returns transcriptions and responses
   - Server forwards transcriptions and responses to the client
   - Client displays transcriptions and plays audio responses

2. **Text Input Flow**:
   - User types a message and clicks "Send" or presses Enter
   - Message is sent to the server via WebSocket
   - Server forwards the message to OpenAI's Realtime API
   - OpenAI processes the text and returns responses
   - Server forwards responses to the client
   - Client displays text responses and plays audio responses

## Dependencies

### Backend Dependencies:

- **FastAPI**: Web framework for building APIs
- **Uvicorn**: ASGI server for running FastAPI
- **Jinja2**: Template engine for HTML
- **OpenAI**: Python client for OpenAI API
- **NumPy**: Numerical computing library
- **PyAudio**: Audio I/O library
- **Pydub**: Audio processing library
- **SoundDevice**: Audio playback library

### Frontend Dependencies:

- **Web Audio API**: For audio recording and processing
- **Canvas API**: For audio visualization
- **WebSocket API**: For real-time communication

## Setup and Configuration

### Environment Variables:

- `OPENAI_API_KEY`: OpenAI API key
- `LOG_LEVEL`: Logging level (debug, info, warning, error, critical)

### Running the Application:

1. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```

2. Set environment variables:
   ```bash
   export OPENAI_API_KEY=your_api_key_here
   ```

3. Run the server:
   ```bash
   ./run.sh [log_level]
   ```
   or
   ```bash
   python server.py
   ```

4. Access the web interface at `http://localhost:8000`

## Security Considerations

1. **API Key Protection**: The OpenAI API key is stored as an environment variable and not exposed to clients.
2. **Input Validation**: All user inputs are validated before processing.
3. **Error Handling**: Comprehensive error handling to prevent application crashes.
4. **Secure WebSocket Communication**: WebSocket connections are established with proper handshaking.

## Performance Considerations

1. **Audio Chunking**: Audio is processed in small chunks to reduce latency.
2. **Asynchronous Processing**: Asynchronous handling of WebSocket connections and API requests.
3. **Efficient Audio Processing**: Audio is processed efficiently to minimize CPU usage.
4. **Connection Management**: Proper management of WebSocket and OpenAI connections to prevent resource leaks.

## Limitations

1. **Browser Compatibility**: Requires a modern browser with WebAudio and WebSocket support.
2. **Network Dependency**: Requires a stable internet connection for communication with OpenAI's API.
3. **API Rate Limits**: Subject to OpenAI's API rate limits and quotas.
4. **Audio Quality**: Audio quality depends on the user's microphone and network conditions.

## Future Enhancements

1. **Multi-user Support**: Add support for multiple concurrent users with separate sessions.
2. **Authentication**: Implement user authentication for secure access.
3. **Conversation History**: Store and retrieve conversation history.
4. **Custom Model Parameters**: Allow users to customize model parameters.
5. **Mobile App**: Develop a native mobile application for improved performance on mobile devices.
6. **Offline Mode**: Implement offline capabilities for basic functionality without an internet connection.

## Troubleshooting

1. **Microphone Access**: If the microphone doesn't work, check browser permissions.
2. **Audio Playback**: If audio playback doesn't work, try using a different browser.
3. **Connection Issues**: If the WebSocket connection fails, check your internet connection and firewall settings.
4. **API Errors**: If you encounter API errors, verify your API key and check OpenAI's service status.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 