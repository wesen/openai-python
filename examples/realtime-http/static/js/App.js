/**
 * Main application that coordinates all components
 */
import WebSocketManager from './WebSocketManager.js';
import MessageHandler from './MessageHandler.js';
import ChatUI from './ChatUI.js';
import AudioManager from './AudioManager.js';
import config from './config.js';

class App {
  /**
   * Initialize the application
   */
  constructor() {
    console.log('Initializing application');
    
    // Initialize DOM elements
    this.elements = {
      startRecordingBtn: document.getElementById('start-recording'),
      stopRecordingBtn: document.getElementById('stop-recording'),
      statusIndicator: document.getElementById('status-indicator'),
      statusText: document.getElementById('status-text'),
      connectionStatus: document.getElementById('connection-status'),
      sessionIdElement: document.getElementById('session-id'),
      chatMessages: document.getElementById('chat-messages'),
      textMessageInput: document.getElementById('text-message'),
      sendTextBtn: document.getElementById('send-text')
    };
    
    // Initialize components
    this.chatUI = new ChatUI(this.elements);
    this.audioManager = null;
    this.visualizer = null;
    this.messageHandler = null;
    this.webSocketManager = null;
    
    // Application state
    this.isConnected = false;
  }

  /**
   * Initialize the application
   * @returns {Promise<boolean>} - Whether initialization was successful
   */
  async initialize() {
    try {
      // Initialize audio visualizer
      console.log('Initializing audio visualizer');
      const AudioVisualizer = (await import('./visualizer.js')).default;
      this.visualizer = new AudioVisualizer('visualizer');
      
      // Initialize audio manager
      console.log('Initializing audio manager');
      this.audioManager = new AudioManager(this.visualizer);
      const audioInitialized = await this.audioManager.initialize();
      if (!audioInitialized) {
        this.chatUI.showError('Failed to initialize audio. Please check your microphone permissions.');
        console.error('Audio initialization failed');
        return false;
      }
      
      // Initialize message handler
      console.log('Initializing message handler');
      this.messageHandler = new MessageHandler(this.chatUI, this.audioManager, this.visualizer);
      
      // Initialize WebSocket manager
      console.log('Initializing WebSocket manager');
      this.webSocketManager = new WebSocketManager(
        config,
        (message) => this.messageHandler.handleMessage(message),
        (isConnected, statusText) => this.handleConnectionStatusChange(isConnected, statusText)
      );
      
      // Connect to WebSocket server
      console.log('Connecting to WebSocket server');
      this.webSocketManager.connect();
      
      // Set up event listeners
      console.log('Setting up event listeners');
      this.setupEventListeners();
      
      console.log('Application initialization complete');
      return true;
    } catch (error) {
      console.error('Error initializing application:', error);
      this.chatUI.showError(`Error initializing application: ${error.message}`);
      return false;
    }
  }

  /**
   * Set up event listeners for UI elements
   */
  setupEventListeners() {
    // Start recording button
    this.elements.startRecordingBtn.addEventListener('click', () => {
      this.startRecording();
    });
    
    // Stop recording button
    this.elements.stopRecordingBtn.addEventListener('click', () => {
      this.stopRecording();
    });
    
    // Send text button
    this.elements.sendTextBtn.addEventListener('click', () => {
      this.sendTextMessage();
    });
    
    // Text input enter key
    this.elements.textMessageInput.addEventListener('keypress', (event) => {
      if (event.key === 'Enter') {
        this.sendTextMessage();
      }
    });
    
    // Connection status observer - enable/disable buttons based on connection status
    const connectionObserver = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.type === 'characterData' || mutation.type === 'childList') {
          if (this.elements.connectionStatus.textContent === 'Connected') {
            this.elements.startRecordingBtn.disabled = false;
          } else {
            this.elements.startRecordingBtn.disabled = true;
            this.elements.stopRecordingBtn.disabled = true;
          }
        }
      });
    });
    
    connectionObserver.observe(this.elements.connectionStatus, { 
      characterData: true, 
      childList: true,
      subtree: true 
    });
    
    // Handle page unload
    window.addEventListener('beforeunload', () => {
      this.cleanup();
    });
    
    console.log('All event listeners set up');
  }

  /**
   * Handle connection status change
   * @param {boolean} isConnected - Whether the connection is established
   * @param {string} statusText - Optional status text to display
   */
  handleConnectionStatusChange(isConnected, statusText = null) {
    this.isConnected = isConnected;
    this.chatUI.updateConnectionStatus(isConnected, statusText);
    
    // Update button states
    if (isConnected) {
      this.elements.startRecordingBtn.disabled = false;
    } else {
      this.elements.startRecordingBtn.disabled = true;
      this.elements.stopRecordingBtn.disabled = true;
      
      // Stop recording if it's active
      if (this.audioManager && this.audioManager.isActive()) {
        this.audioManager.stopRecording();
        this.chatUI.updateRecordingStatus(false);
      }
    }
  }

  /**
   * Start recording audio
   */
  startRecording() {
    if (!this.isConnected) {
      console.error('Cannot start recording: Not connected to server');
      this.chatUI.showError('Not connected to server');
      return;
    }
    
    console.log('Attempting to start recording');
    const started = this.audioManager.startRecording((base64data, formatInfo) => {
      if (this.isConnected) {
        try {
          // If format info is provided (first chunk), send it to the server
          if (formatInfo) {
            console.log('Sending audio format info to server:', formatInfo);
            this.webSocketManager.sendMessage('audio_format', { format: formatInfo });
          }
          
          // Send the audio data
          console.log('Sending audio data to server, length:', base64data.length);
          this.webSocketManager.sendMessage('audio_data', { data: base64data });
        } catch (error) {
          console.error('Error sending audio data:', error);
          this.chatUI.showError('Error sending audio data. Connection may be lost.');
          this.stopRecording();
        }
      } else {
        console.warn('Cannot send audio data: WebSocket not connected');
        this.chatUI.showError('WebSocket connection lost. Please try again.');
        this.stopRecording();
      }
    });
    
    if (started) {
      console.log('Recording started successfully');
      this.chatUI.updateRecordingButtons(false, true);
      this.chatUI.updateRecordingStatus(true);
    } else {
      console.error('Failed to start recording');
      this.chatUI.showError('Failed to start recording. Please check microphone permissions.');
    }
  }

  /**
   * Stop recording audio
   */
  stopRecording() {
    console.log('Stop recording requested');
    const stopped = this.audioManager.stopRecording();
    
    if (stopped && this.isConnected) {
      console.log('Sending audio_end signal to server');
      try {
        this.webSocketManager.sendMessage('audio_end');
      } catch (error) {
        console.error('Error sending audio_end signal:', error);
        this.chatUI.showError('Error sending audio end signal. Connection may be lost.');
      }
    } else {
      console.warn('Could not send audio_end: recording not stopped or WebSocket not connected');
    }
    
    this.chatUI.updateRecordingButtons(true, false);
    this.chatUI.updateRecordingStatus(false);
  }

  /**
   * Send a text message to the server
   */
  sendTextMessage() {
    const text = this.chatUI.getInputText();
    if (!text || !this.isConnected) {
      console.log('Cannot send message: text empty or not connected');
      return;
    }
    
    console.log('Sending text message:', text);
    
    // Stop any ongoing audio playback
    this.audioManager.stopAllAudio();
    
    // Reset current item IDs for the new conversation
    this.messageHandler.resetConversation();
    
    // Add message to chat
    this.chatUI.addMessage('user', text);
    
    // Send to server
    this.webSocketManager.sendMessage('text_message', { text });
    
    // Clear input
    this.chatUI.clearInput();
  }

  /**
   * Clean up resources
   */
  cleanup() {
    console.log('Cleaning up resources');
    
    if (this.audioManager) {
      this.audioManager.cleanup();
    }
    
    if (this.webSocketManager) {
      this.webSocketManager.disconnect();
    }
  }
}

// Initialize the application when the DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  console.log('DOM loaded, initializing application');
  const app = new App();
  app.initialize();
});

export default App; 