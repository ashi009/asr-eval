package qwen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"asr-eval/pkg/volc/common"
)

const (
	defaultURL      = "wss://dashscope.aliyuncs.com/api-ws/v1/realtime"
	segmentDuration = 200 // 200ms
)

type Client struct {
	model  string
	apiKey string
	url    string
}

func NewClient(model, apiKey string) *Client {
	return &Client{
		model:  model,
		apiKey: apiKey,
		url:    defaultURL,
	}
}

// Result holds the transcription result
type Result struct {
	Text      string
	IsFinal   bool
	Error     error
	RequestID string
}

func (c *Client) ProcessFile(ctx context.Context, filePath string, corpusText string, resChan chan<- Result) error {
	// 1. Prepare Audio
	pcmData, err := c.prepareAudio(filePath)
	if err != nil {
		return fmt.Errorf("failed to prepare audio: %v", err)
	}

	// 2. Connect WebSocket
	conn, err := c.connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	// 3. Send Session Update (Initial Config)
	if err := c.sendSessionUpdate(conn, corpusText); err != nil {
		return fmt.Errorf("failed to send session update: %v", err)
	}

	// 4. Start concurrent sending and receiving
	var wg sync.WaitGroup
	wg.Add(1)

	// Channel to signal session.updated
	readyChan := make(chan struct{})

	// Receiver routine
	go func() {
		defer wg.Done()
		c.receiveLoop(conn, resChan, readyChan)
	}()

	// Wait for session.updated
	select {
	case <-readyChan:
		log.Println("Session initialized (session.updated received)")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for session.updated")
	}

	// Optional delay (from JS example: "Wait for session config completion")
	time.Sleep(2 * time.Second)

	// 5. Send Audio
	err = c.sendAudio(conn, pcmData)
	if err != nil {
		log.Printf("Error sending audio: %v", err)
		// Don't return here, let the receiver finish or error out
	}

	// 6. Send Session Finish
	c.sendSessionFinish(conn)

	// Wait for receiver to finish (session.finished or error)
	wg.Wait()
	return nil
}

func (c *Client) prepareAudio(filePath string) ([]byte, error) {
	// Use common package to read/convert file to WAV bytes
	// Note: common.ConvertWavWithPath calls ffmpeg
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	isWav := common.JudgeWav(content)
	if !isWav {
		log.Printf("File is not WAV, converting: %s", filePath)
		content, err = common.ConvertWavWithPath(filePath, common.DefaultSampleRate)
		if err != nil {
			return nil, err
		}
	}
	log.Printf("Audio content size after preparation: %d bytes", len(content))

	// Parse WAV to find 'data' chunk
	// Standard WAV header parsing is brittle if there are extra chunks like LIST/INFO.
	// We need to scan for "data" chunk.

	// Skip 12 bytes (RIFF + size + WAVE)
	if len(content) < 12 {
		return nil, fmt.Errorf("wav content too short")
	}
	offset := 12
	var pcmData []byte

	for offset+8 < len(content) {
		chunkID := string(content[offset : offset+4])
		chunkSize := int(uint32(content[offset+4]) | uint32(content[offset+5])<<8 | uint32(content[offset+6])<<16 | uint32(content[offset+7])<<24)
		offset += 8

		if chunkID == "data" {
			// If chunkSize is larger than available data, just use what we have.
			// This happens if ffmpeg didn't update the header correctly or if file is truncated.
			available := len(content) - offset
			log.Printf("Found data chunk at offset %d, declared size %d, available %d", offset, chunkSize, available)

			if chunkSize > available {
				log.Printf("Data chunk size %d > available %d, using available", chunkSize, available)
				chunkSize = available
			}
			pcmData = content[offset : offset+chunkSize]
			break
		}

		// Skip other chunks (fmt , LIST, etc.)
		offset += chunkSize
	}

	if pcmData == nil {
		return nil, fmt.Errorf("data chunk not found in WAV")
	}

	return pcmData, nil
}

func (c *Client) connect(ctx context.Context) (*websocket.Conn, error) {
	u := fmt.Sprintf("%s?model=%s", c.url, c.model)
	headers := http.Header{}
	headers.Set("Authorization", "bearer "+c.apiKey)
	headers.Set("OpenAI-Beta", "realtime=v1")

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial failed: %v, status: %s", err, resp.Status)
		}
		return nil, fmt.Errorf("dial failed: %v", err)
	}
	// log.Printf("Connected to Qwen, X-Log-Id: %s", resp.Header.Get("X-Log-Id")) // hypothetical header
	return conn, nil
}

func (c *Client) sendSessionUpdate(conn *websocket.Conn, corpusText string) error {
	eventID := uuid.NewString()
	update := SessionUpdateEvent{
		EventID: eventID,
		Type:    EventTypeSessionUpdate,
		Session: Session{
			Modalities:       []string{"text"},
			InputAudioFormat: "pcm",
			SampleRate:       16000,
			TurnDetection: &TurnDetection{
				Type:              "server_vad",
				Threshold:         0.0,
				SilenceDurationMs: 400,
			},
		},
	}

	if corpusText != "" {
		update.Session.InputAudioTranscription = InputAudioTranscription{
			Corpus: &Corpus{
				Text: corpusText,
			},
		}
	}

	return conn.WriteJSON(update)
}

func (c *Client) sendAudio(conn *websocket.Conn, pcmData []byte) error {
	// Calculate chunk size: 16k * 1 channel * 2 bytes/sample * 0.2s = 6400 bytes
	chunkSize := 16000 * 2 * segmentDuration / 1000

	log.Printf("Starting to send audio. Total data size: %d bytes", len(pcmData))

	ticker := time.NewTicker(time.Duration(segmentDuration) * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < len(pcmData); i += chunkSize {
		end := i + chunkSize
		if end > len(pcmData) {
			end = len(pcmData)
		}
		chunk := pcmData[i:end]

		eventID := uuid.NewString()
		// Base64 encode
		b64Audio := base64.StdEncoding.EncodeToString(chunk)

		event := InputAudioBufferAppendEvent{
			EventID: eventID,
			Type:    EventTypeInputAudioBufferAppend,
			Audio:   b64Audio,
		}

		if err := conn.WriteJSON(event); err != nil {
			return err
		}

		<-ticker.C // Simulate real-time sending
	}

	// In VAD Mode, we do NOT send input_audio_buffer.commit.
	// The server handles turn detection.

	return nil
}

func (c *Client) sendSessionFinish(conn *websocket.Conn) error {
	event := SessionFinishEvent{
		EventID: uuid.NewString(),
		Type:    EventTypeSessionFinish,
	}
	return conn.WriteJSON(event)
}

func (c *Client) receiveLoop(conn *websocket.Conn, resChan chan<- Result, readyChan chan struct{}) {
	defer close(resChan)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ReadMessage error: %v", err)
			resChan <- Result{Error: err}
			return
		}

		var event ServerEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		if event.Type == "error" || event.Type == EventTypeTranscriptionText || event.Type == EventTypeTranscriptionCompleted {
			log.Printf("Raw message: %s", string(msg))
		}

		log.Printf("Received event: %s", event.Type)

		switch event.Type {
		case EventTypeSessionUpdated:
			close(readyChan)
		case EventTypeSessionFinished:
			return // Session done
		case EventTypeError:
			errMsg := "unknown error"
			if event.Error != nil {
				errMsg = fmt.Sprintf("%s - %s", event.Error.Code, event.Error.Message)
			} else if event.Code != "" {
				errMsg = fmt.Sprintf("%s - %s", event.Code, event.Message)
			}
			resChan <- Result{Error: fmt.Errorf("server error: %s", errMsg)}
			return
		case EventTypeTranscriptionText:
			txt := event.Text
			if txt == "" {
				txt = event.Stash
			}
			if txt != "" {
				// Stash seems to be the full accumulated text?
				// If Stash is accumulating, we just update lastTranscript.
				// If Stash is delta, we should accumulate.
				// Based on logs: "那个", "那个四十三", "那个四十三，那个玻璃。" -> It is ACCUMULATING.

				resChan <- Result{
					Text:    txt,
					IsFinal: false,
				}
			}
		case EventTypeTranscriptionCompleted:
			// Final text for a sentence
			txt := event.Transcript
			if txt == "" && event.InputAudioTranscription != nil {
				txt = event.InputAudioTranscription.Text
			}
			if txt != "" {
				// If we get completed event, it's definitely final for that segment.
				// We might want to clear lastTranscript if it was tracking this segment?
				// But since we are likely in VAD disabled mode or just one huge segment...
				resChan <- Result{
					Text:    txt,
					IsFinal: true,
				}
				// If it's single utterance logic, we might be done?
				// But let's keep going until session finish.
			}
		}
	}
}
