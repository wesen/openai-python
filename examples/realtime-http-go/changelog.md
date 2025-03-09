# Changelog

## OpenAI Realtime HTTP Go Implementation

This implementation ports the Python version of the OpenAI Realtime HTTP server to Go, using HTMX and the Templ templating language. The implementation provides a web interface for real-time audio and text communication with OpenAI's Realtime API.

### Changes:
- Created Go implementation of the OpenAI Realtime HTTP server
- Used HTMX for frontend interactivity
- Used Templ for HTML templating
- Implemented WebSocket communication for real-time updates
- Added audio recording and visualization
- Added text-based chat functionality
- Implemented communication with OpenAI's Realtime API 

## 2023-03-09: Fixed OpenAI Realtime Connection Issues

Fixed connection issues with the OpenAI Realtime API by updating the WebSocket URL structure:
- Changed the endpoint from `realtime.openai.com` to `api.openai.com` to match the Python SDK implementation
- Added model parameter to the WebSocket URL
- Added support for `OPENAI_API_BASE` environment variable to customize the base URL
- Added required `OpenAI-Beta: realtime=v1` header
- Improved error logging when connecting to the API

## 2023-03-09: Added Comprehensive Debug Logging

Enhanced the application with comprehensive debug logging to troubleshoot WebSocket connections:
- Added detailed logging to the OpenAI client to track connection lifecycle and message exchange
- Added logging to WebSocket server handlers to trace client connections
- Enhanced WebSocket connection manager with message tracking
- Added client-side JavaScript debugging for WebSocket connections and events
- Improved error handling and reporting for the HTMX WebSocket extension
- Added better parsing of session information from OpenAI responses
- Fixed event handling in the client-side WebSocket code

## 2023-03-09: Fixed Client-Side JavaScript Issues

Fixed several client-side JavaScript issues to improve user experience and browser compatibility:
- Deferred AudioContext initialization until user interaction to comply with browser autoplay policies
- Fixed layout issues that were causing visual flicker during page load
- Improved audio visualization with better rendering and initialization
- Enhanced HTMX WebSocket event handling for more reliable connections
- Added proper session ID display and connection status updates
- Fixed audio recording process to ensure consistent audio data
- Improved error handling with more descriptive messages and proper fallbacks

## 2023-03-09: Fixed OpenAI Session Handling Issues

Fixed critical issues with OpenAI Realtime API session handling that were causing connection errors:
- Updated session.created event parsing to correctly extract session ID from nested session object
- Added proper session ID inclusion in all API requests to OpenAI
- Added session recovery mechanism for error responses
- Improved error handling for specific OpenAI error codes
- Updated message structure to include session ID in all communications
- Added warning logs for operations attempted without a valid session ID
- Fixed JSON parsing for OpenAI's response format

## 2023-03-09: Improved Audio Data Logging

Improved the logging system to make it more readable and manageable when dealing with audio data:
- Added a utility function to truncate long strings in logs
- Implemented truncated logging for audio data in the OpenAI client
- Enhanced WebSocket manager to show truncated audio data while preserving context
- Added byte length information to all audio data logs
- Improved readability of WebSocket binary messages
- Added conditional logging based on message type and length
- Standardized log format for audio-related operations 

## 2023-03-09: Fixed OpenAI Realtime Session Management

Fixed critical issues with the OpenAI Realtime API session management:
- Removed premature session update attempts before session ID is received from the API
- Fixed "missing_required_parameter" errors by waiting for the session.created event before attempting session updates
- Improved security by not logging sensitive headers with API keys
- Truncated request header logs to only show header names, not values 

## 2023-03-09: Fixed OpenAI Realtime API Message Format

Fixed critical issues with the OpenAI Realtime API message format:
- Corrected the request structure to properly include a "session" parameter with ID instead of using "session_id"
- Fixed "missing_required_parameter" errors by following the correct message format per the official API specification
- Updated all message types (audio data, commits, text messages) to use the correct session parameter structure
- Changed conversation item creation format to match the expected API structure 