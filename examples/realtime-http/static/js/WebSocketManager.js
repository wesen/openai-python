/**
 * Manages WebSocket connections and message handling
 */
class WebSocketManager {
  /**
   * Initialize the WebSocket manager
   * @param {Object} config - Configuration options
   * @param {Function} onMessageCallback - Callback for handling messages
   * @param {Function} onConnectionStatusChange - Callback for connection status changes
   */
  constructor(config, onMessageCallback, onConnectionStatusChange) {
    this.socket = null;
    this.isConnected = false;
    this.reconnectAttempts = 0;
    this.config = config || {
      reconnection: {
        baseDelay: 1000,
        maxDelay: 30000,
        factor: 1.5
      }
    };
    this.onMessageCallback = onMessageCallback;
    this.onConnectionStatusChange = onConnectionStatusChange;
  }

  /**
   * Connect to the WebSocket server
   */
  connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    console.log(`Connecting to WebSocket at ${wsUrl}`);
    
    // Close existing socket if it exists
    if (this.socket) {
      console.log('Closing existing WebSocket connection');
      this.socket.close();
    }
    
    this.socket = new WebSocket(wsUrl);
    
    this.socket.onopen = () => {
      console.log('WebSocket connection established');
      this.isConnected = true;
      
      // Reset reconnection attempts on successful connection
      this.reconnectAttempts = 0;
      
      // Notify about connection status change
      if (this.onConnectionStatusChange) {
        this.onConnectionStatusChange(true);
      }
    };
    
    this.socket.onclose = (event) => {
      console.log(`WebSocket connection closed: code=${event.code}, reason=${event.reason}`);
      this.isConnected = false;
      
      // Notify about connection status change
      if (this.onConnectionStatusChange) {
        this.onConnectionStatusChange(false);
      }
      
      // Try to reconnect with exponential backoff
      const { baseDelay, factor, maxDelay } = this.config.reconnection;
      const delay = Math.min(baseDelay * Math.pow(factor, this.reconnectAttempts), maxDelay);
      this.reconnectAttempts++;
      
      console.log(`Will attempt to reconnect in ${delay/1000} seconds (attempt ${this.reconnectAttempts})`);
      setTimeout(() => this.connect(), delay);
    };
    
    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
      // Notify about connection error
      if (this.onConnectionStatusChange) {
        this.onConnectionStatusChange(false, 'Connection Error');
      }
    };
    
    this.socket.onmessage = (event) => this.handleMessage(event);
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect() {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
      this.isConnected = false;
    }
  }

  /**
   * Send a message to the server
   * @param {string} type - Message type
   * @param {Object} data - Message data
   * @returns {boolean} - Whether the message was sent successfully
   */
  sendMessage(type, data = {}) {
    if (!this.isConnected || !this.socket) {
      console.error(`Cannot send message of type ${type}: Not connected to server`);
      return false;
    }
    
    try {
      const message = { type, ...data };
      this.socket.send(JSON.stringify(message));
      return true;
    } catch (error) {
      console.error(`Error sending message of type ${type}:`, error);
      return false;
    }
  }

  /**
   * Handle incoming WebSocket messages
   * @param {MessageEvent} event - WebSocket message event
   */
  handleMessage(event) {
    try {
      const message = JSON.parse(event.data);
      console.log('Received message:', message); // Debug all incoming messages
      
      // Pass the message to the callback
      if (this.onMessageCallback) {
        this.onMessageCallback(message);
      }
    } catch (error) {
      console.error('Error parsing WebSocket message:', error);
    }
  }
}

export default WebSocketManager; 