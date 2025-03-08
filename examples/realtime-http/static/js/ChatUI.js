/**
 * Manages the chat interface
 */
class ChatUI {
  /**
   * Initialize the chat UI
   * @param {Object} elements - DOM elements
   */
  constructor(elements) {
    this.elements = elements;
    this.chatMessages = elements.chatMessages;
    this.textMessageInput = elements.textMessageInput;
  }

  /**
   * Add a message to the chat
   * @param {string} role - 'user' or 'ai'
   * @param {string} content - Message content
   * @returns {HTMLElement} - The created message element
   */
  addMessage(role, content) {
    const messageDiv = document.createElement('div');
    messageDiv.classList.add('message', `${role}-message`);
    messageDiv.setAttribute('data-role', role);
    messageDiv.textContent = content;
    
    this.chatMessages.appendChild(messageDiv);
    this.scrollToBottom();
    
    return messageDiv;
  }

  /**
   * Update an existing message or create a new one
   * @param {string} role - 'user' or 'ai'
   * @param {string} content - Message content
   * @param {boolean} isDelta - Whether this is a delta update
   * @param {boolean} forceUpdate - Whether to force update the last message of this role
   * @returns {HTMLElement} - The updated or created message element
   */
  updateMessage(role, content, isDelta = false, forceUpdate = false) {
    // Find the last message with this role
    const messages = this.chatMessages.querySelectorAll(`.${role}-message`);
    const lastMessage = messages[messages.length - 1];
    
    if ((lastMessage && isDelta) || (lastMessage && forceUpdate)) {
      // Update existing message for deltas or when forced
      lastMessage.textContent = isDelta ? lastMessage.textContent + content : content;
      this.scrollToBottom();
      return lastMessage;
    } else {
      // Create new message
      return this.addMessage(role, content);
    }
  }

  /**
   * Show an error message
   * @param {string} message - Error message
   * @returns {HTMLElement} - The created error message element
   */
  showError(message) {
    console.error(message);
    
    // Add error message to chat
    const errorDiv = document.createElement('div');
    errorDiv.classList.add('message', 'error-message');
    errorDiv.textContent = `Error: ${message}`;
    
    this.chatMessages.appendChild(errorDiv);
    this.scrollToBottom();
    
    return errorDiv;
  }

  /**
   * Clear the input field
   */
  clearInput() {
    if (this.textMessageInput) {
      this.textMessageInput.value = '';
    }
  }

  /**
   * Get the current input text
   * @returns {string} - The current input text
   */
  getInputText() {
    return this.textMessageInput ? this.textMessageInput.value.trim() : '';
  }

  /**
   * Scroll the chat to the bottom
   */
  scrollToBottom() {
    if (this.chatMessages) {
      this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
    }
  }

  /**
   * Update the connection status display
   * @param {boolean} isConnected - Whether the connection is established
   * @param {string} statusText - Optional status text to display
   */
  updateConnectionStatus(isConnected, statusText = null) {
    if (this.elements.connectionStatus) {
      this.elements.connectionStatus.textContent = statusText || (isConnected ? 'Connected' : 'Disconnected');
    }
  }

  /**
   * Update the session ID display
   * @param {string} sessionId - The session ID
   */
  updateSessionId(sessionId) {
    if (this.elements.sessionIdElement) {
      this.elements.sessionIdElement.textContent = `Session: ${sessionId || 'None'}`;
    }
  }

  /**
   * Update the recording status display
   * @param {boolean} isRecording - Whether recording is active
   */
  updateRecordingStatus(isRecording) {
    if (this.elements.statusIndicator && this.elements.statusText) {
      if (isRecording) {
        this.elements.statusIndicator.classList.add('recording');
        this.elements.statusText.textContent = 'Recording...';
      } else {
        this.elements.statusIndicator.classList.remove('recording');
        this.elements.statusText.textContent = 'Not recording';
      }
    }
  }

  /**
   * Enable or disable the recording buttons
   * @param {boolean} canStart - Whether the start button should be enabled
   * @param {boolean} canStop - Whether the stop button should be enabled
   */
  updateRecordingButtons(canStart, canStop) {
    if (this.elements.startRecordingBtn) {
      this.elements.startRecordingBtn.disabled = !canStart;
    }
    if (this.elements.stopRecordingBtn) {
      this.elements.stopRecordingBtn.disabled = !canStop;
    }
  }
}

export default ChatUI; 