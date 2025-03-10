# OpenAI Real-Time Voice API Specification

## Overview

The OpenAI Real-Time Voice API (often called the Realtime API) allows developers to create low-latency, multi-modal conversational experiences with voice. It supports **both text and audio** as inputs and outputs, and also offers **function calling** capabilities (Realtime API | OpenAI Help Center) (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs). In other words, you can have natural speech-to-speech conversations with an AI model over a persistent WebSocket connection, similar to ChatGPT’s voice mode (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs). Key benefits of the API include native speech-to-speech communication (no text intermediary, reducing latency), expressive and steerable voices, and simultaneous text + audio output for each response (Realtime API | OpenAI Help Center).

Under the hood, the Real-Time API uses a WebSocket interface to stream audio in and out. This means you maintain a persistent session with the model (currently GPT-4o) for continuous interaction (Introducing the Realtime API | OpenAI) (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs). The model can handle **interruptions** (barge-in) gracefully – if a user starts speaking over the AI, the API can detect it and stop the AI’s speech, similar to how ChatGPT’s voice mode works (Introducing the Realtime API | OpenAI). Developers no longer need to glue together separate speech recognition and synthesis systems; the Real-Time API handles speech recognition, language understanding, and speech synthesis in one unified service (Introducing the Realtime API | OpenAI).

## Authentication and Endpoint

To use the Real-Time Voice API, you must have an OpenAI API key with access to the beta. Authentication is handled via an **API key** passed in the `Authorization` header as a Bearer token, similar to other OpenAI APIs (OpenAI Realtime API: A Guide With Examples | DataCamp) (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs). Additionally, during the beta period, a special header `OpenAI-Beta: realtime=v1` must be included in requests to enable the realtime features (OpenAI Realtime API: A Guide With Examples | DataCamp) (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs).

**Connection details:** Communication occurs over WebSockets. The endpoint to initiate a connection is: 

- **WebSocket URL:** `wss://api.openai.com/v1/realtime`  
- **Query parameters:** You must specify the model, e.g. `?model=gpt-4o-realtime-preview-2024-10-01` (this is the model ID for the GPT-4 voice model in preview) (OpenAI Realtime API: A Guide With Examples | DataCamp) (Revolutionizing Education with OpenAI’s Realtime API: AI Teachers are getting close-to-natural conversations - Springs). The model name may be updated as new versions are released.  
- **HTTP Headers:**  
  - `Authorization: Bearer YOUR_API_KEY`  
  - `OpenAI-Beta: realtime=v1`  (required for beta access)  

When the client successfully opens the WebSocket connection with these parameters and headers, the server will respond by **creating a new session**. The first message from the server is a JSON event of type `session.created`, indicating the session is ready (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). This event contains a session ID and the default session configuration (such as default voice, language, etc.) that will be used unless overridden.

## Supported Features and Capabilities

The Real-Time Voice API supports a range of features to enable rich voice interactions:

- **Bi-directional audio and text:** You can send either raw audio or text as user input, and you can request the model’s response in audio, text, or both. The API supports *streaming* audio input and output, meaning you don’t have to send the entire audio file at once – you can stream chunks for real-time processing (Introducing the Realtime API | OpenAI). Likewise, the model’s audio reply can start playing before the full response is generated (it streams faster-than-real-time audio) (Realtime API | OpenAI Help Center).

- **Expressive voices:** The model can respond with different voice personas. Initially, OpenAI provides **multiple preset voices** (the same voices available in ChatGPT’s voice mode). For example, voices such as “alloy”, “ash”, “coral”, “echo”, “fable”, “onyx”, “nova”, “sage”, and “shimmer” are supported (OpenAI API Reference). Each voice has a distinct tone and style (some are more emotive, some more calm, etc.). Developers can choose a voice for the AI by configuring the session. The voices are *natural and steerable* – the AI can laugh, whisper, change intonation and emotion based on context or explicit instructions (Realtime API | OpenAI Help Center). (Note: Once a voice is chosen and used in a session, it typically cannot be changed for that session (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn).)

- **Function calling (Tools):** The Real-Time API supports OpenAI’s function calling feature (Introducing the Realtime API | OpenAI) (Realtime API | OpenAI Help Center). This means the assistant can invoke developer-defined functions in response to user requests (e.g. schedule a meeting, fetch weather info, control a device) – enabling interactive voice assistants that perform actions. In an active session, if the model decides a function call is needed, it will output a function call event instead of normal content, which the client can handle and then send the function’s result back into the conversation.

- **Conversation management:** The API keeps track of the conversation context (history) within the session. Each user input (whether text or audio) and each assistant response are added as *conversation items* in the session state. The developer can optionally send a system message or instructions at the start (as part of session configuration) to prime the assistant’s behavior (like a persona or guidelines). The session can be updated on the fly with new instructions or parameters using a `session.update` event if needed (except some fields like voice cannot be changed once set) (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn).

- **Automatic speech recognition:** When you send audio input, the API will transcribe it into text internally. The transcription result is provided as an event (`conversation.item.input_audio_transcription.completed`) containing the recognized text of the user’s utterance (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). This allows you to display or log what the user said. You don’t need to call Whisper separately – the speech recognition is built-in. You can configure aspects like the transcription language or whether to enable interim transcription results via session settings, if needed.

- **Text-to-speech and output modalities:** The assistant’s replies can be received in text form, audio form, or both. By default in a real-time voice session, you’d typically want audio output (the spoken answer) for the user to hear. The API can stream the audio of the assistant’s reply. Simultaneously, it provides the text transcript of the reply (for moderation or UI display) (Realtime API | OpenAI Help Center). This is referred to as *simultaneous multimodal output*. In practice, as the model generates a response, you will receive events like `response.content_part.added` (with chunks of text) and `response.audio.delta` (with chunks of audio data) in parallel (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn) (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). When the response is finished, a `response.done` event is sent, which includes usage tokens and final status.

- **Voice activity detection (VAD) & turn-taking:** The API can handle when to start and stop listening or speaking. By default, **server-side VAD** is enabled, meaning the server will automatically decide when the user has stopped speaking (based on silence) and then proceed to generate the assistant’s response (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). This simplifies turn-taking – you can continuously stream audio from the user, and the server will cut it off at the appropriate moment for the AI to respond. If you prefer manual control, you can disable automatic VAD and manually signal end-of-input (using an `input_audio_buffer.commit` event) when the user stops talking. The server also manages interruptions: if the user starts talking while the assistant is speaking, the client can send an interrupt event (or simply start sending new audio) to stop the assistant’s speech mid-stream (Introducing the Realtime API | OpenAI).

- **Configurable audio formats:** The audio data is typically raw 24 kHz PCM by default, but the API allows specifying input and output audio formats for compatibility. For example, if you are using telephone audio (8 kHz μ-law) or other codecs, you can set `input_audio_format` and `output_audio_format` in the session configuration so the API knows how to interpret and encode audio (AI Voice Assistant with Twilio Voice, OpenAI’s Realtime API, and Python | Twilio). Common formats include `pcm_s16le` (16-bit PCM) and `g711_ulaw` (used in telephony) among others. Audio data exchanged over the WebSocket is encoded as base64 within JSON messages (for text frames) – the client must decode/encode the binary audio accordingly.

- **Scalability and rate limits:** In the beta, the number of simultaneous sessions is limited (e.g. ~100 concurrent sessions for highest tier developers, fewer for lower tiers) as noted by OpenAI (Introducing the Realtime API | OpenAI). Each session can handle a continuous back-and-forth conversation. The pricing during beta is based on both text and audio tokens consumed (Introducing the Realtime API | OpenAI), and is higher for audio (due to the computational cost of speech). Since this is a streaming API, the concept of “requests per minute” is less applicable; instead, limits may be on number of concurrent sessions and total tokens per minute. (Developers should refer to official rate limit documentation as it evolves.)

## Request-Response Format (WebSocket Events)

Interaction with the Real-Time API is event-driven. Both the client and server exchange messages over the WebSocket in a **JSON-based event format**. Each message is a JSON object with a `"type"` field (and additional fields depending on the event type). Below is an outline of the typical message flow and important event types:

- **Session Initialization:** When you connect, the server immediately sends a **`session.created`** event. For example: 

  ```json
  {
    "type": "session.created",
    "session": {
      "session_id": "<uuid>",
      "model": "gpt-4o-realtime-preview-2024-10-01",
      "voice": "echo",
      "input_audio_format": "pcm_s16le",
      "output_audio_format": "pcm_s16le",
      "turn_detection": { "type": "server_vad" },
      "instructions": "",
      ... // other default settings
    }
  }
  ``` 

  This event confirms the connection and provides the default session configuration (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). The client can optionally send a **`session.update`** event to change settings (e.g., select a different voice or change `input_audio_format`) before proceeding. For instance, to switch the assistant’s voice and enable text+audio replies, a client might send:

  ```json
  {
    "type": "session.update",
    "session": {
      "voice": "sage",
      "modalities": ["text", "audio"],
      "instructions": "You are a helpful assistant.",
      "temperature": 0.7
    }
  }
  ``` 

  The server will reply with a **`session.updated`** event echoing the full current config. (Any fields not provided remain as default.)

- **User input (audio):** To send audio from the user’s microphone, the client streams the audio data in chunks. Each chunk is sent as an **`input_audio_buffer.append`** event, containing a base64-encoded audio payload. For example:

  ```json
  {
    "type": "input_audio_buffer.append",
    "audio": "<base64_audio_chunk>"
  }
  ``` 

  The client continues sending `...append` events as the user speaks. If **server VAD** is enabled, the server is monitoring the stream for silence. Once it detects the user stopped, it will automatically finalize the user’s utterance. If VAD is disabled, the client must explicitly signal end of utterance by sending an **`input_audio_buffer.commit`** event:

  ```json
  { "type": "input_audio_buffer.commit" }
  ```

  (In VAD mode, you typically do *not* send a commit, but you can if needed to force an early cut-off.)

- **Transcription result:** After the user’s audio is committed (either by VAD or by commit event), the server transcribes the audio. The result comes as a **`conversation.item.input_audio_transcription.completed`** event, which includes the text transcription. For example:

  ```json
  {
    "type": "conversation.item.input_audio_transcription.completed",
    "item": {
      "id": "<item_id_1>",
      "role": "user",
      "content": { "text": "What's the weather today?" }
    }
  }
  ``` 

  This indicates the user's spoken words (`content.text`) were recognized. The server also typically sends a **`conversation.item.created`** event for the new user message item in the conversation (which may accompany or be part of the above event). The `item_id` can be used to reference this turn in future events.

- **Assistant response generation:** Once the user message is processed, the model starts generating a response. The process goes through a few server events, all streamed to the client in real-time:
  
  - **`response.created`** – indicates the model has begun formulating a response (inference started for a new answer). Contains a `response_id` that ties together the upcoming content.  
  - **`conversation.item.created`** (assistant) – a new conversation item for the assistant’s answer is created (with `role: "assistant"` and an `item_id`). Initially, this item may have no content yet.  
  - **`response.content_part.added`** – as the model generates text, you receive one or more of these events containing partial **text** content for the assistant’s message (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). For instance, you might get: 

    ```json
    { "type": "response.content_part.added",
      "response_id": "<resp_id>",
      "item_id": "<assistant_item_id>",
      "output_index": 0,
      "content": { "text": "Sure, the weather today " }
    }
    ```
    followed by another `content_part.added` with `"text": "is sunny and warm."` etc. Together these build the full text of the assistant’s reply. (The `output_index` might distinguish multiple outputs, e.g. if the model returns both a message and a function call.)

  - **`response.content_part.done`** – marks the end of the text content stream for this part (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). After this, the assistant’s text message is complete.

  - **`response.audio.delta`** – concurrently, if audio output is requested, the server streams chunks of the **audio** for the assistant’s speech (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). Each `audio.delta` event carries a portion of the synthesized speech audio, often as a base64 string (or sometimes binary WebSocket frame) representing a small slice of audio. The client should append these chunks in order and play them (or buffer them for playback). These chunks arrive faster than real-time playback, enabling the client to buffer a bit and play smoothly (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). 

  - **`response.audio.done`** – indicates the assistant’s audio output is fully sent (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). At this point, the client has all audio data needed to play the entire response.

  - **`response.done`** – final event marking the end of the assistant’s response generation. This typically includes a usage summary (token counts for text/audio, possibly whether cached tokens were used) and confirms that the model is ready for the next turn.

- **User input (text):** If instead of audio the user sends a text message, the client can directly send a **`conversation.item.create`** event with the text content. For example:

  ```json
  {
    "type": "conversation.item.create",
    "item": {
      "role": "user",
      "content": { "text": "Hello!" }
    }
  }
  ```
  
  The server will add this to the conversation and proceed to generate a response just as above (skipping the transcription step). In practice, the official reference client uses a simpler alias for this – one can just send a JSON like `{"type": "input_text", "text": "Hello!"}` and the server interprets it similarly (Azure OpenAI Service Realtime API Reference - Azure OpenAI | Microsoft Learn). But the above is the explicit form.

- **Function call results:** If the assistant called a function, the flow involves additional events (e.g., a `conversation.item.created` with `role: "assistant"` and a function call payload, then the client should execute the function and send back a `conversation.item.created` with `role: "function"` containing the result). This would then lead to another `response.created` for the assistant’s answer to the function result. This sequence is analogous to function calls in Chat Completion API, just represented through events. (The specifics can be found in OpenAI’s documentation, but for brevity, not all function call event types are enumerated here.)

- **Error events:** If something goes wrong (e.g., audio chunk too large, unauthorized, model error), the server may send an `error` event with details, and possibly close the connection. The client should handle error events by logging or displaying the error, and perhaps retrying or terminating the session as appropriate.

All events share a similar JSON structure. The `"type"` field distinguishes the event. Most events will also include either a `session`, `item`, or `response` object with relevant data. The conversation item objects contain fields like `id`, `role` (`"user"`, `"assistant"`, or `"function"`), and `content`. Audio content is transmitted in the `audio` fields of certain events as base64 strings, whereas textual content is usually under `content.text`. The design is such that **each new message or result from the model is signaled incrementally**, allowing clients to react in real time (e.g., start playing audio as soon as it arrives, display partial text, etc.).

**Example sequence:** To illustrate, here’s a simplified sequence of events for a single turn, where the user asks a question via audio and the assistant responds with speech:

1. *User speaks “Hello, how are you?” into the microphone.* The client streams audio:
   - `input_audio_buffer.append` (multiple events with audio data)...

2. *User finishes speaking.* Server (with VAD) auto-detects end and transcribes:
   - `conversation.item.input_audio_transcription.completed` – contains `"content": {"text": "Hello, how are you?"}` for the user’s utterance.
   - (Also a `conversation.item.created` for the user message.)

3. *Assistant formulates answer.* Server sends:
   - `response.created` (start generating response)
   - `conversation.item.created` (assistant message placeholder)
   - `response.content_part.added` ... (maybe multiple, with partial text like "I'm doing well, thank you for asking.")
   - `response.content_part.done` (text response complete)
   - `response.audio.delta` ... (multiple events with chunks of audio corresponding to the spoken reply “I’m doing well, thank you for asking.”)
   - `response.audio.done` (all audio for reply sent)
   - `response.done` (marks turn completion, includes usage stats)

4. *Client plays the audio for the user.* The user hears the assistant’s answer.

5. The session stays open, ready for the next user input (back to step 1 for the next turn).

This event-driven protocol might seem complex, but OpenAI provides SDKs/clients to abstract some of it. Essentially, think of it as an ongoing conversation where each side (user and assistant) contributes messages. The Real-Time API just exposes those messages in a streaming JSON form, along with the actual audio bytes for the voice.

## Example Usage of the API (Raw WebSocket)

To directly use the Real-Time Voice API, you can connect via WebSocket and send/receive JSON messages as described. Here’s a brief pseudo-code example (in JavaScript-style pseudocode) demonstrating a basic interaction:

```js
const WebSocket = require('ws');
const API_KEY = "<your_openai_api_key>";
const url = "wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01";

// Open the WebSocket connection
const ws = new WebSocket(url, {
  headers: {
    "Authorization": `Bearer ${API_KEY}`,
    "OpenAI-Beta": "realtime=v1"
  }
});

// When connection opens, configure session (optional)
ws.on('open', () => {
  console.log("Connected to OpenAI Realtime API.");
  // Example: switch voice and request both text+audio output
  const sessionUpdate = {
    type: "session.update",
    session: {
      voice: "alloy",
      modalities: ["text", "audio"]
    }
  };
  ws.send(JSON.stringify(sessionUpdate));
  // Then, for demo, immediately send a text message (instead of audio)
  const userMessage = {
    type: "conversation.item.create",
    item: { role: "user", content: { text: "Tell me a joke." } }
  };
  ws.send(JSON.stringify(userMessage));
});

// Handle incoming events
ws.on('message', (data) => {
  const event = JSON.parse(data);
  console.log("Received event:", event.type);
  if (event.type === 'conversation.item.input_audio_transcription.completed') {
    console.log("User said:", event.item.content.text);
  } else if (event.type === 'response.content_part.added') {
    process.stdout.write(event.content.text); // print partial text
  } else if (event.type === 'response.content_part.done') {
    console.log("\nAssistant response text complete.");
  } else if (event.type === 'response.audio.delta') {
    const audioChunk = Buffer.from(event.audio, 'base64');
    // append to audio buffer or play chunk
  } else if (event.type === 'response.audio.done') {
    console.log("Assistant audio complete. Playing response...");
    // play the accumulated audio buffer to the user
  } else if (event.type === 'response.done') {
    console.log("Assistant finished speaking. Ready for next user input.");
    // potentially prompt user to speak again
  }
});
```

In practice, you’d integrate microphone input (sending `input_audio_buffer.append` events continuously while capturing audio) and speaker output (playing the audio chunks as they arrive). The above example simply sends a text message for brevity and logs the events.

Using the API directly can be involved, so building on top of an SDK or client library (like OpenAI’s reference client, or our Go library below) is recommended. The following section presents a Go library that wraps these details.

# Go Library for OpenAI Real-Time Voice API

## Introduction

We have developed a Go package `openaivoice` that wraps the OpenAI Real-Time Voice API, making it easier to build voice-enabled applications. This library manages the WebSocket connection, authentication, and message exchange with the OpenAI API, so developers can focus on handling audio and responses at a higher level. The goal is to provide an intuitive interface for real-time speech interactions: you can stream microphone audio to the API and receive both transcripts and synthesized voice replies with simple function calls.

**Key features of the Go library:**

- **Connection management:** Handles connecting to the correct WebSocket endpoint with all required headers (API key, beta flags, model parameters). The library abstracts away the low-level WebSocket handling into a simple connect call.

- **Authentication:** You just provide your API key (and optionally a model ID), and the library attaches the proper `Authorization` and `OpenAI-Beta` headers for you.

- **Easy audio/text sending:** Functions are provided to send audio input in real-time. You can feed raw audio bytes (PCM data) into the library, and it will internally chunk and send them as needed. There’s also a method to send a text message if needed.

- **Event handling:** The library continuously listens for events from the server. It assembles the assistant’s response for you – buffering partial text and audio – and exposes high-level events or callbacks. For example, you can register a callback to be notified when a full transcription is available, or when a final response is ready.

- **Voice configuration:** You can choose the assistant’s voice easily via configuration. The library lets you set the desired voice name, as well as other parameters like temperature or language, either on connect or during the session.

- **Output management:** The library can automatically gather the audio chunks of the assistant’s reply and provide you the complete audio data. It also gives you the text of responses. This makes it simple to play the voice response using any audio output mechanism and/or display the text.

- **Documentation & example:** Comes with usage documentation and an example program (see below) to demonstrate a simple voice assistant loop.

The library uses the popular Gorilla WebSocket package internally for robust WebSocket support in Go.

## Implementation

Below is the implementation of the `openaivoice` Go library. It consists of a single Go source file for simplicity. This defines a `Client` struct to manage the connection and provides methods to interact with the Real-Time API. Most of the heavy lifting (JSON formatting, base64, etc.) is handled internally, so the usage is straightforward.

```go
// Package openaivoice provides a client for the OpenAI Real-Time Voice API.
package openaivoice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// EventType constants for relevant events (not exhaustive).
const (
	EventSessionCreated  = "session.created"
	EventSessionUpdated  = "session.updated"
	EventConvCreated     = "conversation.item.created"
	EventTranscription   = "conversation.item.input_audio_transcription.completed"
	EventResponseCreated = "response.created"
	EventContentPart     = "response.content_part.added"
	EventContentDone     = "response.content_part.done"
	EventAudioDelta      = "response.audio.delta"
	EventAudioDone       = "response.audio.done"
	EventResponseDone    = "response.done"
)

// ClientConfig holds optional configuration for a session.
type ClientConfig struct {
	Voice           string  // Desired voice (e.g. "alloy", "sage", etc.)
	Modalities      []string // e.g. []string{"text", "audio"} for both types of output.
	Temperature     *float64 // Sampling temperature for the model (nil for default).
	InputFormat     string  // Input audio format (e.g. "pcm_s16le"); default is 16-bit PCM if empty.
	OutputFormat    string  // Output audio format (e.g. "pcm_s16le"); default if empty.
	Instructions    string  // System instructions or persona for the assistant.
	TurnDetection   string  // "server_vad" (default) or "disabled".
}

// Client manages a realtime voice API session.
type Client struct {
	apiKey   string
	model    string
	conn     *websocket.Conn
	// Channels or callbacks for events could be added here.
	recvChan chan serverEvent // internal channel for receiving events
	// Buffer for assembling the latest response
	currentResponseText  string
	currentResponseAudio []byte
}

// serverEvent is a generic structure to unmarshal incoming events (partial).
type serverEvent struct {
	Type   string          `json:"type"`
	Item   json.RawMessage `json:"item,omitempty"`
	Session json.RawMessage `json:"session,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	Audio  string          `json:"audio,omitempty"` // base64 audio data (for audio.delta events)
	// ... (other fields like response_id, etc., can be added as needed)
}

// NewClient creates a new Client for the given API key and model. 
// If model is empty, a default "gpt-4o-realtime-preview-2024-10-01" will be used.
func NewClient(apiKey string, model string) *Client {
	if model == "" {
		model = "gpt-4o-realtime-preview-2024-10-01"
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		recvChan: make(chan serverEvent, 100),
	}
}

// Connect opens the WebSocket connection to the OpenAI Realtime API and initializes the session.
// Optionally, you can pass a ClientConfig to update session settings (voice, etc.) immediately after connecting.
func (c *Client) Connect(ctx context.Context, config *ClientConfig) error {
	// Prepare the WebSocket dialer and request headers
	dialer := websocket.DefaultDialer
	url := "wss://api.openai.com/v1/realtime?model=" + c.model
	header := http.Header{}
	header.Add("Authorization", "Bearer "+c.apiKey)
	header.Add("OpenAI-Beta", "realtime=v1")

	// Connect to the websocket
	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		return err
	}
	c.conn = conn

	// Start a goroutine to read messages continuously
	go c.listenLoop()

	// Wait for session.created (or a timeout)
	// In a production setting, you'd want to actually parse the first message to ensure session is created.
	// Here, we simply wait a brief moment for session.created to arrive.
	select {
	case <-time.After(2 * time.Second):
		// Proceed even if not received in time; assume connected.
	case evt := <-c.recvChan:
		if evt.Type != EventSessionCreated {
			// If we got something else first (like an error), handle it
			// (For simplicity, we ignore that scenario here.)
		}
	}

	// Send initial session.update if config provided
	if config != nil {
		if err := c.sendSessionUpdate(config); err != nil {
			return err
		}
	}
	return nil
}

// internal helper to send a session.update event based on ClientConfig
func (c *Client) sendSessionUpdate(cfg *ClientConfig) error {
	if c.conn == nil {
		return errors.New("not connected")
	}
	// Build the session.update message
	msg := make(map[string]interface{})
	msg["type"] = "session.update"
	sessionObj := make(map[string]interface{})
	if cfg.Voice != "" {
		sessionObj["voice"] = cfg.Voice
	}
	if len(cfg.Modalities) > 0 {
		sessionObj["modalities"] = cfg.Modalities
	}
	if cfg.Temperature != nil {
		sessionObj["temperature"] = *cfg.Temperature
	}
	if cfg.InputFormat != "" {
		sessionObj["input_audio_format"] = cfg.InputFormat
	}
	if cfg.OutputFormat != "" {
		sessionObj["output_audio_format"] = cfg.OutputFormat
	}
	if cfg.Instructions != "" {
		sessionObj["instructions"] = cfg.Instructions
	}
	if cfg.TurnDetection != "" {
		sessionObj["turn_detection"] = map[string]string{"type": cfg.TurnDetection}
	}
	msg["session"] = sessionObj

	return c.conn.WriteJSON(msg)
}

// SendText sends a text message from the user into the conversation.
func (c *Client) SendText(text string) error {
	if c.conn == nil {
		return errors.New("not connected")
	}
	userMsg := map[string]interface{}{
		"type": "conversation.item.create",
		"item": map[string]interface{}{
			"role": "user",
			"content": map[string]string{
				"text": text,
			},
		},
	}
	return c.conn.WriteJSON(userMsg)
}

// SendAudioChunk streams a chunk of audio input from the user (PCM bytes).
// The audio should match the format specified in the session (e.g. 24kHz 16-bit PCM by default).
func (c *Client) SendAudioChunk(audioData []byte) error {
	if c.conn == nil {
		return errors.New("not connected")
	}
	encoded := base64.StdEncoding.EncodeToString(audioData)
	chunkMsg := map[string]interface{}{
		"type":  "input_audio_buffer.append",
		"audio": encoded,
	}
	return c.conn.WriteJSON(chunkMsg)
}

// CommitAudio signals that the user has finished speaking (for manual turn detection).
// This sends an input_audio_buffer.commit event. Not needed if server VAD is on.
func (c *Client) CommitAudio() error {
	if c.conn == nil {
		return errors.New("not connected")
	}
	commitMsg := map[string]string{
		"type": "input_audio_buffer.commit",
	}
	return c.conn.WriteJSON(commitMsg)
}

// Close ends the session and closes the WebSocket connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Listen for next completed assistant response (blocking).
// This function waits until an entire assistant reply is received (text and audio).
// It returns the text of the response and the raw audio bytes.
func (c *Client) ListenOnce(ctx context.Context) (text string, audio []byte, err error) {
	// Reset current response buffers
	c.currentResponseText = ""
	c.currentResponseAudio = nil

	// Loop, reading from the internal recvChan, until response.done or context done
	for {
		select {
		case evt := <-c.recvChan:
			switch evt.Type {
			case EventTranscription:
				// Transcription of user input completed (we could surface this if needed)
			case EventContentPart:
				// Append partial text
				var content struct {
					Text string `json:"text"`
				}
				json.Unmarshal(evt.Content, &content)
				c.currentResponseText += content.Text
			case EventContentDone:
				// Assistant text done (we have full text in currentResponseText)
			case EventAudioDelta:
				// Append audio chunk to buffer
				if evt.Audio != "" {
					chunk, _ := base64.StdEncoding.DecodeString(evt.Audio)
					c.currentResponseAudio = append(c.currentResponseAudio, chunk...)
				}
			case EventAudioDone:
				// Finished receiving audio (currentResponseAudio has full data)
			case EventResponseDone:
				// Completed one response
				return c.currentResponseText, c.currentResponseAudio, nil
			default:
				// Other events (session.created, etc.) can be handled as needed
			}
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}
}

// listenLoop continuously reads messages from the WebSocket and pushes them into recvChan.
func (c *Client) listenLoop() {
	for {
		if c.conn == nil {
			break
		}
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			// If connection closed or error, exit loop
			close(c.recvChan)
			return
		}
		var evt serverEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			// skip or log malformed event
			continue
		}
		// Push event to channel for processing
		c.recvChan <- evt
	}
}
```

**Explanation:** This code establishes a WebSocket connection in `Connect()`, then spawns a `listenLoop` goroutine to continuously read incoming messages. Incoming JSON events are parsed into a generic `serverEvent` structure and sent into a channel. The client can either consume these events via a provided method (like `ListenOnce`) or we could extend the library to support callback registration (for example, a callback for when a transcription arrives, or when a full answer is ready). For simplicity, `ListenOnce` demonstrates a synchronous way to wait for one complete response from the assistant.

The library provides `SendAudioChunk` for streaming audio input. In a real scenario, you’d call this in a loop as you capture audio from the microphone (perhaps in a separate goroutine), and then call `CommitAudio` when done, unless you rely on the server’s VAD (default) to auto-detect end of speech. The `SendText` method allows injecting a text query directly.

We also have a simple `ClientConfig` struct to pass initial settings (voice, etc.) to the session. The code sends a `session.update` with those settings right after the connection if provided. This is how you select a different voice or change the input/output format to match your audio source. For example, if you wanted to use the “shimmer” voice, you’d set `Voice: "shimmer"` in `ClientConfig`.

## Basic Usage

To use this library, you would import `openaivoice` in your Go application. The typical usage pattern is:

1. **Initialize the client:** Provide your API key (and optionally a model). For example: `client := openaivoice.NewClient("<API_KEY>", "")`. If you leave model blank, it defaults to the current GPT-4 voice model.

2. **Connect:** Establish the WebSocket connection. This is done with a context (for timeout/cancel support) and an optional config. For example:

   ```go
   cfg := &openaivoice.ClientConfig{ Voice: "echo", Modalities: []string{"text", "audio"} }
   err := client.Connect(context.Background(), cfg)
   if err != nil {
       log.Fatal("Failed to connect:", err)
   }
   ```
   This will open the connection and set the voice to "echo" for responses, asking for both text and audio.

3. **Send audio or text:** Now you can send user input. For voice input, you might capture microphone audio frames (e.g., 24kHz PCM bytes) and call `client.SendAudioChunk(data)` repeatedly. If you prefer to test with a pre-recorded audio file, you could read the file bytes and send them in chunks. Alternatively, use `client.SendText("Hello")` to send a text query.

4. **Receive the response:** You can use `client.ListenOnce()` to wait for one complete response from the assistant. This will block until the assistant has finished speaking. It returns the full text and audio of the reply. You can then play the audio (using a speaker output library) and/or display the text. For continuous interactive conversation, you might call `ListenOnce()` in a loop or handle events as they come.

5. **Close when done:** Call `client.Close()` to cleanly close the WebSocket when your program is finishing or you are done with the session.

## Example Application

To demonstrate, here’s a simple Go `main` program that uses the `openaivoice` library. This example reads an audio file (`input.wav`) containing a user’s question, sends it to the API, then saves the assistant’s spoken answer to `output.wav`. (In a real scenario, instead of reading from a file, you would capture live audio from a microphone, and instead of saving to a file, you might play the audio through speakers.)

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"your_module_path/openaivoice"
)

func main() {
	// 1. Create a new OpenAI voice client with API key
	apiKey := "<YOUR_OPENAI_API_KEY>"
	client := openaivoice.NewClient(apiKey, "")  // default model

	// 2. Connect to the realtime API with desired settings
	cfg := &openaivoice.ClientConfig{
		Voice:        "sage",                       // choose voice for assistant
		Modalities:   []string{"text", "audio"},    // want both text and audio response
		Instructions: "You are a helpful assistant.", // set a system prompt
	}
	if err := client.Connect(context.Background(), cfg); err != nil {
		log.Fatalf("Failed to connect to OpenAI Realtime API: %v", err)
	}
	fmt.Println("Connected to OpenAI realtime voice API.")

	// 3. Load an example audio file to send as user input (WAV/PCM)
	userAudio, err := ioutil.ReadFile("input.wav")
	if err != nil {
		log.Fatal("Could not read input.wav:", err)
	}
	// In a real app, you'd stream microphone data via SendAudioChunk in small pieces.
	// Here, we'll simulate streaming by sending the file in chunks of 3200 bytes (~0.1s of 16k audio).
	chunkSize := 3200
	for i := 0; i < len(userAudio); i += chunkSize {
		end := i + chunkSize
		if end > len(userAudio) {
			end = len(userAudio)
		}
		chunk := userAudio[i:end]
		if err := client.SendAudioChunk(chunk); err != nil {
			log.Fatal("Error sending audio chunk:", err)
		}
	}
	// Signal end of user speech (not strictly needed if using server VAD, but included for completeness)
	if err := client.CommitAudio(); err != nil {
		log.Println("Commit audio error (possibly not needed):", err)
	}

	// 4. Wait for the assistant's response (text and audio)
	responseText, responseAudio, err := client.ListenOnce(context.Background())
	if err != nil {
		log.Fatal("Error while waiting for response:", err)
	}
	fmt.Println("Assistant response (transcribed):", responseText)

	// 5. Save the audio response to file (as an example of handling the audio output)
	if err := ioutil.WriteFile("output.wav", responseAudio, 0644); err != nil {
		log.Fatal("Failed to save output.wav:", err)
	}
	fmt.Println("Assistant audio response saved to output.wav")

	// Close the session
	client.Close()
}
```

**What this example does:** It connects to the API, configures the assistant’s voice and modality, reads an audio file (`input.wav` should be a short question or phrase spoken by a user), splits it into chunks to simulate streaming, and sends those chunks via the `SendAudioChunk` method. We then call `ListenOnce` to block until the assistant has fully responded. We print out the assistant’s response text to the console and write the received audio to `output.wav`. You could then listen to `output.wav` to hear the assistant’s voice answer. In a live scenario, instead of writing to a file, you’d pass `responseAudio` bytes to an audio playback library to play sound to the user.

This example is kept simple for illustration. In a real interactive voice assistant, you would likely run the sending and receiving in parallel: continuously send microphone audio on one goroutine and continuously listen for events on another, allowing for back-and-forth conversation without distinct start-stop of the entire program for each turn.

## Future Optimizations and Enhancements

While the provided Go library covers the basics, there are several areas for future optimization and features:

- **Improved concurrency and callbacks:** The current implementation uses a simple channel and a blocking `ListenOnce` method. In the future, we could allow registering event callbacks (e.g., on transcription, on partial response, on full response) for a more event-driven integration. This would let an application start acting on partial transcripts or start playing audio before the full response is ready, reducing perceived latency.

- **Audio processing optimizations:** We can add support for audio format conversions internally. For example, if the microphone provides 48kHz audio, the library could downsample to 24kHz PCM on the fly to meet the API’s expected format. Similarly, supporting compressed audio formats (if the API ever allows Opus or other codecs) could drastically reduce bandwidth. Currently, audio is base64-encoded (which adds ~33% overhead); if the API/SDK allows binary frames, switching to binary WebSocket frames for audio could be more efficient. These optimizations could improve real-time performance, especially on mobile or low-bandwidth scenarios.

- **Dynamic VAD control:** The library could incorporate a voice activity detection on the client side for finer control when `turn_detection` is disabled. For instance, automatically calling `CommitAudio()` after a certain period of silence in the input stream. This would help when server VAD is turned off (perhaps to handle barge-in differently). Integrating an actual VAD algorithm on the client side could be an enhancement.

- **Session management and scaling:** In future iterations, the library could manage multiple parallel sessions (for scaling to many concurrent calls or users). It could also expose the token usage or cost info from `response.done` events to help developers monitor usage. Additionally, implementing automatic reconnection logic in case of transient network issues would make the library more robust for long-lived sessions.

- **Integration with audio I/O libraries:** To make a fully comprehensive solution, we might integrate with a library (or provide examples) for capturing microphone input and playing speaker output in real-time. While that’s outside the core scope of the API wrapper, providing utility functions or examples that tie into something like PortAudio or Oto could help developers get started faster.

- **Prompt caching and cost optimizations:** As OpenAI introduces **prompt caching** for the Realtime API (New Realtime API voices and cache pricing - Announcements - OpenAI Developer Community), the library could help take advantage of that by optionally storing recent conversation context and reusing it with cache tokens, reducing cost. Exposing controls for using the caching feature or automatically managing the cache might be possible.

By addressing these areas, the Go library can become more efficient and developer-friendly. As the OpenAI Real-Time Voice API evolves (e.g., adding more modalities like vision or releasing new model versions (Introducing the Realtime API | OpenAI)), the library can be updated to support those with minimal changes required from the application code. For now, this library provides a solid foundation to start building real-time voice AI applications in Go, abstracting away the intricacies of the WebSocket communication and allowing developers to focus on the creative aspects of their voice assistant or voice-enabled product.

