/**
 * Azure OpenAI Service Realtime API TypeScript Definitions
 * Based on Azure OpenAI Service Realtime API Reference
 * API Version: 2024-12-17
 */

// ----------------------------------------------------------------------------
// Enums
// ----------------------------------------------------------------------------

/**
 * Audio format for input and output
 */
export enum RealtimeAudioFormat {
  PCM16 = "pcm16",
  G711_ULAW = "g711_ulaw",
  G711_ALAW = "g711_alaw"
}

/**
 * Model for audio input transcription
 */
export enum RealtimeAudioInputTranscriptionModel {
  WHISPER_1 = "whisper-1"
}

/**
 * Type of client events that can be sent to the server
 */
export enum RealtimeClientEventType {
  SESSION_UPDATE = "session.update",
  INPUT_AUDIO_BUFFER_APPEND = "input_audio_buffer.append",
  INPUT_AUDIO_BUFFER_COMMIT = "input_audio_buffer.commit",
  INPUT_AUDIO_BUFFER_CLEAR = "input_audio_buffer.clear",
  CONVERSATION_ITEM_CREATE = "conversation.item.create",
  CONVERSATION_ITEM_DELETE = "conversation.item.delete",
  CONVERSATION_ITEM_TRUNCATE = "conversation.item.truncate",
  RESPONSE_CREATE = "response.create",
  RESPONSE_CANCEL = "response.cancel"
}

/**
 * Types of content parts within messages
 */
export enum RealtimeContentPartType {
  INPUT_TEXT = "input_text",
  INPUT_AUDIO = "input_audio",
  TEXT = "text",
  AUDIO = "audio",
  ITEM_REFERENCE = "item_reference"
}

/**
 * Status of an item in the conversation
 */
export enum RealtimeItemStatus {
  IN_PROGRESS = "in_progress",
  COMPLETED = "completed",
  INCOMPLETE = "incomplete"
}

/**
 * Types of items in the conversation
 */
export enum RealtimeItemType {
  MESSAGE = "message",
  FUNCTION_CALL = "function_call",
  FUNCTION_CALL_OUTPUT = "function_call_output"
}

/**
 * Roles for message items
 */
export enum RealtimeMessageRole {
  SYSTEM = "system",
  USER = "user",
  ASSISTANT = "assistant"
}

/**
 * Status of a response
 */
export enum RealtimeResponseStatus {
  IN_PROGRESS = "in_progress",
  COMPLETED = "completed",
  CANCELLED = "cancelled",
  INCOMPLETE = "incomplete",
  FAILED = "failed"
}

/**
 * Types of server events that can be received from the server
 */
export enum RealtimeServerEventType {
  SESSION_CREATED = "session.created",
  SESSION_UPDATED = "session.updated",
  CONVERSATION_CREATED = "conversation.created",
  CONVERSATION_ITEM_CREATED = "conversation.item.created",
  CONVERSATION_ITEM_DELETED = "conversation.item.deleted",
  CONVERSATION_ITEM_TRUNCATED = "conversation.item.truncated",
  RESPONSE_CREATED = "response.created",
  RESPONSE_DONE = "response.done",
  RATE_LIMITS_UPDATED = "rate_limits.updated",
  RESPONSE_OUTPUT_ITEM_ADDED = "response.output_item.added",
  RESPONSE_OUTPUT_ITEM_DONE = "response.output_item.done",
  RESPONSE_CONTENT_PART_ADDED = "response.content_part.added",
  RESPONSE_CONTENT_PART_DONE = "response.content_part.done",
  RESPONSE_AUDIO_DELTA = "response.audio.delta",
  RESPONSE_AUDIO_DONE = "response.audio.done",
  RESPONSE_AUDIO_TRANSCRIPT_DELTA = "response.audio_transcript.delta",
  RESPONSE_AUDIO_TRANSCRIPT_DONE = "response.audio_transcript.done",
  RESPONSE_TEXT_DELTA = "response.text.delta",
  RESPONSE_TEXT_DONE = "response.text.done",
  RESPONSE_FUNCTION_CALL_ARGUMENTS_DELTA = "response.function_call_arguments.delta",
  RESPONSE_FUNCTION_CALL_ARGUMENTS_DONE = "response.function_call_arguments.done",
  INPUT_AUDIO_BUFFER_SPEECH_STARTED = "input_audio_buffer.speech_started",
  INPUT_AUDIO_BUFFER_SPEECH_STOPPED = "input_audio_buffer.speech_stopped",
  CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_COMPLETED = "conversation.item.input_audio_transcription.completed",
  CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_FAILED = "conversation.item.input_audio_transcription.failed",
  INPUT_AUDIO_BUFFER_COMMITTED = "input_audio_buffer.committed",
  INPUT_AUDIO_BUFFER_CLEARED = "input_audio_buffer.cleared",
  ERROR = "error"
}

/**
 * Type of tool
 */
export enum RealtimeToolType {
  FUNCTION = "function"
}

/**
 * Available tool choice options
 */
export enum RealtimeToolChoiceLiteral {
  AUTO = "auto",
  NONE = "none",
  REQUIRED = "required"
}

/**
 * Type of turn detection
 */
export enum RealtimeTurnDetectionType {
  SERVER_VAD = "server_vad"
}

/**
 * Voice options for audio responses
 */
export enum RealtimeVoice {
  ALLOY = "alloy",
  ASH = "ash",
  BALLAD = "ballad",
  CORAL = "coral",
  ECHO = "echo",
  SAGE = "sage",
  SHIMMER = "shimmer",
  VERSE = "verse"
}

// ----------------------------------------------------------------------------
// Base Interfaces
// ----------------------------------------------------------------------------

/**
 * Base interface for client events
 */
export interface RealtimeClientEventBase {
  type: RealtimeClientEventType;
  event_id?: string;
}

/**
 * Base interface for server events
 */
export interface RealtimeServerEventBase {
  type: RealtimeServerEventType;
  event_id?: string;
}

/**
 * Error object structure
 */
export interface RealtimeError {
  type?: string;
  code?: string;
  message: string;
  param?: string;
  event_id?: string;
}

// ----------------------------------------------------------------------------
// Content Part Interfaces
// ----------------------------------------------------------------------------

/**
 * Base interface for content parts
 */
export interface RealtimeContentPartBase {
  type: RealtimeContentPartType;
}

/**
 * Text content part
 */
export interface RealtimeTextContentPart extends RealtimeContentPartBase {
  type: RealtimeContentPartType.TEXT;
  text: string;
}

/**
 * Input text content part
 */
export interface RealtimeInputTextContentPart extends RealtimeContentPartBase {
  type: RealtimeContentPartType.INPUT_TEXT;
  text: string;
}

/**
 * Input audio content part
 */
export interface RealtimeInputAudioContentPart extends RealtimeContentPartBase {
  type: RealtimeContentPartType.INPUT_AUDIO;
  audio?: string;
  transcript?: string;
}

/**
 * Audio content part
 */
export interface RealtimeAudioContentPart extends RealtimeContentPartBase {
  type: RealtimeContentPartType.AUDIO;
  transcript?: string;
}

/**
 * Item reference content part
 */
export interface RealtimeItemReferenceContentPart extends RealtimeContentPartBase {
  type: RealtimeContentPartType.ITEM_REFERENCE;
  id: string;
}

/**
 * Union of all content part types
 */
export type RealtimeContentPart =
  | RealtimeTextContentPart
  | RealtimeInputTextContentPart
  | RealtimeInputAudioContentPart
  | RealtimeAudioContentPart
  | RealtimeItemReferenceContentPart;

// ----------------------------------------------------------------------------
// Tool Interfaces
// ----------------------------------------------------------------------------

/**
 * Base interface for tools
 */
export interface RealtimeToolBase {
  type: RealtimeToolType;
}

/**
 * Function tool
 */
export interface RealtimeFunctionTool extends RealtimeToolBase {
  type: RealtimeToolType.FUNCTION;
  name: string;
  description?: string;
  parameters: Record<string, any>;
}

/**
 * Union of all tool types
 */
export type RealtimeTool = RealtimeFunctionTool;

/**
 * Base interface for tool choice
 */
export interface RealtimeToolChoiceObject {
  type: RealtimeToolType;
}

/**
 * Function tool choice
 */
export interface RealtimeToolChoiceFunctionObject extends RealtimeToolChoiceObject {
  type: RealtimeToolType.FUNCTION;
  function: {
    name: string;
  };
}

/**
 * Union of all tool choice types
 */
export type RealtimeToolChoice = RealtimeToolChoiceLiteral | RealtimeToolChoiceFunctionObject;

// ----------------------------------------------------------------------------
// Audio Interfaces
// ----------------------------------------------------------------------------

/**
 * Audio input transcription settings
 */
export interface RealtimeAudioInputTranscriptionSettings {
  model: RealtimeAudioInputTranscriptionModel;
}

// ----------------------------------------------------------------------------
// Turn Detection Interfaces
// ----------------------------------------------------------------------------

/**
 * Base turn detection interface
 */
export interface RealtimeTurnDetectionBase {
  type: RealtimeTurnDetectionType;
}

/**
 * Server VAD turn detection
 */
export interface RealtimeServerVadTurnDetection extends RealtimeTurnDetectionBase {
  type: RealtimeTurnDetectionType.SERVER_VAD;
  threshold?: number;
  prefix_padding_ms?: number;
  silence_duration_ms?: number;
  create_response?: boolean;
}

/**
 * Union of all turn detection types
 */
export type RealtimeTurnDetection = RealtimeServerVadTurnDetection;

// ----------------------------------------------------------------------------
// Conversation Item Interfaces
// ----------------------------------------------------------------------------

/**
 * Base interface for conversation items
 */
export interface RealtimeConversationItemBase {
  id?: string;
  type: RealtimeItemType;
  object?: string;
  status?: RealtimeItemStatus;
}

/**
 * Message item
 */
export interface RealtimeMessageItem extends RealtimeConversationItemBase {
  type: RealtimeItemType.MESSAGE;
  role: RealtimeMessageRole;
  content?: RealtimeContentPart[];
}

/**
 * Function call item
 */
export interface RealtimeFunctionCallItem extends RealtimeConversationItemBase {
  type: RealtimeItemType.FUNCTION_CALL;
  call_id: string;
  name: string;
  arguments: string;
}

/**
 * Function call output item
 */
export interface RealtimeFunctionCallOutputItem extends RealtimeConversationItemBase {
  type: RealtimeItemType.FUNCTION_CALL_OUTPUT;
  call_id: string;
  output: string;
}

/**
 * Union of all conversation item types
 */
export type RealtimeConversationItem =
  | RealtimeMessageItem
  | RealtimeFunctionCallItem
  | RealtimeFunctionCallOutputItem;

/**
 * Interface for conversation item in request
 */
export interface RealtimeConversationRequestItem extends RealtimeConversationItemBase {
  id?: string;
  type: RealtimeItemType;
}

/**
 * Interface for system message item in request
 */
export interface RealtimeRequestSystemMessageItem {
  role: RealtimeMessageRole.SYSTEM;
  content: RealtimeInputTextContentPart[];
}

/**
 * Interface for user message item in request
 */
export interface RealtimeRequestUserMessageItem {
  role: RealtimeMessageRole.USER;
  content: (RealtimeInputTextContentPart | RealtimeInputAudioContentPart)[];
}

/**
 * Interface for assistant message item in request
 */
export interface RealtimeRequestAssistantMessageItem {
  role: RealtimeMessageRole.ASSISTANT;
  content: RealtimeInputTextContentPart[];
}

/**
 * Interface for message reference item in request
 */
export interface RealtimeRequestMessageReferenceItem {
  type: RealtimeItemType.MESSAGE;
  id: string;
}

/**
 * Interface for function call item in request
 */
export interface RealtimeRequestFunctionCallItem {
  type: RealtimeItemType.FUNCTION_CALL;
  name: string;
  call_id: string;
  arguments: string;
  status?: RealtimeItemStatus;
}

/**
 * Interface for function call output item in request
 */
export interface RealtimeRequestFunctionCallOutputItem {
  type: RealtimeItemType.FUNCTION_CALL_OUTPUT;
  call_id: string;
  output: string;
}

/**
 * Interface for conversation item in response
 */
export interface RealtimeConversationResponseItem extends RealtimeConversationItemBase {
  object: string;
  type: RealtimeItemType;
  id: string;
}

// ----------------------------------------------------------------------------
// Session Interfaces
// ----------------------------------------------------------------------------

/**
 * Base session interface
 */
export interface RealtimeSessionBase {
  modalities?: string[];
  instructions?: string;
  voice?: RealtimeVoice;
  input_audio_format?: RealtimeAudioFormat;
  output_audio_format?: RealtimeAudioFormat;
  input_audio_transcription?: RealtimeAudioInputTranscriptionSettings;
  turn_detection?: RealtimeTurnDetection;
  tools?: RealtimeTool[];
  tool_choice?: RealtimeToolChoice;
  temperature?: number;
  max_response_output_tokens?: number | "inf";
}

/**
 * Request session interface
 */
export interface RealtimeRequestSession extends RealtimeSessionBase {}

/**
 * Response session interface
 */
export interface RealtimeResponseSession extends RealtimeSessionBase {
  object: string;
  id: string;
  model: string;
}

// ----------------------------------------------------------------------------
// Response Interfaces
// ----------------------------------------------------------------------------

/**
 * Base response interface
 */
export interface RealtimeResponseBase {
  object?: string;
  id?: string;
  status?: RealtimeResponseStatus;
}

/**
 * Response status details
 */
export interface RealtimeResponseStatusDetails {
  type: RealtimeResponseStatus;
}

/**
 * Token usage details
 */
export interface RealtimeResponseUsage {
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  input_token_details: {
    cached_tokens: number;
    text_tokens: number;
    audio_tokens: number;
  };
  output_token_details: {
    text_tokens: number;
    audio_tokens: number;
  };
}

/**
 * Response interface
 */
export interface RealtimeResponse extends RealtimeResponseBase {
  object: string;
  id: string;
  status: RealtimeResponseStatus;
  status_details?: RealtimeResponseStatusDetails;
  output: RealtimeConversationResponseItem[];
  usage: RealtimeResponseUsage;
}

/**
 * Response options interface
 */
export interface RealtimeResponseOptions {
  modalities?: string[];
  instructions?: string;
  voice?: RealtimeVoice;
  output_audio_format?: RealtimeAudioFormat;
  tools?: RealtimeTool[];
  tool_choice?: RealtimeToolChoice;
  temperature?: number;
  max_response_output_tokens?: number | "inf";
  conversation?: "auto" | "none";
  metadata?: Record<string, string>;
  input?: RealtimeConversationItemBase[];
}

// ----------------------------------------------------------------------------
// Client Event Interfaces
// ----------------------------------------------------------------------------

/**
 * Session update client event
 */
export interface RealtimeClientEventSessionUpdate extends RealtimeClientEventBase {
  type: RealtimeClientEventType.SESSION_UPDATE;
  session: RealtimeRequestSession;
}

/**
 * Input audio buffer append client event
 */
export interface RealtimeClientEventInputAudioBufferAppend extends RealtimeClientEventBase {
  type: RealtimeClientEventType.INPUT_AUDIO_BUFFER_APPEND;
  audio: string;
}

/**
 * Input audio buffer commit client event
 */
export interface RealtimeClientEventInputAudioBufferCommit extends RealtimeClientEventBase {
  type: RealtimeClientEventType.INPUT_AUDIO_BUFFER_COMMIT;
}

/**
 * Input audio buffer clear client event
 */
export interface RealtimeClientEventInputAudioBufferClear extends RealtimeClientEventBase {
  type: RealtimeClientEventType.INPUT_AUDIO_BUFFER_CLEAR;
}

/**
 * Conversation item create client event
 */
export interface RealtimeClientEventConversationItemCreate extends RealtimeClientEventBase {
  type: RealtimeClientEventType.CONVERSATION_ITEM_CREATE;
  previous_item_id?: string;
  item: RealtimeConversationRequestItem;
}

/**
 * Conversation item delete client event
 */
export interface RealtimeClientEventConversationItemDelete extends RealtimeClientEventBase {
  type: RealtimeClientEventType.CONVERSATION_ITEM_DELETE;
  item_id: string;
}

/**
 * Conversation item truncate client event
 */
export interface RealtimeClientEventConversationItemTruncate extends RealtimeClientEventBase {
  type: RealtimeClientEventType.CONVERSATION_ITEM_TRUNCATE;
  item_id: string;
  content_index: number;
  audio_end_ms: number;
}

/**
 * Response create client event
 */
export interface RealtimeClientEventResponseCreate extends RealtimeClientEventBase {
  type: RealtimeClientEventType.RESPONSE_CREATE;
  response?: RealtimeResponseOptions;
}

/**
 * Response cancel client event
 */
export interface RealtimeClientEventResponseCancel extends RealtimeClientEventBase {
  type: RealtimeClientEventType.RESPONSE_CANCEL;
}

/**
 * Union of all client event types
 */
export type RealtimeClientEvent =
  | RealtimeClientEventSessionUpdate
  | RealtimeClientEventInputAudioBufferAppend
  | RealtimeClientEventInputAudioBufferCommit
  | RealtimeClientEventInputAudioBufferClear
  | RealtimeClientEventConversationItemCreate
  | RealtimeClientEventConversationItemDelete
  | RealtimeClientEventConversationItemTruncate
  | RealtimeClientEventResponseCreate
  | RealtimeClientEventResponseCancel;

// ----------------------------------------------------------------------------
// Server Event Interfaces
// ----------------------------------------------------------------------------

/**
 * Session created server event
 */
export interface RealtimeServerEventSessionCreated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.SESSION_CREATED;
  session: RealtimeResponseSession;
}

/**
 * Session updated server event
 */
export interface RealtimeServerEventSessionUpdated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.SESSION_UPDATED;
  session: RealtimeResponseSession;
}

/**
 * Conversation created server event
 */
export interface RealtimeServerEventConversationCreated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_CREATED;
  conversation: {
    id: string;
    object: string;
  };
}

/**
 * Conversation item created server event
 */
export interface RealtimeServerEventConversationItemCreated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_ITEM_CREATED;
  previous_item_id?: string;
  item: RealtimeConversationResponseItem;
}

/**
 * Conversation item deleted server event
 */
export interface RealtimeServerEventConversationItemDeleted extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_ITEM_DELETED;
  item_id: string;
}

/**
 * Conversation item truncated server event
 */
export interface RealtimeServerEventConversationItemTruncated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_ITEM_TRUNCATED;
  item_id: string;
  content_index: number;
  audio_end_ms: number;
}

/**
 * Input audio buffer cleared server event
 */
export interface RealtimeServerEventInputAudioBufferCleared extends RealtimeServerEventBase {
  type: RealtimeServerEventType.INPUT_AUDIO_BUFFER_CLEARED;
}

/**
 * Input audio buffer committed server event
 */
export interface RealtimeServerEventInputAudioBufferCommitted extends RealtimeServerEventBase {
  type: RealtimeServerEventType.INPUT_AUDIO_BUFFER_COMMITTED;
  previous_item_id?: string;
  item_id: string;
}

/**
 * Input audio buffer speech started server event
 */
export interface RealtimeServerEventInputAudioBufferSpeechStarted extends RealtimeServerEventBase {
  type: RealtimeServerEventType.INPUT_AUDIO_BUFFER_SPEECH_STARTED;
  audio_start_ms: number;
  item_id: string;
}

/**
 * Input audio buffer speech stopped server event
 */
export interface RealtimeServerEventInputAudioBufferSpeechStopped extends RealtimeServerEventBase {
  type: RealtimeServerEventType.INPUT_AUDIO_BUFFER_SPEECH_STOPPED;
  audio_end_ms: number;
  item_id: string;
}

/**
 * Conversation item input audio transcription completed server event
 */
export interface RealtimeServerEventConversationItemInputAudioTranscriptionCompleted extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_COMPLETED;
  item_id: string;
  content_index: number;
  transcript: string;
}

/**
 * Conversation item input audio transcription failed server event
 */
export interface RealtimeServerEventConversationItemInputAudioTranscriptionFailed extends RealtimeServerEventBase {
  type: RealtimeServerEventType.CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_FAILED;
  item_id: string;
  content_index: number;
  error: RealtimeError;
}

/**
 * Response created server event
 */
export interface RealtimeServerEventResponseCreated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_CREATED;
  response: RealtimeResponse;
}

/**
 * Response done server event
 */
export interface RealtimeServerEventResponseDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_DONE;
  response: RealtimeResponse;
}

/**
 * Rate limits updated server event
 */
export interface RealtimeServerEventRateLimitsUpdated extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RATE_LIMITS_UPDATED;
  rate_limits: {
    name: string;
    limit: number;
    remaining: number;
    reset_seconds: number;
  }[];
}

/**
 * Response output item added server event
 */
export interface RealtimeServerEventResponseOutputItemAdded extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_OUTPUT_ITEM_ADDED;
  response_id: string;
  output_index: number;
  item: RealtimeConversationResponseItem;
}

/**
 * Response output item done server event
 */
export interface RealtimeServerEventResponseOutputItemDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_OUTPUT_ITEM_DONE;
  response_id: string;
  output_index: number;
  item: RealtimeConversationResponseItem;
}

/**
 * Response content part added server event
 */
export interface RealtimeServerEventResponseContentPartAdded extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_CONTENT_PART_ADDED;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  part: RealtimeContentPart;
}

/**
 * Response content part done server event
 */
export interface RealtimeServerEventResponseContentPartDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_CONTENT_PART_DONE;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  part: RealtimeContentPart;
}

/**
 * Response audio delta server event
 */
export interface RealtimeServerEventResponseAudioDelta extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_AUDIO_DELTA;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  delta: string;
}

/**
 * Response audio done server event
 */
export interface RealtimeServerEventResponseAudioDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_AUDIO_DONE;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
}

/**
 * Response audio transcript delta server event
 */
export interface RealtimeServerEventResponseAudioTranscriptDelta extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_AUDIO_TRANSCRIPT_DELTA;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  delta: string;
}

/**
 * Response audio transcript done server event
 */
export interface RealtimeServerEventResponseAudioTranscriptDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_AUDIO_TRANSCRIPT_DONE;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  transcript: string;
}

/**
 * Response text delta server event
 */
export interface RealtimeServerEventResponseTextDelta extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_TEXT_DELTA;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  delta: string;
}

/**
 * Response text done server event
 */
export interface RealtimeServerEventResponseTextDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_TEXT_DONE;
  response_id: string;
  item_id: string;
  output_index: number;
  content_index: number;
  text: string;
}

/**
 * Response function call arguments delta server event
 */
export interface RealtimeServerEventResponseFunctionCallArgumentsDelta extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_FUNCTION_CALL_ARGUMENTS_DELTA;
  response_id: string;
  item_id: string;
  output_index: number;
  call_id: string;
  delta: string;
}

/**
 * Response function call arguments done server event
 */
export interface RealtimeServerEventResponseFunctionCallArgumentsDone extends RealtimeServerEventBase {
  type: RealtimeServerEventType.RESPONSE_FUNCTION_CALL_ARGUMENTS_DONE;
  response_id: string;
  item_id: string;
  output_index: number;
  call_id: string;
  arguments: string;
}

/**
 * Error server event
 */
export interface RealtimeServerEventError extends RealtimeServerEventBase {
  type: RealtimeServerEventType.ERROR;
  error: RealtimeError;
}

/**
 * Union of all server event types
 */
export type RealtimeServerEvent =
  | RealtimeServerEventSessionCreated
  | RealtimeServerEventSessionUpdated
  | RealtimeServerEventConversationCreated
  | RealtimeServerEventConversationItemCreated
  | RealtimeServerEventConversationItemDeleted
  | RealtimeServerEventConversationItemTruncated
  | RealtimeServerEventInputAudioBufferCleared
  | RealtimeServerEventInputAudioBufferCommitted
  | RealtimeServerEventInputAudioBufferSpeechStarted
  | RealtimeServerEventInputAudioBufferSpeechStopped
  | RealtimeServerEventConversationItemInputAudioTranscriptionCompleted
  | RealtimeServerEventConversationItemInputAudioTranscriptionFailed
  | RealtimeServerEventResponseCreated
  | RealtimeServerEventResponseDone
  | RealtimeServerEventRateLimitsUpdated
  | RealtimeServerEventResponseOutputItemAdded
  | RealtimeServerEventResponseOutputItemDone
  | RealtimeServerEventResponseContentPartAdded
  | RealtimeServerEventResponseContentPartDone
  | RealtimeServerEventResponseAudioDelta
  | RealtimeServerEventResponseAudioDone
  | RealtimeServerEventResponseAudioTranscriptDelta
  | RealtimeServerEventResponseAudioTranscriptDone
  | RealtimeServerEventResponseTextDelta
  | RealtimeServerEventResponseTextDone
  | RealtimeServerEventResponseFunctionCallArgumentsDelta
  | RealtimeServerEventResponseFunctionCallArgumentsDone
  | RealtimeServerEventError; 