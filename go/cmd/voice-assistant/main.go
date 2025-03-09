package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/go-go-golems/openai-realtime/pkg/openai/realtime"
)

func main() {
	// Parse command-line flags
	apiKey := flag.String("api-key", "", "OpenAI API key (or set OPENAI_API_KEY env var)")
	voice := flag.String("voice", "echo", "Voice to use (alloy, echo, fable, onyx, nova, shimmer)")
	inputFile := flag.String("input", "", "Input audio file (WAV/PCM format)")
	outputFile := flag.String("output", "output.pcm", "Output audio file (PCM format)")
	instructions := flag.String("instructions", "You are a helpful assistant.", "System instructions for the assistant")
	textInput := flag.String("text", "", "Text input (instead of audio file)")
	flag.Parse()

	// Get API key from environment if not provided via flag
	if *apiKey == "" {
		*apiKey = os.Getenv("OPENAI_API_KEY")
		if *apiKey == "" {
			log.Fatal("OpenAI API key is required. Provide it with --api-key flag or OPENAI_API_KEY environment variable.")
		}
	}

	// Check that we have either text or audio input
	if *textInput == "" && *inputFile == "" {
		log.Fatal("Either --text or --input flag must be provided.")
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a new client
	client := realtime.NewClient(*apiKey, "")

	// Configure session
	config := &realtime.Config{
		Voice:        *voice,
		Modalities:   []string{"text", "audio"},
		Instructions: *instructions,
	}

	// Connect to the API
	fmt.Println("Connecting to OpenAI Realtime API...")
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close(ctx)

	// Update session with our configuration
	if err := client.UpdateSession(ctx, config); err != nil {
		log.Fatalf("Failed to update session: %v", err)
	}

	fmt.Printf("Connected. Using voice: %s\n", *voice)

	// Create a response assembler to help with handling responses
	assembler := realtime.NewResponseAssembler(client)

	// Start the event listener
	if err := client.ListenForEvents(ctx); err != nil {
		log.Fatalf("Failed to start event listener: %v", err)
	}

	// Process either text or audio input
	var responseText string
	var responseAudio []byte
	var err error

	if *textInput != "" {
		// Send text input
		fmt.Printf("Sending text: %s\n", *textInput)
		responseText, responseAudio, err = assembler.SendTextAndWaitForResponse(ctx, *textInput)
	} else {
		// Read the input audio file
		fmt.Printf("Reading audio from %s\n", *inputFile)
		audioData, err := ioutil.ReadFile(*inputFile)
		if err != nil {
			log.Fatalf("Failed to read audio file: %v", err)
		}

		// Send audio input
		fmt.Printf("Sending audio (%d bytes)\n", len(audioData))
		
		// In a real application, you would stream audio in chunks
		// Here, we simulate by sending the whole file at once
		responseText, responseAudio, err = assembler.SendAudioAndWaitForResponse(ctx, audioData, false)
	}

	if err != nil {
		log.Fatalf("Error processing input: %v", err)
	}

	// Output the response
	fmt.Printf("\nAssistant response (text):\n%s\n", responseText)
	fmt.Printf("Audio response size: %d bytes\n", len(responseAudio))

	// Save the audio to a file
	if err := ioutil.WriteFile(*outputFile, responseAudio, 0644); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}
	fmt.Printf("Audio saved to %s\n", *outputFile)
} 