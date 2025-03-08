/**
 * Application configuration
 */
const config = {
  // WebSocket reconnection settings
  reconnection: {
    baseDelay: 1000,
    maxDelay: 30000,
    factor: 1.5
  },
  
  // Audio settings
  audio: {
    minAudioSize: 4800, // 100ms of audio at 24kHz, 16-bit, mono
    sampleRate: 24000,
    channels: 1,
    format: 'audio/mp4'
  },
  
  // UI settings
  ui: {
    messageUpdateInterval: 50, // ms
    visualizerUpdateInterval: 50 // ms
  },
  
  // Debug settings
  debug: {
    logWebSocketMessages: true,
    logAudioData: false
  }
};

export default config; 