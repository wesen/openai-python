/**
 * Main application logic for the OpenAI Realtime Audio Chat
 */
document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM loaded, initializing application');
    
    // DOM Elements
    const startRecordingBtn = document.getElementById('start-recording');
    const stopRecordingBtn = document.getElementById('stop-recording');
    const statusIndicator = document.getElementById('status-indicator');
    const statusText = document.getElementById('status-text');
    const connectionStatus = document.getElementById('connection-status');
    const sessionIdElement = document.getElementById('session-id');
    const chatMessages = document.getElementById('chat-messages');
    const textMessageInput = document.getElementById('text-message');
    const sendTextBtn = document.getElementById('send-text');

    console.log('DOM elements initialized');
    
    // Initialize audio processor
    const audioProcessor = new AudioProcessor();
    let visualizer = null;
    
    // WebSocket connection
    let socket = null;
    let isConnected = false;
    let sessionId = null;
    let reconnectAttempts = 0;
    
    /**
     * Initialize the application
     */
    async function init() {
        console.log('Initializing application');
        // Initialize audio
        console.log('Initializing audio processor');
        const audioInitialized = await audioProcessor.initialize();
        if (!audioInitialized) {
            showError('Failed to initialize audio. Please check your microphone permissions.');
            console.error('Audio initialization failed');
            return;
        }
        console.log('Audio processor initialized successfully');
        
        // Initialize visualizer
        console.log('Initializing audio visualizer');
        visualizer = new AudioVisualizer('visualizer', audioProcessor);
        console.log('Audio visualizer initialized');
        
        // Connect to WebSocket
        console.log('Connecting to WebSocket');
        connectWebSocket();
        
        // Set up event listeners
        console.log('Setting up event listeners');
        setupEventListeners();
        console.log('Application initialization complete');
    }
    
    /**
     * Connect to the WebSocket server
     */
    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        console.log(`Connecting to WebSocket at ${wsUrl}`);
        
        // Close existing socket if it exists
        if (socket) {
            console.log('Closing existing WebSocket connection');
            socket.close();
        }
        
        socket = new WebSocket(wsUrl);
        
        socket.onopen = () => {
            console.log('WebSocket connection established');
            connectionStatus.textContent = 'Connected';
            isConnected = true;
            
            // Reset reconnection attempts on successful connection
            reconnectAttempts = 0;
        };
        
        socket.onclose = (event) => {
            console.log(`WebSocket connection closed: code=${event.code}, reason=${event.reason}`);
            connectionStatus.textContent = 'Disconnected';
            isConnected = false;
            
            // Disable recording buttons when disconnected
            startRecordingBtn.disabled = true;
            stopRecordingBtn.disabled = true;
            
            // Try to reconnect with exponential backoff
            const delay = Math.min(1000 * Math.pow(1.5, reconnectAttempts), 30000);
            reconnectAttempts++;
            
            console.log(`Will attempt to reconnect in ${delay/1000} seconds (attempt ${reconnectAttempts})`);
            setTimeout(connectWebSocket, delay);
        };
        
        socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            connectionStatus.textContent = 'Connection Error';
        };
        
        socket.onmessage = handleWebSocketMessage;
    }
    
    /**
     * Handle incoming WebSocket messages
     * @param {MessageEvent} event - WebSocket message event
     */
    function handleWebSocketMessage(event) {
        try {
            const message = JSON.parse(event.data);
            console.log('Received message:', message); // Debug all incoming messages
            
            switch (message.type) {
                case 'session.created':
                    sessionId = message.session_id;
                    sessionIdElement.textContent = `Session: ${sessionId}`;
                    console.log('Session created with ID:', sessionId);
                    break;
                    
                case 'audio_stream_start':
                    // A new audio stream is starting, clear any previous audio
                    console.log('New audio stream starting:', message.item_id);
                    audioProcessor.handleNewAudioStream();
                    visualizer.updateForAudio(true);
                    break;
                    
                case 'audio_data':
                    // Play the audio
                    console.log('Received audio data, length:', message.data.length);
                    audioProcessor.playAudio(message.data);
                    visualizer.updateForAudio(true);
                    break;
                    
                case 'transcript':
                    // Always update the existing AI message or create a new one
                    console.log('Received transcript:', message.text);
                    updateOrCreateMessage('ai', message.text, false, true);
                    break;
                    
                case 'text_delta':
                    // Always update the existing AI message or create a new one
                    console.log('Received text delta:', message.delta);
                    updateOrCreateMessage('ai', message.delta, true, true);
                    break;
                    
                case 'response.done':
                    // Response is complete
                    console.log('Response completed');
                    visualizer.updateForAudio(false);
                    break;
                    
                case 'error':
                    console.error('Error from server:', message.message);
                    showError(message.message);
                    break;
                    
                case 'text_message_received':
                    // Text message was received by the server
                    console.log('Text message received confirmation');
                    audioProcessor.stopAllAudio();
                    break;
                    
                case 'audio_data_received':
                    // Audio data was received by the server
                    console.log('Audio data received confirmation');
                    break;
                    
                case 'audio_end_received':
                    // Audio end signal was received by the server
                    console.log('Audio end received confirmation');
                    break;
                    
                default:
                    console.log('Received unknown message type:', message.type);
            }
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
        }
    }
    
    /**
     * Set up event listeners for UI elements
     */
    function setupEventListeners() {
        console.log('Setting up event listeners for UI elements');
        
        // Start recording button
        startRecordingBtn.addEventListener('click', () => {
            console.log('Start recording button clicked');
            if (!isConnected) {
                console.error('Cannot start recording: Not connected to server');
                showError('Not connected to server');
                return;
            }
            
            console.log('Attempting to start recording');
            const started = audioProcessor.startRecording((base64data, formatInfo) => {
                if (isConnected) {
                    try {
                        // If format info is provided (first chunk), send it to the server
                        if (formatInfo) {
                            console.log('Sending audio format info to server:', formatInfo);
                            socket.send(JSON.stringify({
                                type: 'audio_format',
                                format: formatInfo
                            }));
                        }
                        
                        // Send the audio data
                        console.log('Sending audio data to server, length:', base64data.length);
                        socket.send(JSON.stringify({
                            type: 'audio_data',
                            data: base64data
                        }));
                    } catch (error) {
                        console.error('Error sending audio data:', error);
                        showError('Error sending audio data. Connection may be lost.');
                        audioProcessor.stopRecording();
                        startRecordingBtn.disabled = false;
                        stopRecordingBtn.disabled = true;
                        statusIndicator.classList.remove('recording');
                        statusText.textContent = 'Not recording';
                    }
                } else {
                    console.warn('Cannot send audio data: WebSocket not connected');
                    showError('WebSocket connection lost. Please try again.');
                    audioProcessor.stopRecording();
                    startRecordingBtn.disabled = false;
                    stopRecordingBtn.disabled = true;
                    statusIndicator.classList.remove('recording');
                    statusText.textContent = 'Not recording';
                }
            });
            
            if (started) {
                console.log('Recording started successfully');
                startRecordingBtn.disabled = true;
                stopRecordingBtn.disabled = false;
                statusIndicator.classList.add('recording');
                statusText.textContent = 'Recording...';
                visualizer.start();
            } else {
                console.error('Failed to start recording');
                showError('Failed to start recording. Please check microphone permissions.');
            }
        });
        
        // Stop recording button
        stopRecordingBtn.addEventListener('click', () => {
            console.log('Stop recording button clicked');
            const stopped = audioProcessor.stopRecording();
            
            if (stopped && isConnected) {
                console.log('Sending audio_end signal to server');
                try {
                    socket.send(JSON.stringify({
                        type: 'audio_end'
                    }));
                } catch (error) {
                    console.error('Error sending audio_end signal:', error);
                    showError('Error sending audio end signal. Connection may be lost.');
                }
            } else {
                console.warn('Could not send audio_end: recording not stopped or WebSocket not connected');
            }
            
            startRecordingBtn.disabled = false;
            stopRecordingBtn.disabled = true;
            statusIndicator.classList.remove('recording');
            statusText.textContent = 'Not recording';
        });
        
        // Connection status observer - enable/disable buttons based on connection status
        const connectionObserver = new MutationObserver((mutations) => {
            mutations.forEach((mutation) => {
                if (mutation.type === 'characterData' || mutation.type === 'childList') {
                    if (connectionStatus.textContent === 'Connected') {
                        startRecordingBtn.disabled = false;
                    } else {
                        startRecordingBtn.disabled = true;
                        stopRecordingBtn.disabled = true;
                    }
                }
            });
        });
        
        connectionObserver.observe(connectionStatus, { 
            characterData: true, 
            childList: true,
            subtree: true 
        });
        
        // Send text button
        sendTextBtn.addEventListener('click', () => {
            console.log('Send text button clicked');
            sendTextMessage();
        });
        
        // Text input enter key
        textMessageInput.addEventListener('keypress', (event) => {
            if (event.key === 'Enter') {
                console.log('Enter key pressed in text input');
                sendTextMessage();
            }
        });
        
        // Handle page unload
        window.addEventListener('beforeunload', () => {
            console.log('Page unloading, cleaning up resources');
            audioProcessor.cleanup();
            if (socket) {
                socket.close();
            }
        });
        
        console.log('All event listeners set up');
    }
    
    /**
     * Send a text message to the server
     */
    function sendTextMessage() {
        const text = textMessageInput.value.trim();
        if (!text || !isConnected) {
            console.log('Cannot send message: text empty or not connected');
            return;
        }
        
        console.log('Sending text message:', text);
        
        // Stop any ongoing audio playback
        audioProcessor.stopAllAudio();
        
        // Add message to chat
        addMessage('user', text);
        
        // Send to server
        socket.send(JSON.stringify({
            type: 'text_message',
            text: text
        }));
        
        // Clear input
        textMessageInput.value = '';
    }
    
    /**
     * Add a message to the chat
     * @param {string} role - 'user' or 'ai'
     * @param {string} content - Message content
     */
    function addMessage(role, content) {
        const messageDiv = document.createElement('div');
        messageDiv.classList.add('message', `${role}-message`);
        messageDiv.setAttribute('data-role', role);
        messageDiv.textContent = content;
        
        chatMessages.appendChild(messageDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    /**
     * Update an existing message or create a new one
     * @param {string} role - 'user' or 'ai'
     * @param {string} content - Message content
     * @param {boolean} isDelta - Whether this is a delta update
     * @param {boolean} forceUpdate - Whether to force update the last message of this role
     */
    function updateOrCreateMessage(role, content, isDelta = false, forceUpdate = false) {
        // Find the last message with this role
        const messages = chatMessages.querySelectorAll(`.${role}-message`);
        const lastMessage = messages[messages.length - 1];
        
        if ((lastMessage && isDelta) || (lastMessage && forceUpdate)) {
            // Update existing message for deltas or when forced
            lastMessage.textContent = isDelta ? lastMessage.textContent + content : content;
        } else {
            // Create new message
            addMessage(role, content);
        }
        
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    /**
     * Show an error message
     * @param {string} message - Error message
     */
    function showError(message) {
        console.error(message);
        
        // Add error message to chat
        const errorDiv = document.createElement('div');
        errorDiv.classList.add('message', 'error-message');
        errorDiv.textContent = `Error: ${message}`;
        
        chatMessages.appendChild(errorDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    // Initialize the application
    init();
}); 