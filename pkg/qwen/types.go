package qwen

// EventType constants
const (
	EventTypeSessionUpdate                 = "session.update"
	EventTypeSessionUpdated                = "session.updated"
	EventTypeInputAudioBufferAppend        = "input_audio_buffer.append"
	EventTypeInputAudioBufferCommit        = "input_audio_buffer.commit" // Not used in VAD mode
	EventTypeSessionFinish                 = "session.finish"
	EventTypeSessionFinished               = "session.finished"
	EventTypeConversationItemCreated       = "conversation.item.created"
	EventTypeTranscriptionText             = "conversation.item.input_audio_transcription.text"
	EventTypeTranscriptionCompleted        = "conversation.item.input_audio_transcription.completed"
	EventTypeInputAudioBufferSpeechStarted = "input_audio_buffer.speech_started"
	EventTypeInputAudioBufferSpeechStopped = "input_audio_buffer.speech_stopped"
	EventTypeInputAudioBufferCommitted     = "input_audio_buffer.committed"
	EventTypeError                         = "error"
)

// But wait, Qwen API example:
// { "event_id": "event_123", "type": "session.update", "session": { ... } }
// It's flat, not header/payload like Volc.

// ClientEvent common fields
type ClientEvent struct {
	EventID string `json:"event_id"`
	Type    string `json:"type"`
}

type SessionUpdateEvent struct {
	EventID string  `json:"event_id"`
	Type    string  `json:"type"`
	Session Session `json:"session"`
}

type Session struct {
	Modalities              []string                `json:"modalities,omitempty"`         // ["text"]
	InputAudioFormat        string                  `json:"input_audio_format,omitempty"` // pcm, opus
	SampleRate              int                     `json:"sample_rate,omitempty"`        // 16000, 8000
	InputAudioTranscription InputAudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           *TurnDetection          `json:"turn_detection"` // pointer to allow null (no omitempty)
}

type InputAudioTranscription struct {
	Language string  `json:"language,omitempty"`
	Corpus   *Corpus `json:"corpus,omitempty"`
}

type Corpus struct {
	Text string `json:"text,omitempty"`
}

type TurnDetection struct {
	Type              string  `json:"type"` // server_vad
	Threshold         float64 `json:"threshold,omitempty"`
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
}

type InputAudioBufferAppendEvent struct {
	EventID string `json:"event_id"`
	Type    string `json:"type"`
	Audio   string `json:"audio"` // Base64
}

type SessionFinishEvent struct {
	EventID string `json:"event_id"`
	Type    string `json:"type"`
}

// Server Response Events

// We can just decode into a map or a struct with optional fields since different types have different fields.
type ServerEvent struct {
	EventID string `json:"event_id"`
	Type    string `json:"type"`
	// Common fields or specific objects
	Error *struct {
		Code    string `json:"code,omitempty"`
		Type    string `json:"type,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`

	// error (legacy flat fields if any, but above is what we saw)
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`

	// conversation.item.input_audio_transcription.text
	// conversation.item.input_audio_transcription.completed
	ItemID      string `json:"item_id,omitempty"`
	OutputIndex int    `json:"output_index,omitempty"`

	ContentIndex            int                            `json:"content_index,omitempty"`
	Text                    string                         `json:"text,omitempty"`
	Stash                   string                         `json:"stash,omitempty"`      // Running transcript in 'text' event
	Transcript              string                         `json:"transcript,omitempty"` // Final transcript in 'completed' event
	InputAudioTranscription *InputAudioTranscriptionResult `json:"input_audio_transcription,omitempty"`
}

type InputAudioTranscriptionResult struct {
	Completed bool   `json:"completed"`
	Text      string `json:"text"` // In 'completed' event
	// potentially timestamps etc.
}
