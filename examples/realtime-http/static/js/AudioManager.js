/**
 * Manages audio recording and playback
 */
class AudioManager {
  /**
   * Initialize the audio manager
   * @param {Object} visualizer - Audio visualizer instance
   */
  constructor(visualizer) {
    this.audioProcessor = null;
    this.visualizer = visualizer;
    this.isRecording = false;
    this.onDataCallback = null;
  }

  /**
   * Initialize the audio processor and visualizer
   * @returns {Promise<boolean>} - Whether initialization was successful
   */
  async initialize() {
    try {
      console.log('Initializing audio processor');
      // Import the AudioProcessor dynamically
      const AudioProcessor = (await import('./audio-processor.js')).default;
      this.audioProcessor = new AudioProcessor();
      
      const audioInitialized = await this.audioProcessor.initialize();
      if (!audioInitialized) {
        console.error('Audio initialization failed');
        return false;
      }
      
      console.log('Audio processor initialized successfully');
      
      // Initialize visualizer with the audio processor
      if (this.visualizer) {
        this.visualizer.setAudioProcessor(this.audioProcessor);
        console.log('Audio visualizer initialized');
      }
      
      return true;
    } catch (error) {
      console.error('Error initializing audio manager:', error);
      return false;
    }
  }

  /**
   * Start recording audio
   * @param {Function} onDataAvailable - Callback for when audio data is available
   * @returns {boolean} - Whether recording started successfully
   */
  startRecording(onDataAvailable) {
    if (!this.audioProcessor) {
      console.error('Cannot start recording: Audio processor not initialized');
      return false;
    }
    
    if (this.isRecording) {
      console.warn('Already recording');
      return false;
    }
    
    console.log('Starting audio recording');
    this.onDataCallback = onDataAvailable;
    const started = this.audioProcessor.startRecording((base64data, formatInfo) => {
      if (this.onDataCallback) {
        this.onDataCallback(base64data, formatInfo);
      }
    });
    
    if (started) {
      this.isRecording = true;
      if (this.visualizer) {
        this.visualizer.start();
      }
    }
    
    return started;
  }

  /**
   * Stop recording audio
   * @returns {boolean} - Whether recording was stopped successfully
   */
  stopRecording() {
    if (!this.audioProcessor) {
      console.error('Cannot stop recording: Audio processor not initialized');
      return false;
    }
    
    if (!this.isRecording) {
      console.warn('Not currently recording');
      return false;
    }
    
    console.log('Stopping audio recording');
    const stopped = this.audioProcessor.stopRecording();
    
    if (stopped) {
      this.isRecording = false;
      if (this.visualizer) {
        this.visualizer.stop();
      }
    }
    
    return stopped;
  }

  /**
   * Play audio from base64-encoded data
   * @param {string} base64Audio - Base64-encoded audio data
   */
  playAudio(base64Audio) {
    if (!this.audioProcessor) {
      console.error('Cannot play audio: Audio processor not initialized');
      return;
    }
    
    this.audioProcessor.playAudio(base64Audio);
  }

  /**
   * Stop all audio playback
   */
  stopAllAudio() {
    if (!this.audioProcessor) {
      console.error('Cannot stop audio: Audio processor not initialized');
      return;
    }
    
    this.audioProcessor.stopAllAudio();
  }

  /**
   * Handle a new audio stream
   */
  handleNewAudioStream() {
    if (!this.audioProcessor) {
      console.error('Cannot handle new audio stream: Audio processor not initialized');
      return;
    }
    
    this.audioProcessor.handleNewAudioStream();
  }

  /**
   * Clean up audio resources
   */
  cleanup() {
    if (this.audioProcessor) {
      this.audioProcessor.cleanup();
    }
    
    if (this.visualizer) {
      this.visualizer.stop();
    }
    
    this.isRecording = false;
    this.onDataCallback = null;
  }

  /**
   * Check if recording is active
   * @returns {boolean} - Whether recording is active
   */
  isActive() {
    return this.isRecording;
  }
}

export default AudioManager; 