/**
 * Audio Visualizer for rendering audio waveforms
 */
class AudioVisualizer {
    /**
     * Initialize the audio visualizer
     * @param {string} canvasId - ID of the canvas element to render on
     * @param {AudioProcessor} audioProcessor - Audio processor instance
     */
    constructor(canvasId, audioProcessor) {
        this.canvas = document.getElementById(canvasId);
        this.ctx = this.canvas.getContext('2d');
        this.audioProcessor = audioProcessor;
        this.isAnimating = false;
        this.animationId = null;
        
        // Set canvas dimensions
        this.resizeCanvas();
        
        // Handle window resize
        window.addEventListener('resize', () => this.resizeCanvas());
    }
    
    /**
     * Resize the canvas to match its container
     */
    resizeCanvas() {
        const container = this.canvas.parentElement;
        this.canvas.width = container.clientWidth;
        this.canvas.height = container.clientHeight;
    }
    
    /**
     * Start the visualization animation
     */
    start() {
        if (this.isAnimating) return;
        
        this.isAnimating = true;
        this.draw();
    }
    
    /**
     * Stop the visualization animation
     */
    stop() {
        this.isAnimating = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        
        // Clear the canvas
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    }
    
    /**
     * Draw the audio visualization
     */
    draw() {
        if (!this.isAnimating) return;
        
        // Get the audio data
        const dataArray = this.audioProcessor.getVisualizationData();
        if (!dataArray) return;
        
        // Clear the canvas
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        
        // Set up drawing parameters
        const barWidth = (this.canvas.width / dataArray.length) * 2.5;
        const barSpacing = 1;
        let x = 0;
        
        // Draw each frequency bar
        for (let i = 0; i < dataArray.length; i++) {
            const barHeight = (dataArray[i] / 255) * this.canvas.height * 0.8;
            
            // Create gradient
            const gradient = this.ctx.createLinearGradient(0, this.canvas.height - barHeight, 0, this.canvas.height);
            gradient.addColorStop(0, '#10a37f');
            gradient.addColorStop(1, '#0d8c6d');
            
            this.ctx.fillStyle = gradient;
            this.ctx.fillRect(x, this.canvas.height - barHeight, barWidth, barHeight);
            
            x += barWidth + barSpacing;
        }
        
        // Continue animation loop
        this.animationId = requestAnimationFrame(() => this.draw());
    }
    
    /**
     * Update the visualization for incoming audio
     * @param {boolean} isActive - Whether audio is active
     */
    updateForAudio(isActive) {
        if (isActive) {
            this.start();
        } else {
            // Keep visualizer running but with reduced intensity
            // This is handled by the audio processor data
        }
    }
} 

export default AudioVisualizer;