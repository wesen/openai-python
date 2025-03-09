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

## 2025-03-09: Fixed OpenAI Realtime API Message Format

Fixed issues with the OpenAI Realtime API communication by updating the message format to match the official API specification:
- Fixed session update messages to use `"session"` field instead of `"content"` to comply with the API requirements
- Updated audio message format to use `"input_audio_buffer.append"` instead of `"input_audio.data"`
- Changed audio commit format to use `"input_audio_buffer.commit"` instead of `"input_audio.commit"`
- Simplified text message format to match the API specification for `"conversation.item.create"`
- Removed unnecessary `"response.create"` calls that were causing errors
- Restructured JSON payload format for all message types to correctly handle session information
- Fixed session ID inclusion in update messages to ensure proper session tracking
- Eliminated redundant message wrapping that was causing parsing errors on the server 

## 2025-03-09: Enhanced OpenAI Message Logging

Improved message logging for OpenAI API communication to help with debugging and API integration:
- Added detailed JSON logging for all outgoing WebSocket messages to OpenAI
- Enhanced incoming message logging to show complete message content
- Formatted audio data logging to show message type and content length information
- Added special handling for audio-related messages to avoid log bloat while still showing relevant details
- Improved readability of WebSocket message types (TEXT/BINARY) in logs
- Added consistent log prefixes for better log filtering and analysis
- Enhanced message structure visibility to help with API debugging 

## 2025-03-09: Fixed Session Update Message Format

Fixed remaining session update structure issues based on official OpenAI Realtime API documentation:
- Removed incorrect session ID field from the session update object
- Aligned message structure exactly with the official API specification format
- Fixed WebSocket connection handling to properly maintain session context
- Updated error recovery mechanism to handle WebSocket reconnections
- Improved session state tracking to prevent errors in message handling
- Enhanced error messaging to provide clearer diagnostics for API protocol issues 