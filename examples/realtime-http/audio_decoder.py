#!/usr/bin/env python3
"""
FFmpeg-based audio decoder for real-time audio streaming.
"""
import logging
import subprocess
import threading
from typing import Optional

class FFmpegAudioDecoder:
    def __init__(self, input_format: str = 'mp4', output_sample_rate: int = 24000, output_channels: int = 1):
        self.logger = logging.getLogger("ffmpeg-decoder")
        self.input_format = input_format
        self.output_sample_rate = output_sample_rate
        self.output_channels = output_channels
        self.process: Optional[subprocess.Popen[bytes]] = None
        self.buffer = bytearray()
        self.lock = threading.Lock()
        # Calculate minimum buffer size for 100ms of audio
        # 24000 samples/sec * 2 bytes/sample * 0.1 sec = 4800 bytes
        self.min_buffer_size = int(output_sample_rate * 2 * 0.1)  # 100ms of audio
        self.logger.info(f"Minimum buffer size for 100ms: {self.min_buffer_size} bytes")
        self.start_process()
        
    def start_process(self):
        """Start the FFmpeg process for audio decoding"""
        self.logger.info(f"Starting FFmpeg process: {self.input_format} -> PCM")
        
        self.process = subprocess.Popen([
            'ffmpeg',
            '-hide_banner',
            '-loglevel', 'error',
            '-protocol_whitelist', 'pipe,udp,rtp',
            '-f', self.input_format,
            '-i', 'pipe:0',
            '-f', 's16le',  # 16-bit signed little-endian PCM
            '-ar', str(self.output_sample_rate),
            '-ac', str(self.output_channels),
            'pipe:1'
        ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        
        # Start a thread to continuously read from FFmpeg's stdout
        self.read_thread = threading.Thread(target=self._read_output, daemon=True)
        self.read_thread.start()
    
    def _read_output(self):
        """Continuously read from FFmpeg's stdout"""
        while self.process and self.process.poll() is None:
            try:
                # Read in small chunks to reduce latency
                if self.process.stdout:
                    data = self.process.stdout.read(4096)
                    if data:
                        with self.lock:
                            self.buffer.extend(data)
            except Exception as e:
                self.logger.error(f"Error reading from FFmpeg: {e}")
                break
    
    def decode_chunk(self, chunk_data: bytes):
        """Feed a chunk of MP4 data to FFmpeg"""
        if not self.process or self.process.poll() is not None:
            self.logger.warning("FFmpeg process not running, restarting...")
            self.start_process()
            
        try:
            # Write the chunk to FFmpeg's stdin
            if self.process and self.process.stdin:
                self.process.stdin.write(chunk_data)
                self.process.stdin.flush()
        except Exception as e:
            self.logger.error(f"Error feeding data to FFmpeg: {e}")
            # Restart the process on error
            self.start_process()
    
    def get_decoded_audio(self, max_size: Optional[int] = None) -> Optional[bytes]:
        """Get decoded PCM audio from the buffer if enough data is available"""
        with self.lock:
            buffer_size = len(self.buffer)
            
            # If buffer is smaller than minimum required size, return None
            if buffer_size < self.min_buffer_size:
                self.logger.debug(f"Buffer too small ({buffer_size} bytes), need at least {self.min_buffer_size} bytes")
                return None
                
            if max_size and buffer_size > max_size:
                # Return a portion of the buffer
                result = bytes(self.buffer[:max_size])
                self.buffer = self.buffer[max_size:]
                self.logger.debug(f"Returning {len(result)} bytes, {len(self.buffer)} bytes left in buffer")
            else:
                # Return the entire buffer
                result = bytes(self.buffer)
                self.buffer = bytearray()
                self.logger.debug(f"Returning entire buffer: {len(result)} bytes")
                
        return result
    
    def has_enough_data(self) -> bool:
        """Check if the buffer has enough data to meet minimum requirements"""
        with self.lock:
            return len(self.buffer) >= self.min_buffer_size
    
    def close(self):
        """Clean up resources"""
        if self.process:
            try:
                if self.process.stdin:
                    self.process.stdin.close()
                self.process.terminate()
                self.process.wait(timeout=2)
            except:
                self.process.kill()
            finally:
                self.process = None 