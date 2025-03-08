/**
 * Audio Processor for handling WebAudio API interactions
 */
class AudioProcessor {
    constructor() {
        console.log('AudioProcessor: Initializing');
        this.audioContext = null;
        this.stream = null;
        this.mediaRecorder = null;
        this.audioChunks = [];
        this.isRecording = false;
        this.analyser = null;
        this.dataArray = null;
        this.sampleRate = 24000; // Match the server's expected sample rate
        this.channels = 1; // Mono audio
        
        // Audio queue system
        this.audioQueue = [];
        this.isPlaying = false;
        this.currentSource = null;
        this.audioFormatSent = false;
    }

    /**
     * Initialize the audio context and request microphone permissions
     */
    async initialize() {
        console.log('AudioProcessor: Initializing audio context and requesting microphone permissions');
        try {
            // Create audio context
            this.audioContext = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: this.sampleRate
            });
            console.log('AudioProcessor: Audio context created with sample rate:', this.sampleRate);
            
            // Request microphone access
            console.log('AudioProcessor: Requesting microphone access...');
            this.stream = await navigator.mediaDevices.getUserMedia({ 
                audio: {
                    channelCount: this.channels,
                    sampleRate: this.sampleRate,
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                } 
            });
            console.log('AudioProcessor: Microphone access granted');
            
            // Set up analyser for visualization
            const source = this.audioContext.createMediaStreamSource(this.stream);
            this.analyser = this.audioContext.createAnalyser();
            this.analyser.fftSize = 2048;
            source.connect(this.analyser);
            console.log('AudioProcessor: Audio analyser set up');
            
            // Create buffer for visualization data
            this.dataArray = new Uint8Array(this.analyser.frequencyBinCount);
            console.log('AudioProcessor: Visualization buffer created with size:', this.analyser.frequencyBinCount);
            
            return true;
        } catch (error) {
            console.error('AudioProcessor ERROR: Error initializing audio:', error);
            return false;
        }
    }

    /**
     * Start recording audio
     * @param {Function} onDataAvailable - Callback for when audio data is available
     */
    startRecording(onDataAvailable) {
        console.log('AudioProcessor: Starting recording...');
        if (!this.stream) {
            console.error('AudioProcessor ERROR: Audio stream not initialized');
            return false;
        }

        this.audioChunks = [];
        this.isRecording = true;
        
        try {
            // Create media recorder
            console.log('AudioProcessor: Creating MediaRecorder');
            
            // Get the supported MIME types
            const supportedMimeTypes = [
                'audio/wav',
                'audio/wave',
                'audio/webm;codecs=pcm',
                'audio/webm;codecs=opus',
                'audio/webm',
                'audio/ogg;codecs=opus',
                'audio/ogg',
                'audio/mp4',
                'audio/pcm'
            ].filter(mimeType => MediaRecorder.isTypeSupported(mimeType));
            
            console.log('AudioProcessor: Supported MIME types:', supportedMimeTypes);
            
            // Use the first supported MIME type
            const mimeType = supportedMimeTypes.length > 0 ? supportedMimeTypes[0] : '';
            
            // Get audio track settings
            const audioTracks = this.stream.getAudioTracks();
            if (audioTracks.length > 0) {
                const settings = audioTracks[0].getSettings();
                console.log('AudioProcessor: Audio track settings:', settings);
                
                // Send audio format info to the server on the first chunk
                this.audioFormatSent = false;
            }
            
            // Create MediaRecorder with the selected MIME type
            this.mediaRecorder = new MediaRecorder(this.stream, { 
                mimeType: mimeType,
                audioBitsPerSecond: this.sampleRate * 16 // 16 bits per sample
            });
            
            console.log('AudioProcessor: MediaRecorder created with mimeType:', this.mediaRecorder.mimeType);
            
            // Handle data available event
            this.mediaRecorder.ondataavailable = (event) => {
                console.log('AudioProcessor: Data available event fired, data size:', event.data.size);
                if (event.data.size > 0) {
                    this.audioChunks.push(event.data);
                    
                    // Convert to base64 and send to server
                    const reader = new FileReader();
                    reader.onloadend = () => {
                        const base64data = reader.result.split(',')[1];
                        console.log('AudioProcessor: Converted audio chunk to base64, length:', base64data ? base64data.length : 0);
                        
                        if (onDataAvailable && base64data) {
                            console.log('AudioProcessor: Sending audio data to callback');
                            
                            // Send audio format info with the first chunk
                            if (!this.audioFormatSent) {
                                const formatInfo = {
                                    mimeType: this.mediaRecorder.mimeType,
                                    sampleRate: this.sampleRate,
                                    channels: this.channels,
                                    bitsPerSample: 16
                                };
                                console.log('AudioProcessor: Sending audio format info:', formatInfo);
                                onDataAvailable(base64data, formatInfo);
                                this.audioFormatSent = true;
                            } else {
                                onDataAvailable(base64data);
                            }
                        } else {
                            console.warn('AudioProcessor: No callback provided or base64data is empty');
                        }
                    };
                    reader.onerror = (error) => {
                        console.error('AudioProcessor ERROR: Error reading audio data:', error);
                    };
                    reader.readAsDataURL(event.data);
                } else {
                    console.warn('AudioProcessor: Received empty audio data');
                }
            };
            
            this.mediaRecorder.onerror = (error) => {
                console.error('AudioProcessor ERROR: MediaRecorder error:', error);
            };
            
            this.mediaRecorder.onstart = () => {
                console.log('AudioProcessor: MediaRecorder started');
            };
            
            this.mediaRecorder.onstop = () => {
                console.log('AudioProcessor: MediaRecorder stopped');
            };
            
            // Start recording with small time slices for real-time streaming
            console.log('AudioProcessor: Starting MediaRecorder with 100ms time slices');
            this.mediaRecorder.start(100);
            console.log('AudioProcessor: Recording started successfully');
            return true;
        } catch (error) {
            console.error('AudioProcessor ERROR: Failed to start recording:', error);
            this.isRecording = false;
            return false;
        }
    }

    /**
     * Stop recording audio
     */
    stopRecording() {
        console.log('AudioProcessor: Stopping recording...');
        if (this.mediaRecorder && this.isRecording) {
            try {
                this.mediaRecorder.stop();
                console.log('AudioProcessor: MediaRecorder stopped');
                this.isRecording = false;
                return true;
            } catch (error) {
                console.error('AudioProcessor ERROR: Error stopping recording:', error);
                return false;
            }
        } else {
            console.warn('AudioProcessor: Cannot stop recording - not currently recording');
            return false;
        }
    }

    /**
     * Get visualization data for the audio visualizer
     * @returns {Uint8Array} - Audio frequency data
     */
    getVisualizationData() {
        if (this.analyser) {
            this.analyser.getByteFrequencyData(this.dataArray);
            return this.dataArray;
        }
        return null;
    }

    /**
     * Play audio from base64 data
     * @param {string} base64Audio - Base64 encoded audio data
     */
    playAudio(base64Audio) {
        try {
            console.log('Adding audio to queue, data length:', base64Audio.length);
            
            // Add to queue and process
            this.audioQueue.push(base64Audio);
            
            // Start playing if not already playing
            if (!this.isPlaying) {
                this.processAudioQueue();
            }
        } catch (error) {
            console.error('Error processing audio data:', error);
        }
    }

    /**
     * Process the audio queue sequentially
     */
    async processAudioQueue() {
        if (this.audioQueue.length === 0 || this.isPlaying) {
            return;
        }
        
        this.isPlaying = true;
        
        try {
            // Get the next audio chunk
            const base64Audio = this.audioQueue.shift();
            
            // Create audio context if needed
            if (!this.audioContext) {
                console.warn('Audio context not initialized, creating one');
                this.audioContext = new (window.AudioContext || window.webkitAudioContext)({
                    sampleRate: this.sampleRate
                });
            }
            
            // Process the PCM audio
            await this.playPCMAudio(base64Audio);
            
        } catch (error) {
            console.error('Error in audio queue processing:', error);
            // Continue with next item even if there's an error
            this.isPlaying = false;
            this.processAudioQueue();
        }
    }

    /**
     * Play PCM audio data using Web Audio API
     * @param {string} base64Audio - Base64 encoded PCM audio data
     * @returns {Promise} - Resolves when audio playback is complete
     */
    playPCMAudio(base64Audio) {
        return new Promise((resolve, reject) => {
            try {
                console.log('Playing PCM audio directly');
                
                // Decode base64
                const binaryString = window.atob(base64Audio);
                const len = binaryString.length;
                const bytes = new Uint8Array(len);
                for (let i = 0; i < len; i++) {
                    bytes[i] = binaryString.charCodeAt(i);
                }
                
                // Convert to Int16 array (PCM format)
                const pcmData = new Int16Array(bytes.buffer);
                console.log('PCM data length:', pcmData.length, 'First few samples:', pcmData.slice(0, 10));
                
                // Convert to float32 for Web Audio API
                const floatData = new Float32Array(pcmData.length);
                for (let i = 0; i < pcmData.length; i++) {
                    floatData[i] = pcmData[i] / 32768.0;  // Convert from int16 to float
                }
                
                // Create buffer
                const audioBuffer = this.audioContext.createBuffer(
                    this.channels,
                    floatData.length,
                    this.sampleRate
                );
                
                // Fill the buffer
                const channelData = audioBuffer.getChannelData(0);
                channelData.set(floatData);
                
                // Create source and play
                const source = this.audioContext.createBufferSource();
                source.buffer = audioBuffer;
                source.connect(this.audioContext.destination);
                
                // Store current source
                this.currentSource = source;
                
                // Add event listeners for debugging and queue management
                source.onended = () => {
                    console.log('PCM audio playback ended');
                    this.currentSource = null;
                    this.isPlaying = false;
                    
                    // Process next item in queue
                    setTimeout(() => this.processAudioQueue(), 0);
                    
                    resolve();
                };
                
                // Start playback
                source.start();
                console.log('PCM audio playback started');
            } catch (error) {
                console.error('Error playing PCM audio:', error);
                console.error('Error details:', error.name, error.message);
                
                // Reset playing state
                this.isPlaying = false;
                this.currentSource = null;
                
                // Try fallback method
                this.fallbackPCMAudio(base64Audio)
                    .then(resolve)
                    .catch(reject);
            }
        });
    }

    /**
     * Fallback method for playing PCM audio
     * @param {string} base64Audio - Base64 encoded PCM audio data
     * @returns {Promise} - Resolves when audio playback is complete
     */
    fallbackPCMAudio(base64Audio) {
        return new Promise((resolve, reject) => {
            try {
                console.log('Trying fallback PCM audio playback method');
                
                // Create a new audio context
                const audioCtx = new (window.AudioContext || window.webkitAudioContext)({
                    sampleRate: this.sampleRate
                });
                
                // Decode base64
                const binary = atob(base64Audio);
                const buffer = new ArrayBuffer(binary.length);
                const view = new Uint8Array(buffer);
                for (let i = 0; i < binary.length; i++) {
                    view[i] = binary.charCodeAt(i);
                }
                
                // Use AudioContext.decodeAudioData for more robust decoding
                const audioData = new Int16Array(buffer);
                
                // Create a temporary WAV file in memory
                const wavData = this.createWAV(audioData);
                
                // Use the audio context to decode the WAV data
                audioCtx.decodeAudioData(wavData.buffer, 
                    (decodedData) => {
                        // Play the decoded audio
                        const source = audioCtx.createBufferSource();
                        source.buffer = decodedData;
                        source.connect(audioCtx.destination);
                        
                        // Add event listener for completion
                        source.onended = () => {
                            console.log('Fallback PCM audio playback ended');
                            this.isPlaying = false;
                            
                            // Process next item in queue
                            setTimeout(() => this.processAudioQueue(), 0);
                            
                            resolve();
                        };
                        
                        source.start(0);
                        console.log('Fallback PCM audio playback started');
                    },
                    (err) => {
                        console.error('Error decoding audio data:', err);
                        this.isPlaying = false;
                        reject(err);
                        
                        // Process next item in queue despite error
                        setTimeout(() => this.processAudioQueue(), 0);
                    }
                );
            } catch (error) {
                console.error('Error in fallback PCM audio playback:', error);
                this.isPlaying = false;
                reject(error);
                
                // Process next item in queue despite error
                setTimeout(() => this.processAudioQueue(), 0);
            }
        });
    }

    /**
     * Handle a new audio stream starting
     * This should clear any existing audio and prepare for a new stream
     */
    handleNewAudioStream() {
        console.log('Handling new audio stream');
        this.stopAllAudio();
    }

    /**
     * Stop all audio playback and clear the queue
     */
    stopAllAudio() {
        console.log('Stopping all audio playback');
        
        // Stop current playback
        if (this.currentSource) {
            try {
                this.currentSource.stop();
            } catch (e) {
                console.error('Error stopping current source:', e);
            }
            this.currentSource = null;
        }
        
        // Clear the queue
        this.audioQueue = [];
        this.isPlaying = false;
    }

    /**
     * Create a WAV file from PCM data
     * @param {Int16Array} pcmData - PCM audio data
     * @returns {Uint8Array} - WAV file data
     */
    createWAV(pcmData) {
        // WAV header parameters
        const numChannels = this.channels;
        const sampleRate = this.sampleRate;
        const bitsPerSample = 16;
        const blockAlign = numChannels * bitsPerSample / 8;
        const byteRate = sampleRate * blockAlign;
        const dataSize = pcmData.length * 2; // 16-bit samples = 2 bytes per sample
        const headerSize = 44;
        const totalSize = headerSize + dataSize;
        
        // Create the WAV header
        const wav = new Uint8Array(totalSize);
        
        // "RIFF" chunk descriptor
        wav.set([0x52, 0x49, 0x46, 0x46]); // "RIFF" in ASCII
        this.setUint32(wav, 4, 36 + dataSize, true); // Chunk size
        wav.set([0x57, 0x41, 0x56, 0x45], 8); // "WAVE" in ASCII
        
        // "fmt " sub-chunk
        wav.set([0x66, 0x6D, 0x74, 0x20], 12); // "fmt " in ASCII
        this.setUint32(wav, 16, 16, true); // Subchunk1Size (16 for PCM)
        this.setUint16(wav, 20, 1, true); // AudioFormat (1 for PCM)
        this.setUint16(wav, 22, numChannels, true); // NumChannels
        this.setUint32(wav, 24, sampleRate, true); // SampleRate
        this.setUint32(wav, 28, byteRate, true); // ByteRate
        this.setUint16(wav, 32, blockAlign, true); // BlockAlign
        this.setUint16(wav, 34, bitsPerSample, true); // BitsPerSample
        
        // "data" sub-chunk
        wav.set([0x64, 0x61, 0x74, 0x61], 36); // "data" in ASCII
        this.setUint32(wav, 40, dataSize, true); // Subchunk2Size
        
        // Write the PCM data
        const dataView = new DataView(wav.buffer);
        let offset = 44;
        for (let i = 0; i < pcmData.length; i++, offset += 2) {
            dataView.setInt16(offset, pcmData[i], true);
        }
        
        return wav;
    }

    /**
     * Helper function to set Uint16 values in a Uint8Array
     */
    setUint16(data, offset, value, littleEndian) {
        data[offset] = littleEndian ? (value & 0xFF) : (value >> 8);
        data[offset + 1] = littleEndian ? (value >> 8) : (value & 0xFF);
    }

    /**
     * Helper function to set Uint32 values in a Uint8Array
     */
    setUint32(data, offset, value, littleEndian) {
        if (littleEndian) {
            data[offset] = value & 0xFF;
            data[offset + 1] = (value >> 8) & 0xFF;
            data[offset + 2] = (value >> 16) & 0xFF;
            data[offset + 3] = (value >> 24) & 0xFF;
        } else {
            data[offset] = (value >> 24) & 0xFF;
            data[offset + 1] = (value >> 16) & 0xFF;
            data[offset + 2] = (value >> 8) & 0xFF;
            data[offset + 3] = value & 0xFF;
        }
    }

    /**
     * Clean up resources
     */
    cleanup() {
        this.stopAllAudio();
        
        if (this.mediaRecorder && this.isRecording) {
            this.mediaRecorder.stop();
        }
        
        if (this.stream) {
            this.stream.getTracks().forEach(track => track.stop());
        }
        
        if (this.audioContext) {
            this.audioContext.close();
        }
    }
} 