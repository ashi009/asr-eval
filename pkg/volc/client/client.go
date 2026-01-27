package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"asr-eval/pkg/volc/common"
	"asr-eval/pkg/volc/request"
	"asr-eval/pkg/volc/response"
)

type AsrWsClient struct {
	seq             int
	segmentDuration int
	url             string
	connect         *websocket.Conn
	context         string
}

func NewAsrWsClient(url string, segmentDuration int) *AsrWsClient {
	return &AsrWsClient{
		seq:             1,
		url:             url,
		segmentDuration: segmentDuration,
	}
}

func (c *AsrWsClient) SetContext(ctx string) {
	c.context = ctx
}

func (c *AsrWsClient) readAudioData(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("failed to read file: %s", err)
	}
	isWav := common.JudgeWav(content)
	if !isWav {
		content, err = common.ConvertWavWithPath(filePath, common.DefaultSampleRate)
		if err != nil {
			return nil, fmt.Errorf("convert wav err: %w", err)
		}
	}
	return content, nil
}

func (c *AsrWsClient) getSegmentSize(content []byte) (int, error) {
	channelNum, sampWidth, frameRate, _, _, err := common.ReadWavInfo(content)
	if err != nil {
		return 0, fmt.Errorf("failed to read wav info: %w", err)
	}
	sizePerSec := channelNum * sampWidth * frameRate
	segmentSize := sizePerSec * c.segmentDuration / 1000
	return segmentSize, nil
}

func (c *AsrWsClient) createConnection(ctx context.Context) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("Connection attempt %d failed, retrying in %v...", attempt, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		header := request.NewAuthHeader()
		conn, resp, err := websocket.DefaultDialer.DialContext(ctx, c.url, header)
		if err != nil {
			lastErr = fmt.Errorf("dial websocket err: %w", err)
			continue
		}
		log.Printf("logid: %s", resp.Header.Get("X-Tt-Logid"))
		c.connect = conn
		return nil
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *AsrWsClient) sendFullClientRequest() error {
	fullClientRequest := request.NewFullClientRequest(&request.RequestMeta{
		Corpus: request.CorpusMeta{
			Context: c.context,
		},
	})
	c.seq++
	err := c.connect.WriteMessage(websocket.BinaryMessage, fullClientRequest)
	if err != nil {
		return fmt.Errorf("full client message write websocket err: %w", err)
	}
	_, resp, err := c.connect.ReadMessage()
	if err != nil {
		return fmt.Errorf("full client message read err: %w", err)
	}
	respStruct := response.ParseResponse(resp)
	log.Println(respStruct)
	return nil
}

func (c *AsrWsClient) sendMessages(segmentSize int, content []byte, stopChan <-chan struct{}) error {
	messageChan := make(chan []byte)
	go func() {
		for message := range messageChan {
			err := c.connect.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("write message err: %s", err)
				return
			}
		}
	}()

	audioSegments := splitAudio(content, segmentSize)

	ticker := time.NewTicker(time.Duration(c.segmentDuration) * time.Millisecond)
	defer ticker.Stop()
	defer close(messageChan)
	log.Printf("Start sending audio segments. Total segments: %d, Segment size: %d", len(audioSegments), segmentSize)
	for _, segment := range audioSegments {
		select {
		case <-ticker.C:
			if c.seq == len(audioSegments)+1 {
				c.seq = -c.seq
			}
			message := request.NewAudioOnlyRequest(c.seq, segment)
			messageChan <- message
			log.Printf("Sent segment seq: %d / %d", c.seq, len(audioSegments))
			c.seq++
		case <-stopChan:
			log.Println("Stop signal received in sendMessages")
			return nil
		}
	}
	log.Println("Finished sending all segments")
	return nil
}

func (c *AsrWsClient) recvMessages(resChan chan<- *response.AsrResponse, stopChan chan<- struct{}) {
	defer close(resChan)
	for {
		_, message, err := c.connect.ReadMessage()
		if err != nil {
			log.Printf("ReadMessage error: %v", err)
			return
		}
		resp := response.ParseResponse(message)
		log.Printf("Received response: Seq=%d, Code=%d, TextLen=%d, IsLast=%v",
			resp.PayloadSequence, resp.Code, len(resp.PayloadMsg.Result.Text), resp.IsLastPackage)
		resChan <- resp
		if resp.IsLastPackage {
			log.Println("Received last package")
			return
		}
		if resp.Code != 0 {
			close(stopChan)
			return
		}
	}
}

func (c *AsrWsClient) startAudioStream(segmentSize int, content []byte, resChan chan<- *response.AsrResponse) error {
	stopChan := make(chan struct{})
	go func() {
		err := c.sendMessages(segmentSize, content, stopChan)
		if err != nil {
			log.Fatalf("failed to send audio stream: %s", err)
			return
		}
	}()
	c.recvMessages(resChan, stopChan)
	return nil
}

func (c *AsrWsClient) Excute(ctx context.Context, filePath string, resChan chan<- *response.AsrResponse) error {
	if filePath == "" {
		return errors.New("file path is empty")
	}
	c.seq = 1
	if c.url == "" {
		return errors.New("url is empty")
	}
	content, err := c.readAudioData(filePath)
	if err != nil {
		return fmt.Errorf("read audio data err: %w", err)
	}
	segmentSize, err := c.getSegmentSize(content)
	if err != nil {
		return fmt.Errorf("get segment size err: %w", err)
	}

	err = c.createConnection(ctx)
	if err != nil {
		return fmt.Errorf("create connection err: %w", err)
	}
	err = c.sendFullClientRequest()
	if err != nil {
		return fmt.Errorf("send full request err: %w", err)
	}
	err = c.startAudioStream(segmentSize, content, resChan)
	if err != nil {
		return fmt.Errorf("start audio stream err: %w", err)
	}
	return nil
}

func splitAudio(data []byte, segmentSize int) [][]byte {
	if segmentSize <= 0 {
		return nil // 返回空切片，如果 chunkSize 非法
	}
	var segments [][]byte
	for i := 0; i < len(data); i += segmentSize {
		end := i + segmentSize
		if end > len(data) {
			end = len(data)
		}
		segments = append(segments, data[i:end])
	}
	return segments
}
