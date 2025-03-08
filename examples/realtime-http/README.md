# OpenAI Realtime HTTP Server

This is a HTTP server application that serves a webpage for real-time audio streaming with WebAudio and provides an endpoint for sending text to the OpenAI Realtime API.

## Features

- Real-time audio streaming to and from OpenAI's Realtime API
- Audio visualization with WebAudio API
- Text input for sending messages to the API
- WebSocket communication for low-latency updates
- Responsive UI that works on desktop and mobile

## Requirements

- Python 3.9+
- OpenAI API key set as an environment variable (`OPENAI_API_KEY`)
- Microphone access for audio recording

## Installation

1. Install the required dependencies:

```bash
pip install -r requirements.txt
```

2. Set your OpenAI API key as an environment variable:

```bash
export OPENAI_API_KEY=your_api_key_here
```

## Usage

1. Start the server:

```bash
python server.py
```

2. Open your browser and navigate to `http://localhost:8000`

3. Grant microphone permissions when prompted

4. Use the "Start Recording" button to begin speaking, and "Stop Recording" to end

5. Alternatively, type a message in the text input and click "Send" or press Enter

## HTTP API

The server exposes the following endpoints:

- `GET /` - Serves the main webpage
- `WebSocket /ws` - WebSocket endpoint for real-time communication
- `POST /send-text` - HTTP endpoint for sending text messages

## Directory Structure

```
realtime-http/
├── server.py              # Main server application
├── requirements.txt       # Python dependencies
├── README.md              # This file
├── templates/
│   └── index.html         # HTML template
└── static/
    ├── css/
    │   └── styles.css     # CSS styles
    └── js/
        ├── audio-processor.js  # Audio processing logic
        ├── visualizer.js       # Audio visualization
        └── main.js             # Main application logic
```

## Troubleshooting

- If you encounter issues with audio recording, make sure your browser has permission to access your microphone
- If the server fails to start, check that all dependencies are installed and your OpenAI API key is set correctly
- For WebSocket connection issues, check your browser console for error messages

## License

This project is licensed under the MIT License - see the LICENSE file for details. 