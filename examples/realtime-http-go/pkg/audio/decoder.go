package audio

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/rs/zerolog/log"
	"errors"
)

// FFmpegDecoder handles audio transcoding using FFmpeg
type FFmpegDecoder struct {
	// FFmpeg process
	cmd *exec.Cmd
	// Stdin writer for FFmpeg process
	stdinWriter io.WriteCloser
	// Stdout reader for FFmpeg process
	stdoutReader io.ReadCloser
	// Buffer for decoded PCM data
	buffer      bytes.Buffer
	bufferMutex sync.Mutex

	// Audio configuration
	inputFormat     string
	outputSampleRate int
	outputChannels   int
	minBufferSize    int

	// Process status
	running bool
	// Mutex to protect running status
	runningMutex sync.Mutex
}

// NewFFmpegDecoder creates a new FFmpeg decoder
func NewFFmpegDecoder(inputFormat string, outputSampleRate, outputChannels, minBufferSize int) (*FFmpegDecoder, error) {
	decoder := &FFmpegDecoder{
		inputFormat:      inputFormat,
		outputSampleRate: outputSampleRate,
		outputChannels:   outputChannels,
		minBufferSize:    minBufferSize,
	}

	err := decoder.start()
	if err != nil {
		return nil, err
	}

	return decoder, nil
}

// start starts the FFmpeg process
func (d *FFmpegDecoder) start() error {
	d.runningMutex.Lock()
	defer d.runningMutex.Unlock()

	if d.running {
		return nil
	}

	log.Info().Str("input_format", d.inputFormat).Int("sample_rate", d.outputSampleRate).Int("channels", d.outputChannels).Msg("Starting FFmpeg process")

	// Build FFmpeg command
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-protocol_whitelist", "pipe,udp,rtp",
		"-f", d.inputFormat,
		"-i", "pipe:0",
		"-f", "s16le", // 16-bit signed little-endian PCM
		"-ar", fmt.Sprintf("%d", d.outputSampleRate),
		"-ac", fmt.Sprintf("%d", d.outputChannels),
		"pipe:1",
	)

	// Set up stdin and stdout pipes
	stdinWriter, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		stdinWriter.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		stdinWriter.Close()
		return fmt.Errorf("failed to start FFmpeg process: %w", err)
	}

	d.cmd = cmd
	d.stdinWriter = stdinWriter
	d.stdoutReader = stdoutReader
	d.running = true

	// Start goroutine to read FFmpeg output
	go d.readOutput()

	return nil
}

// readOutput continuously reads from FFmpeg's stdout
func (d *FFmpegDecoder) readOutput() {
	buffer := make([]byte, 4096)
	for {
		n, err := d.stdoutReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("Error reading from FFmpeg stdout")
			}
			break
		}

		if n > 0 {
			d.bufferMutex.Lock()
			d.buffer.Write(buffer[:n])
			d.bufferMutex.Unlock()
		}
	}

	// Process has exited
	d.runningMutex.Lock()
	d.running = false
	d.runningMutex.Unlock()
}

// DecodeChunk decodes a chunk of audio data
func (d *FFmpegDecoder) DecodeChunk(data []byte) error {
	d.runningMutex.Lock()
	if !d.running {
		d.runningMutex.Unlock()
		if err := d.start(); err != nil {
			return fmt.Errorf("failed to restart FFmpeg process: %w", err)
		}
		d.runningMutex.Lock()
	}
	d.runningMutex.Unlock()

	// Write data to FFmpeg stdin
	_, err := d.stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to FFmpeg stdin: %w", err)
	}

	return nil
}

// DecodeBase64Chunk decodes a base64 encoded chunk of audio data
func (d *FFmpegDecoder) DecodeBase64Chunk(base64Data string) error {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	return d.DecodeChunk(data)
}

// GetDecodedAudio returns decoded PCM audio data
func (d *FFmpegDecoder) GetDecodedAudio() ([]byte, error) {
	d.bufferMutex.Lock()
	defer d.bufferMutex.Unlock()

	if d.buffer.Len() < d.minBufferSize {
		return nil, nil
	}

	data := d.buffer.Bytes()
	result := make([]byte, len(data))
	copy(result, data)
	d.buffer.Reset()

	return result, nil
}

// HasEnoughData checks if there is enough data in the buffer
func (d *FFmpegDecoder) HasEnoughData() bool {
	d.bufferMutex.Lock()
	defer d.bufferMutex.Unlock()

	return d.buffer.Len() >= d.minBufferSize
}

// Close closes the FFmpeg process
func (d *FFmpegDecoder) Close() error {
	d.runningMutex.Lock()
	defer d.runningMutex.Unlock()

	if !d.running {
		return nil
	}

	// Try to close stdin gracefully
	if d.stdinWriter != nil {
		d.stdinWriter.Close()
	}

	// Wait for the process to exit
	err := d.cmd.Wait()
	if err != nil {
		log.Error().Err(err).Msg("Error waiting for FFmpeg process to exit")
	}

	d.running = false
	return nil
}

// TranscodeAudio transcodes audio data from one format to PCM16 at 24kHz
// This is a utility function for one-off transcoding without setting up a decoder
func TranscodeAudio(data []byte, inputFormat string, outputSampleRate, outputChannels int) ([]byte, error) {
	// Build FFmpeg command
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", inputFormat,
		"-i", "pipe:0",
		"-f", "s16le", // 16-bit signed little-endian PCM
		"-ar", fmt.Sprintf("%d", outputSampleRate),
		"-ac", fmt.Sprintf("%d", outputChannels),
		"pipe:1",
	)

	// Set up stdin and stdout pipes
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("failed to start FFmpeg process: %w", err)
	}

	// Write data to FFmpeg stdin
	go func() {
		defer stdinPipe.Close()
		stdinPipe.Write(data)
	}()

	// Read FFmpeg stdout
	var output bytes.Buffer
	if _, err := io.Copy(&output, stdoutPipe); err != nil {
		return nil, fmt.Errorf("failed to read FFmpeg stdout: %w", err)
	}

	// Wait for the process to exit
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("FFmpeg process failed: %w", err)
	}

	return output.Bytes(), nil
}

// TranscodeBase64Audio transcodes base64 encoded audio data
func TranscodeBase64Audio(base64Data string, inputFormat string, outputSampleRate, outputChannels int) (string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Transcode audio
	pcmData, err := TranscodeAudio(data, inputFormat, outputSampleRate, outputChannels)
	if err != nil {
		return "", err
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(pcmData), nil
}

// FFmpegEncoder encodes PCM audio to other formats
type FFmpegEncoder struct {
	// FFmpeg process
	cmd *exec.Cmd
	// Stdin writer for FFmpeg process
	stdinWriter io.WriteCloser
	// Stdout reader for FFmpeg process
	stdoutReader io.ReadCloser
	// Buffer for encoded data
	buffer      bytes.Buffer
	bufferMutex sync.Mutex

	// Audio configuration
	outputFormat    string
	inputSampleRate int
	inputChannels   int

	// Process status
	running bool
	// Mutex to protect running status
	runningMutex sync.Mutex
}

// NewFFmpegEncoder creates a new FFmpeg encoder
func NewFFmpegEncoder(outputFormat string, inputSampleRate, inputChannels int) (*FFmpegEncoder, error) {
	// Check if FFmpeg is installed
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("ffmpeg not found in PATH, please install FFmpeg")
	}

	encoder := &FFmpegEncoder{
		outputFormat:    outputFormat,
		inputSampleRate: inputSampleRate,
		inputChannels:   inputChannels,
	}

	err = encoder.start()
	if err != nil {
		return nil, err
	}

	return encoder, nil
}

// start starts the FFmpeg process
func (e *FFmpegEncoder) start() error {
	e.runningMutex.Lock()
	defer e.runningMutex.Unlock()

	if e.running {
		return nil
	}

	log.Info().Str("output_format", e.outputFormat).Int("sample_rate", e.inputSampleRate).Int("channels", e.inputChannels).Msg("Starting FFmpeg encoder process")

	// Build FFmpeg command
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "s16le", // Input is 16-bit signed little-endian PCM
		"-ar", fmt.Sprintf("%d", e.inputSampleRate),
		"-ac", fmt.Sprintf("%d", e.inputChannels),
		"-i", "pipe:0",
		"-f", e.outputFormat,
		"pipe:1",
	)

	// Set up stdin and stdout pipes
	stdinWriter, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		stdinWriter.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		stdinWriter.Close()
		return fmt.Errorf("failed to start FFmpeg process: %w", err)
	}

	e.cmd = cmd
	e.stdinWriter = stdinWriter
	e.stdoutReader = stdoutReader
	e.running = true

	// Start goroutine to read FFmpeg output
	go e.readOutput()

	return nil
}

// readOutput continuously reads from FFmpeg's stdout
func (e *FFmpegEncoder) readOutput() {
	buffer := make([]byte, 4096)
	for {
		n, err := e.stdoutReader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Msg("Error reading from FFmpeg stdout")
			}
			break
		}

		if n > 0 {
			e.bufferMutex.Lock()
			e.buffer.Write(buffer[:n])
			e.bufferMutex.Unlock()
		}
	}

	// Process has exited
	e.runningMutex.Lock()
	e.running = false
	e.runningMutex.Unlock()
}

// EncodeChunk encodes a chunk of PCM audio data to the output format
func (e *FFmpegEncoder) EncodeChunk(data []byte) error {
	e.runningMutex.Lock()
	if !e.running {
		e.runningMutex.Unlock()
		if err := e.start(); err != nil {
			return fmt.Errorf("failed to restart FFmpeg process: %w", err)
		}
		e.runningMutex.Lock()
	}
	e.runningMutex.Unlock()

	// Write data to FFmpeg stdin
	_, err := e.stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to FFmpeg stdin: %w", err)
	}

	return nil
}

// EncodeBase64Chunk encodes a base64 encoded chunk of PCM audio data
func (e *FFmpegEncoder) EncodeBase64Chunk(base64Data string) error {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	return e.EncodeChunk(data)
}

// GetEncodedAudio returns encoded audio data
func (e *FFmpegEncoder) GetEncodedAudio() ([]byte, error) {
	e.bufferMutex.Lock()
	defer e.bufferMutex.Unlock()

	if e.buffer.Len() == 0 {
		return nil, nil
	}

	data := e.buffer.Bytes()
	result := make([]byte, len(data))
	copy(result, data)
	e.buffer.Reset()

	return result, nil
}

// GetEncodedBase64Audio returns encoded audio data as base64
func (e *FFmpegEncoder) GetEncodedBase64Audio() (string, error) {
	data, err := e.GetEncodedAudio()
	if err != nil {
		return "", err
	}

	if data == nil {
		return "", nil
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// Close closes the FFmpeg process
func (e *FFmpegEncoder) Close() error {
	e.runningMutex.Lock()
	defer e.runningMutex.Unlock()

	if !e.running {
		return nil
	}

	// Try to close stdin gracefully
	if e.stdinWriter != nil {
		e.stdinWriter.Close()
	}

	// Wait for the process to exit
	err := e.cmd.Wait()
	if err != nil {
		log.Error().Err(err).Msg("Error waiting for FFmpeg process to exit")
	}

	e.running = false
	return nil
}

// EncodePCM encodes PCM audio data to another format
func EncodePCM(data []byte, outputFormat string, inputSampleRate, inputChannels int) ([]byte, error) {
	// Check if FFmpeg is installed
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("ffmpeg not found in PATH, please install FFmpeg")
	}

	// Build FFmpeg command
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "s16le", // Input is 16-bit signed little-endian PCM
		"-ar", fmt.Sprintf("%d", inputSampleRate),
		"-ac", fmt.Sprintf("%d", inputChannels),
		"-i", "pipe:0",
		"-f", outputFormat,
		"pipe:1",
	)

	// Set up stdin and stdout pipes
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("failed to start FFmpeg process: %w", err)
	}

	// Write data to FFmpeg stdin
	go func() {
		defer stdinPipe.Close()
		stdinPipe.Write(data)
	}()

	// Read FFmpeg stdout
	var output bytes.Buffer
	if _, err := io.Copy(&output, stdoutPipe); err != nil {
		return nil, fmt.Errorf("failed to read FFmpeg stdout: %w", err)
	}

	// Wait for the process to exit
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("FFmpeg process failed: %w", err)
	}

	return output.Bytes(), nil
}

// EncodeBase64PCM encodes base64 PCM audio data to another format
func EncodeBase64PCM(base64Data string, outputFormat string, inputSampleRate, inputChannels int) (string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Encode audio
	encodedData, err := EncodePCM(data, outputFormat, inputSampleRate, inputChannels)
	if err != nil {
		return "", err
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(encodedData), nil
}

// CheckFFmpegInstalled checks if FFmpeg is installed
func CheckFFmpegInstalled() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}