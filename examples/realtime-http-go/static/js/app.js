/**
 * OpenAI Realtime HTTP Client
 * 
 * Main application script that handles WebSocket communication,
 * audio recording, and UI interactions.
 */

document.addEventListener('DOMContentLoaded', () => {
    console.log('[Client] DOM loaded, initializing application');
    
    // DOM Elements
    const elements = {
        startRecordingBtn: document.getElementById('start-recording'),
        stopRecordingBtn: document.getElementById('stop-recording'),
        statusIndicator: document.getElementById('status-indicator'),
        statusText: document.getElementById('status-text'),
        connectionStatus: document.getElementById('connection-status'),
        sessionIdElement: document.getElementById('session-id'),
        chatMessages: document.getElementById('chat-messages'),
        textMessageInput: document.getElementById('text-message'),
        sendTextBtn: document.getElementById('send-text'),
        messageSender: document.getElementById('message-sender'),
        visualizer: document.getElementById('visualizer')
    };
    
    // Application state
    const state = {
        isConnected: false,
        isRecording: false,
        mediaRecorder: null,
        audioContext: null,
        analyser: null,
        visualizerAnimationFrame: null,
        sessionId: null,
        audioInitialized: false
    };
    
    // Initialize audio context - now deferred until user interaction
    function initAudio() {
        if (state.audioInitialized) {
            return true;
        }
        
        try {
            state.audioContext = new (window.AudioContext || window.webkitAudioContext)();
            state.analyser = state.audioContext.createAnalyser();
            state.analyser.fftSize = 256;
            
            state.audioInitialized = true;
            return true;
        } catch (error) {
            console.error('[Client] Error initializing audio context:', error);
            showError('Could not initialize audio context. Please check your browser compatibility.');
            return false;
        }
    }
    
    // Initialize audio visualizer
    function initVisualizer() {
        if (!state.audioContext || !state.analyser) {
            console.error('[Client] Cannot initialize visualizer: Audio context not initialized');
            return;
        }
        
        // Get canvas context
        const canvas = elements.visualizer;
        const canvasCtx = canvas.getContext('2d');
        
        function draw() {
            // Continue animation
            state.visualizerAnimationFrame = requestAnimationFrame(draw);
            
            // Get frequency data
            const bufferLength = state.analyser.frequencyBinCount;
            const dataArray = new Uint8Array(bufferLength);
            state.analyser.getByteFrequencyData(dataArray);
            
            // Clear canvas
            canvasCtx.clearRect(0, 0, canvas.width, canvas.height);
            
            // Draw visualization
            const barWidth = (canvas.width / bufferLength) * 2.5;
            let x = 0;
            
            for (let i = 0; i < bufferLength; i++) {
                const barHeight = dataArray[i] / 2;
                
                canvasCtx.fillStyle = `rgb(50, ${barHeight + 100}, 255)`;
                canvasCtx.fillRect(x, canvas.height - barHeight, barWidth, barHeight);
                
                x += barWidth + 1;
            }
        }
        
        draw();
    }
    
    // Stop visualizer
    function stopVisualizer() {
        if (state.visualizerAnimationFrame) {
            cancelAnimationFrame(state.visualizerAnimationFrame);
            state.visualizerAnimationFrame = null;
        }
    }
    
    // Start recording
    function startRecording() {
        // Initialize audio context on user gesture
        if (!initAudio()) {
            return;
        }
        
        // Don't start recording if already recording
        if (state.isRecording) {
            return;
        }
        
        // Don't start recording if not connected
        if (!state.isConnected) {
            showError('Cannot start recording: Not connected to the server');
            return;
        }
        
        // Resume audio context (required by browsers)
        if (state.audioContext.state === 'suspended') {
            state.audioContext.resume();
        }
        
        // Request microphone access
        navigator.mediaDevices.getUserMedia({ audio: true })
            .then(stream => {
                // Set up audio stream
                const audioContext = state.audioContext;
                const audioSource = audioContext.createMediaStreamSource(stream);
                audioSource.connect(state.analyser);
                
                // Initialize visualizer now that we have audio
                initVisualizer();
                
                // Create MediaRecorder
                state.mediaRecorder = new MediaRecorder(stream);
                
                // Handle data available event
                state.mediaRecorder.ondataavailable = event => {
                    if (event.data.size > 0) {
                        const reader = new FileReader();
                        reader.onloadend = () => {
                            const base64Data = reader.result.split(',')[1];
                            
                            // Send audio format information first (only once)
                            if (!state.hassentFormat) {
                                const formatInfo = {
                                    mimeType: state.mediaRecorder.mimeType,
                                    sampleRate: state.audioContext.sampleRate,
                                    channels: 1
                                };
                                
                                sendWebSocketMessage({
                                    type: 'audio_format',
                                    data: formatInfo
                                });
                                
                                state.hassentFormat = true;
                            }
                            
                            // Send audio data
                            sendWebSocketMessage({
                                type: 'audio_data',
                                data: base64Data
                            });
                        };
                        
                        reader.readAsDataURL(event.data);
                    }
                };
                
                // Start recording
                state.mediaRecorder.start(100);
                state.isRecording = true;
                
                // Update UI
                elements.startRecordingBtn.disabled = true;
                elements.stopRecordingBtn.disabled = false;
                elements.statusIndicator.classList.add('recording');
                elements.statusText.textContent = 'Recording...';
            })
            .catch(error => {
                console.error('Error accessing microphone:', error);
                showError('Could not access microphone. Please check your permissions.');
            });
    }
    
    // Stop recording audio
    function stopRecording() {
        if (!state.isRecording || !state.mediaRecorder) {
            return;
        }
        
        // Stop recording
        state.mediaRecorder.stop();
        state.mediaRecorder.stream.getTracks().forEach(track => track.stop());
        state.mediaRecorder = null;
        state.isRecording = false;
        state.hassentFormat = false;
        
        // Stop visualizer
        stopVisualizer();
        
        // Send end of audio signal
        sendWebSocketMessage({
            type: 'audio_end'
        });
        
        // Update UI
        elements.startRecordingBtn.disabled = false;
        elements.stopRecordingBtn.disabled = true;
        elements.statusIndicator.classList.remove('recording');
        elements.statusText.textContent = 'Not recording';
    }
    
    // Send a text message
    function sendTextMessage() {
        const text = elements.textMessageInput.value.trim();
        
        if (!text || !state.isConnected) {
            return;
        }
        
        // Add user message to chat
        addMessage('user', text);
        
        // Send message to server
        sendWebSocketMessage({
            type: 'text_message',
            text: text
        });
        
        // Clear input
        elements.textMessageInput.value = '';
    }
    
    // Send a message over WebSocket
    function sendWebSocketMessage(message) {
        console.log('[Client] Sending WebSocket message:', message);
        
        if (!state.isConnected) {
            console.warn('[Client] Cannot send message: WebSocket is not connected');
            showError('Cannot send message: Not connected to the server');
            return;
        }
        
        try {
            const jsonString = JSON.stringify(message);
            elements.messageSender.setAttribute('ws-send', jsonString);
            
            // Trigger the send
            const event = new CustomEvent('htmx:load', {
                bubbles: true,
                cancelable: true
            });
            
            elements.messageSender.dispatchEvent(event);
            console.log('[Client] Message sent successfully');
        } catch (error) {
            console.error('[Client] Error sending message:', error);
            showError('Error sending message to server');
        }
    }
    
    // Handle WebSocket messages (called by HTMX)
    htmx.on('#ws-events', 'ws-message', function(event) {
        const detail = event.detail;
        
        if (detail && detail.data) {
            const message = JSON.parse(detail.data);
            
            // Process the message based on its type
            switch (message.type) {
                case 'connection_established':
                    updateConnectionStatus(true);
                    state.sessionId = message.session_id || '';
                    if (state.sessionId) {
                        elements.sessionIdElement.textContent = `Session: ${state.sessionId}`;
                    }
                    break;
                    
                case 'session.created':
                    state.sessionId = message.item_id || '';
                    if (state.sessionId) {
                        elements.sessionIdElement.textContent = `Session: ${state.sessionId}`;
                    }
                    break;
                    
                case 'transcript':
                    updateOrCreateMessage('assistant', message.text, !message.is_final);
                    break;
                    
                case 'user_transcript':
                    updateOrCreateMessage('user', message.text, false, true);
                    break;
                    
                case 'audio_data':
                    playAudio(message.data);
                    break;
                    
                case 'error':
                    showError(message.message);
                    break;
            }
        }
    });
    
    // Update connection status
    function updateConnectionStatus(isConnected, statusText) {
        state.isConnected = isConnected;
        
        if (isConnected) {
            elements.connectionStatus.textContent = statusText || 'Connected';
            elements.connectionStatus.classList.remove('bg-warning', 'bg-danger');
            elements.connectionStatus.classList.add('bg-success');
            elements.startRecordingBtn.disabled = false;
        } else {
            elements.connectionStatus.textContent = statusText || 'Disconnected';
            elements.connectionStatus.classList.remove('bg-warning', 'bg-success');
            elements.connectionStatus.classList.add('bg-danger');
            elements.startRecordingBtn.disabled = true;
            elements.stopRecordingBtn.disabled = true;
            
            if (state.isRecording) {
                stopRecording();
            }
        }
    }
    
    // Play audio from base64-encoded data
    function playAudio(base64Audio) {
        if (!base64Audio) {
            return;
        }
        
        // Decode base64 data
        const audioData = atob(base64Audio);
        const arrayBuffer = new ArrayBuffer(audioData.length);
        const view = new Uint8Array(arrayBuffer);
        
        for (let i = 0; i < audioData.length; i++) {
            view[i] = audioData.charCodeAt(i);
        }
        
        // Play the audio
        state.audioContext.decodeAudioData(arrayBuffer)
            .then(buffer => {
                const source = state.audioContext.createBufferSource();
                source.buffer = buffer;
                source.connect(state.audioContext.destination);
                source.start(0);
            })
            .catch(error => {
                console.error('Error decoding audio data:', error);
            });
    }
    
    // Add a message to the chat
    function addMessage(role, content) {
        const messageElement = document.createElement('div');
        messageElement.classList.add('message', role);
        
        const roleLabel = document.createElement('div');
        roleLabel.classList.add('role-label');
        roleLabel.textContent = role === 'user' ? 'You' : 'Assistant';
        
        const contentElement = document.createElement('div');
        contentElement.classList.add('content');
        contentElement.textContent = content;
        
        messageElement.appendChild(roleLabel);
        messageElement.appendChild(contentElement);
        
        elements.chatMessages.appendChild(messageElement);
        
        // Scroll to bottom
        elements.chatMessages.scrollTop = elements.chatMessages.scrollHeight;
    }
    
    // Update or create a message in the chat
    function updateOrCreateMessage(role, content, isDelta = false, forceUpdate = false) {
        const messageElements = elements.chatMessages.querySelectorAll(`.message.${role}`);
        
        if (messageElements.length > 0 && (isDelta || forceUpdate)) {
            // Update the last message
            const lastMessage = messageElements[messageElements.length - 1];
            const contentElement = lastMessage.querySelector('.content');
            
            if (contentElement) {
                contentElement.textContent = content;
            }
        } else {
            // Create a new message
            addMessage(role, content);
        }
    }
    
    // Show an error message
    function showError(message) {
        console.error('Error:', message);
        
        const errorElement = document.createElement('div');
        errorElement.classList.add('alert', 'alert-danger', 'mt-3');
        errorElement.textContent = message;
        
        document.querySelector('.container').prepend(errorElement);
        
        // Remove after 5 seconds
        setTimeout(() => {
            errorElement.remove();
        }, 5000);
    }
    
    // Set up event listeners
    function setupEventListeners() {
        // HTMX WebSocket event listeners
        document.body.addEventListener('htmx:wsConnecting', (event) => {
            console.log('[Client] WebSocket connecting...', event.detail);
            updateConnectionStatus(false, 'Connecting...');
        });

        document.body.addEventListener('htmx:wsOpen', (event) => {
            console.log('[Client] WebSocket connection established', event.detail);
            updateConnectionStatus(true);
        });

        document.body.addEventListener('htmx:wsClose', (event) => {
            console.log('[Client] WebSocket connection closed', event.detail);
            updateConnectionStatus(false);
        });

        document.body.addEventListener('htmx:wsError', (event) => {
            console.error('[Client] WebSocket error', event.detail);
            updateConnectionStatus(false, 'Connection Error');
            showError('WebSocket connection error. Please refresh the page to try again.');
        });

        document.body.addEventListener('htmx:wsBeforeSend', (event) => {
            console.log('[Client] WebSocket sending message', event.detail);
        });

        document.body.addEventListener('htmx:wsAfterMessage', (event) => {
            console.log('[Client] WebSocket received message', event.detail);
            
            // Process the message if it contains data
            const message = event.detail.message;
            if (message) {
                console.log('[Client] Processing WebSocket message:', message);
                
                // Handle different message types
                switch (message.type) {
                    case "connection_established":
                        console.log('[Client] Connection established with server');
                        break;
                        
                    case "session_created":
                        console.log('[Client] Session created with ID:', message.session_id);
                        updateConnectionStatus(true, 'Connected');
                        if (message.session_id) {
                            state.sessionId = message.session_id;
                            elements.sessionIdElement.textContent = `Session ID: ${state.sessionId}`;
                        }
                        break;
                    
                    // Handle other message types
                    // ...
                }
            }
        });

        // Button click event listeners
        elements.startRecordingBtn.addEventListener('click', startRecording);
        elements.stopRecordingBtn.addEventListener('click', stopRecording);
        elements.sendTextBtn.addEventListener('click', sendTextMessage);
        
        // Text input enter key
        elements.textMessageInput.addEventListener('keyup', (e) => {
            if (e.key === 'Enter') {
                sendTextMessage();
            }
        });
    }
    
    // Initialize the application
    function init() {
        // Set up event listeners
        setupEventListeners();
        
        // Update connection status
        updateConnectionStatus(false);
    }
    
    // Initialize the application
    init();
}); 