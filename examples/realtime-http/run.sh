#!/bin/bash

# Script to run the realtime-http server with different log levels

# Default log level
LOG_LEVEL=${1:-"info"}

# Validate log level
case $LOG_LEVEL in
  debug|info|warning|error|critical)
    echo "Setting log level to: $LOG_LEVEL"
    ;;
  *)
    echo "Invalid log level: $LOG_LEVEL"
    echo "Valid options are: debug, info, warning, error, critical"
    echo "Using default: info"
    LOG_LEVEL="info"
    ;;
esac

# Export the log level
export LOG_LEVEL=$LOG_LEVEL

# Print debug information
echo "===== DEBUG INFORMATION ====="
echo "Python version: $(python --version)"
echo "OpenAI version: $(pip show openai | grep Version)"
echo "Working directory: $(pwd)"
echo "OPENAI_API_KEY set: $(if [ -n "$OPENAI_API_KEY" ]; then echo "Yes"; else echo "No"; fi)"
echo "============================="

# Run the server
echo "Starting server with LOG_LEVEL=$LOG_LEVEL"
echo "Access the web interface at: http://localhost:8000"
echo "Debug endpoints:"
echo "  - http://localhost:8000/debug/audio"
echo "  - http://localhost:8000/debug/audio-format"
echo ""
echo "Press Ctrl+C to stop the server"
echo "============================="

# Run with debug flags if in debug mode
if [ "$LOG_LEVEL" = "debug" ]; then
    # Run with more verbose Python debugging
    PYTHONVERBOSE=1 python -u server.py
else
    python server.py 
fi