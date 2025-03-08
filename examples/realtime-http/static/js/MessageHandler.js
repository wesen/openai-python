/**
 * Handles different types of messages from the server
 */
class MessageHandler {
  /**
   * Initialize the message handler
   * @param {Object} chatUI - Chat UI instance
   * @param {Object} audioManager - Audio manager instance
   * @param {Object} visualizer - Audio visualizer instance
   */
  constructor(chatUI, audioManager, visualizer) {
    this.chatUI = chatUI;
    this.audioManager = audioManager;
    this.visualizer = visualizer;
    
    // Track current response items
    this.currentTextItemId = null;
    this.currentTranscriptItemId = null;
    this.currentUserTranscriptItemId = null;
    this.sessionId = null;
  }

  /**
   * Handle a message from the server
   * @param {Object} message - The message to handle
   */
  handleMessage(message) {
    const messageType = message.type;
    
    // Route the message to the appropriate handler
    switch (messageType) {
      case 'session.created':
        this.handleSessionCreated(message);
        break;
        
      case 'audio_stream_start':
        this.handleAudioStream(message);
        break;
        
      case 'audio_data':
        this.handleAudioData(message);
        break;
        
      case 'transcript':
        this.handleTranscript(message);
        break;
        
      case 'text_delta':
        this.handleTextDelta(message);
        break;
        
      case 'user_transcript':
        this.handleUserTranscript(message);
        break;
        
      case 'response.done':
        this.handleResponseDone(message);
        break;
        
      case 'error':
        this.handleError(message);
        break;
        
      case 'text_message_received':
        this.handleTextMessageReceived(message);
        break;
        
      case 'audio_data_received':
        this.handleAudioDataReceived(message);
        break;
        
      case 'audio_end_received':
        this.handleAudioEndReceived(message);
        break;
        
      case 'audio_format_received':
        this.handleAudioFormatReceived(message);
        break;
        
      default:
        console.log('Received unknown message type:', messageType);
    }
  }

  /**
   * Handle session.created message
   * @param {Object} message - The message to handle
   */
  handleSessionCreated(message) {
    this.sessionId = message.session_id;
    this.chatUI.updateSessionId(this.sessionId);
    console.log('Session created with ID:', this.sessionId);
  }

  /**
   * Handle audio_stream_start message
   * @param {Object} message - The message to handle
   */
  handleAudioStream(message) {
    console.log('New audio stream starting:', message.item_id);
    this.audioManager.handleNewAudioStream();
    this.visualizer.updateForAudio(true);
  }

  /**
   * Handle audio_data message
   * @param {Object} message - The message to handle
   */
  handleAudioData(message) {
    console.log('Received audio data, length:', message.data.length);
    this.audioManager.playAudio(message.data);
    this.visualizer.updateForAudio(true);
  }

  /**
   * Handle transcript message
   * @param {Object} message - The message to handle
   */
  handleTranscript(message) {
    console.log('Received transcript:', message.text);
    // Check if this is a new transcript item
    if (this.currentTranscriptItemId !== message.item_id) {
      this.currentTranscriptItemId = message.item_id;
      // Create a new message for a new transcript
      this.chatUI.addMessage('ai', message.text);
    } else {
      // Update existing transcript
      this.chatUI.updateMessage('ai', message.text, false, true);
    }
  }

  /**
   * Handle text_delta message
   * @param {Object} message - The message to handle
   */
  handleTextDelta(message) {
    console.log('Received text delta:', message.delta);
    console.log('Full text so far:', message.text);
    // Check if this is a new text item
    if (this.currentTextItemId !== message.item_id) {
      this.currentTextItemId = message.item_id;
      // Create a new message for a new text response
      this.chatUI.addMessage('ai', message.text);
    } else {
      // Update existing text
      this.chatUI.updateMessage('ai', message.text, false, true);
    }
  }

  /**
   * Handle user_transcript message
   * @param {Object} message - The message to handle
   */
  handleUserTranscript(message) {
    console.log('Received user transcript:', message.text);
    // Check if this is a new user transcript
    if (this.currentUserTranscriptItemId !== message.item_id) {
      this.currentUserTranscriptItemId = message.item_id;
      // Create a new message for a new user transcript
      this.chatUI.addMessage('user', message.text);
    } else {
      // Update existing user transcript
      this.chatUI.updateMessage('user', message.text, false, true);
    }
  }

  /**
   * Handle response.done message
   * @param {Object} message - The message to handle
   */
  handleResponseDone(message) {
    console.log('Response completed');
    // Response is complete, reset current item IDs for next response
    this.currentTextItemId = null;
    this.currentTranscriptItemId = null;
    this.visualizer.updateForAudio(false);
  }

  /**
   * Handle error message
   * @param {Object} message - The message to handle
   */
  handleError(message) {
    console.error('Error from server:', message.message);
    this.chatUI.showError(message.message);
  }

  /**
   * Handle text_message_received message
   * @param {Object} message - The message to handle
   */
  handleTextMessageReceived(message) {
    console.log('Text message received confirmation');
    this.audioManager.stopAllAudio();
  }

  /**
   * Handle audio_data_received message
   * @param {Object} message - The message to handle
   */
  handleAudioDataReceived(message) {
    console.log('Audio data received confirmation');
  }

  /**
   * Handle audio_end_received message
   * @param {Object} message - The message to handle
   */
  handleAudioEndReceived(message) {
    console.log('Audio end received confirmation');
  }

  /**
   * Handle audio_format_received message
   * @param {Object} message - The message to handle
   */
  handleAudioFormatReceived(message) {
    console.log('Audio format received confirmation');
  }

  /**
   * Reset conversation state for a new conversation
   */
  resetConversation() {
    this.currentTextItemId = null;
    this.currentTranscriptItemId = null;
    this.currentUserTranscriptItemId = null;
  }
}

export default MessageHandler; 