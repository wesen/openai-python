// Web Socket client handling
document.addEventListener('DOMContentLoaded', function() {
    // WebSocket connection
    let ws;
    let mediaRecorder;
    let audioChunks = [];
    let sessionId = '';
    let isRecording = false;
    
    // Connect to WebSocket
    function connectWebSocket() {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
        const wsUrl = `${wsProtocol}${window.location.host}/ws`;
        
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            console.log('WebSocket connection established');
            document.getElementById('connection-status').textContent = 'Connected';
            document.getElementById('connect-button').disabled = true;
            document.getElementById('disconnect-button').disabled = false;
        };
        
        ws.onclose = function() {
            console.log('WebSocket connection closed');
            document.getElementById('connection-status').textContent = 'Disconnected';
            document.getElementById('connect-button').disabled = false;
            document.getElementById('disconnect-button').disabled = true;
            document.getElementById('session-id').textContent = '';
            sessionId = '';
        };
        
        ws.onerror = function(error) {
            console.error('WebSocket error:', error);
            document.getElementById('connection-status').textContent = 'Error';
        };
        
        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            } catch (error) {
                console.error('Error parsing WebSocket message:', error);
            }
        };
    }
    
    // Handle WebSocket messages
    function handleWebSocketMessage(data) {
        console.log('Received message:', data.type);
        
        switch(data.type) {
            case 'session.created':
                sessionId = data.session_id;
                document.getElementById('session-id').textContent = sessionId;
                document.getElementById('status-indicator').classList.add('connected');
                document.getElementById('status-text').textContent = 'Connected';
                break;
                
            case 'message':
                // Handle user/assistant message
                addMessageToChat(data.role, data.content);
                break;
                
            case 'response.text.delta':
                // Handle streaming text
                appendToLastMessage(data.delta);
                break;
                
            case 'response.audio.delta':
                // Handle streaming audio
                if (data.delta) {
                    playAudioBase64(data.delta);
                }
                break;
                
            case 'conversation.item.input_audio_transcription.completed':
                // Handle transcription
                if (data.transcript) {
                    addMessageToChat('user', data.transcript);
                }
                break;
                
            case 'error':
                console.error('OpenAI API error:', data.error);
                showError('OpenAI API error: ' + (data.error?.message || 'Unknown error'));
                break;
                
            case 'recording_started':
                updateRecordingStatus(true);
                break;
                
            case 'recording_stopped':
                updateRecordingStatus(false);
                break;
                
            case 'response.done':
                console.log('Response completed');
                break;
        }
    }
    
    // Add a message to the chat
    function addMessageToChat(role, content) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${role === 'user' ? 'user-message' : 'ai-message'}`;
        messageDiv.setAttribute('data-role', role);
        messageDiv.textContent = content;
        
        const chatMessages = document.getElementById('chat-messages');
        chatMessages.appendChild(messageDiv);
        
        // Scroll to bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    // Append text to the last message from the same role
    function appendToLastMessage(text) {
        const chatMessages = document.getElementById('chat-messages');
        let lastMessage = chatMessages.querySelector('.message.ai-message:last-child');
        
        if (!lastMessage || lastMessage.getAttribute('data-role') !== 'assistant') {
            lastMessage = document.createElement('div');
            lastMessage.className = 'message ai-message';
            lastMessage.setAttribute('data-role', 'assistant');
            lastMessage.textContent = text;
            chatMessages.appendChild(lastMessage);
        } else {
            lastMessage.textContent += text;
        }
        
        // Scroll to bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    // Show error message
    function showError(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        
        const chatMessages = document.getElementById('chat-messages');
        chatMessages.appendChild(errorDiv);
        
        // Scroll to bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
    
    // Update recording status UI
    function updateRecordingStatus(recording) {
        isRecording = recording;
        
        if (recording) {
            document.getElementById('start-recording').disabled = true;
            document.getElementById('stop-recording').disabled = false;
            document.getElementById('status-indicator').classList.add('recording');
            document.getElementById('status-text').textContent = 'Recording...';
        } else {
            document.getElementById('start-recording').disabled = false;
            document.getElementById('stop-recording').disabled = true;
            document.getElementById('status-indicator').classList.remove('recording');
            document.getElementById('status-text').textContent = 'Not recording';
        }
    }
    
    // Play audio from base64 string
    function playAudioBase64(base64Audio) {
        const audioContext = new (window.AudioContext || window.webkitAudioContext)();
        const byteCharacters = atob(base64Audio);
        const byteNumbers = new Array(byteCharacters.length);
        
        for (let i = 0; i < byteCharacters.length; i++) {
            byteNumbers[i] = byteCharacters.charCodeAt(i);
        }
        
        const byteArray = new Uint8Array(byteNumbers);
        
        audioContext.decodeAudioData(byteArray.buffer)
            .then(buffer => {
                const source = audioContext.createBufferSource();
                source.buffer = buffer;
                source.connect(audioContext.destination);
                source.start(0);
            })
            .catch(err => console.error('Error decoding audio data:', err));
    }
    
    // Start recording audio
    async function startRecording() {
        if (!sessionId) {
            showError('No active session. Please connect first.');
            return;
        }
        
        try {
            const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            
            mediaRecorder = new MediaRecorder(stream);
            audioChunks = [];
            
            mediaRecorder.ondataavailable = event => {
                if (event.data.size > 0) {
                    audioChunks.push(event.data);
                    
                    // Convert to base64 and send to WebSocket
                    const reader = new FileReader();
                    reader.onloadend = () => {
                        const base64Audio = reader.result.split(',')[1];
                        
                        if (ws && ws.readyState === WebSocket.OPEN) {
                            ws.send(JSON.stringify({
                                type: 'input_audio',
                                audio: base64Audio
                            }));
                        }
                    };
                    reader.readAsDataURL(event.data);
                }
            };
            
            mediaRecorder.onstop = () => {
                // When recording stops, commit the audio buffer
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({
                        type: 'commit_audio'
                    }));
                }
                
                // Stop tracks
                stream.getTracks().forEach(track => track.stop());
                updateRecordingStatus(false);
            };
            
            // Start recording
            mediaRecorder.start(100); // Capture in 100ms chunks
            updateRecordingStatus(true);
            
        } catch (error) {
            console.error('Error starting audio recording:', error);
            showError('Could not access microphone. Please check permissions.');
        }
    }
    
    // Stop recording audio
    function stopRecording() {
        if (mediaRecorder && mediaRecorder.state !== 'inactive') {
            mediaRecorder.stop();
        }
    }
    
    // Send text message
    function sendTextMessage(text) {
        if (!text.trim() || !sessionId) {
            return;
        }
        
        if (ws && ws.readyState === WebSocket.OPEN) {
            // Add message to chat
            addMessageToChat('user', text);
            
            // Send to WebSocket
            ws.send(JSON.stringify({
                type: 'input_text',
                text: text
            }));
        } else {
            showError('No active connection. Please connect first.');
        }
    }
    
    // Initialize UI
    function initUI() {
        // Connect button
        document.getElementById('connect-button').addEventListener('click', function() {
            connectWebSocket();
        });
        
        // Disconnect button
        document.getElementById('disconnect-button').addEventListener('click', function() {
            if (ws) {
                ws.close();
            }
        });
        
        // Start recording button
        document.getElementById('start-recording').addEventListener('click', function() {
            startRecording();
        });
        
        // Stop recording button
        document.getElementById('stop-recording').addEventListener('click', function() {
            stopRecording();
        });
        
        // Form submission for text input
        document.querySelector('.text-input').addEventListener('submit', function(event) {
            event.preventDefault();
            
            const input = document.getElementById('text-message');
            if (input.value.trim() === '') {
                return;
            }
            
            sendTextMessage(input.value);
            input.value = '';
        });
    }
    
    // Initialize
    initUI();
    connectWebSocket(); // Auto-connect on page load
});