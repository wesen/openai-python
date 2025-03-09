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