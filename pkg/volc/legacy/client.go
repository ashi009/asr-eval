package legacy

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

// Constants from the demo
type ProtocolVersion byte
type MessageType byte
type MessageTypeSpecificFlags byte
type SerializationType byte
type CompressionType byte

const (
	SuccessCode = 1000

	PROTOCOL_VERSION    = ProtocolVersion(0b0001)
	DEFAULT_HEADER_SIZE = 0b0001

	PROTOCOL_VERSION_BITS            = 4
	HEADER_BITS                      = 4
	MESSAGE_TYPE_BITS                = 4
	MESSAGE_TYPE_SPECIFIC_FLAGS_BITS = 4
	MESSAGE_SERIALIZATION_BITS       = 4
	MESSAGE_COMPRESSION_BITS         = 4
	RESERVED_BITS                    = 8

	// Message Type:
	CLIENT_FULL_REQUEST       = MessageType(0b0001)
	CLIENT_AUDIO_ONLY_REQUEST = MessageType(0b0010)
	SERVER_FULL_RESPONSE      = MessageType(0b1001)
	SERVER_ACK                = MessageType(0b1011)
	SERVER_ERROR_RESPONSE     = MessageType(0b1111)

	// Message Type Specific Flags
	NO_SEQUENCE    = MessageTypeSpecificFlags(0b0000) // no check sequence
	POS_SEQUENCE   = MessageTypeSpecificFlags(0b0001)
	NEG_SEQUENCE   = MessageTypeSpecificFlags(0b0010)
	NEG_SEQUENCE_1 = MessageTypeSpecificFlags(0b0011)

	// Message Serialization
	NO_SERIALIZATION = SerializationType(0b0000)
	JSON             = SerializationType(0b0001)
	THRIFT           = SerializationType(0b0011)
	CUSTOM_TYPE      = SerializationType(0b1111)

	// Message Compression
	NO_COMPRESSION     = CompressionType(0b0000)
	GZIP               = CompressionType(0b0001)
	CUSTOM_COMPRESSION = CompressionType(0b1111)
)

var DefaultFullClientWsHeader = []byte{0x11, 0x10, 0x11, 0x00}
var DefaultAudioOnlyWsHeader = []byte{0x11, 0x20, 0x11, 0x00}
var DefaultLastAudioWsHeader = []byte{0x11, 0x22, 0x11, 0x00}

type AsrResponse struct {
	Reqid    string   `json:"reqid"`
	Code     int      `json:"code"`
	Message  string   `json:"message"`
	Sequence int      `json:"sequence"`
	Results  []Result `json:"result,omitempty"`
}

type Result struct {
	Text       string      `json:"text"`
	Confidence int         `json:"confidence"`
	Language   string      `json:"language,omitempty"`
	Utterances []Utterance `json:"utterances,omitempty"`
}

type Utterance struct {
	Text      string `json:"text"`
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
	Definite  bool   `json:"definite"`
	Words     []Word `json:"words"`
	Language  string `json:"language"`
}

type Word struct {
	Text          string `json:"text"`
	StartTime     int    `json:"start_time"`
	EndTime       int    `json:"end_time"`
	Pronounce     string `json:"pronounce"`
	BlankDuration int    `json:"blank_duration"`
}

type AsrClient struct {
	Appid    string
	Token    string
	Cluster  string
	Workflow string
	Format   string
	Codec    string
	SegSize  int
	Url      string
}

func NewAsrClient(appid, token, cluster string) *AsrClient {
	return &AsrClient{
		Appid:    appid,
		Token:    token,
		Cluster:  cluster,
		Workflow: "audio_in,resample,partition,vad,fe,decode",
		SegSize:  160000,
		Format:   "wav", // Although user said flac, demo defaults to wav. We might need to adjust or let API handle it.
		Codec:    "raw",
		Url:      "wss://openspeech.bytedance.com/api/v2/asr",
	}
}

func (client *AsrClient) ProcessAudio(audioData []byte, format string) (*AsrResponse, error) {
	client.Format = format
	// set token header
	var tokenHeader = http.Header{"Authorization": []string{fmt.Sprintf("Bearer;%s", client.Token)}}
	c, _, err := websocket.DefaultDialer.Dial(client.Url, tokenHeader)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	defer c.Close()

	// 1. send full client request
	req := client.constructRequest()
	payload := gzipCompress(req)
	payloadSize := len(payload)
	payloadSizeArr := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadSizeArr, uint32(payloadSize))

	fullClientMsg := make([]byte, len(DefaultFullClientWsHeader))
	copy(fullClientMsg, DefaultFullClientWsHeader)
	fullClientMsg = append(fullClientMsg, payloadSizeArr...)
	fullClientMsg = append(fullClientMsg, payload...)
	if err := c.WriteMessage(websocket.BinaryMessage, fullClientMsg); err != nil {
		return nil, fmt.Errorf("write full client message error: %w", err)
	}

	_, msg, err := c.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read full client response error: %w", err)
	}
	asrResponse, err := client.parseResponse(msg)
	if err != nil {
		return nil, fmt.Errorf("parse full client response error: %w", err)
	}

	// Check if initial response signals error
	if asrResponse.Code != 0 && asrResponse.Code != SuccessCode {
		// Some specific error handling if needed, but for now continue or return
	}

	// 3. send segment audio request
	// Determine final response to return (accumulated or just last one)
	// The demo returns the last response which usually contains the full text in `result_type="full"` mode?
	// Actually the demo updates `asrResponse` in the loop.

	// Note: The demo sets Codec="raw". For FLAC/WAV files, we might need to send the file header too if it's not raw PCM.
	// If format is "flac", we should check if we need to skip header or if API handles it.
	// Documentation usually says formatting needs to match.
	// If we send raw bytes of a FLAC file, format should be "flac" (if supported) or we decode to PCM.
	// Let's assume we send the file content as is and set format correctly.

	for sentSize := 0; sentSize < len(audioData); sentSize += client.SegSize {
		lastAudio := false
		if sentSize+client.SegSize >= len(audioData) {
			lastAudio = true
		}
		dataSlice := make([]byte, 0)
		audioMsg := make([]byte, len(DefaultAudioOnlyWsHeader))
		if !lastAudio {
			dataSlice = audioData[sentSize : sentSize+client.SegSize]
			copy(audioMsg, DefaultAudioOnlyWsHeader)
		} else {
			dataSlice = audioData[sentSize:]
			copy(audioMsg, DefaultLastAudioWsHeader)
		}
		payload = gzipCompress(dataSlice)
		payloadSize := len(payload)
		payloadSizeArr := make([]byte, 4)
		binary.BigEndian.PutUint32(payloadSizeArr, uint32(payloadSize))
		audioMsg = append(audioMsg, payloadSizeArr...)
		audioMsg = append(audioMsg, payload...)
		if err := c.WriteMessage(websocket.BinaryMessage, audioMsg); err != nil {
			return nil, fmt.Errorf("write audio message error: %w", err)
		}
		_, msg, err := c.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read audio response error: %w", err)
		}
		asrResponse, err = client.parseResponse(msg)
		if err != nil {
			return nil, fmt.Errorf("parse audio response error: %w", err)
		}
	}
	return &asrResponse, nil
}

func (client *AsrClient) constructRequest() []byte {
	reqid := uuid.NewV4().String()
	req := make(map[string]map[string]interface{})
	req["app"] = make(map[string]interface{})
	req["app"]["appid"] = client.Appid
	req["app"]["cluster"] = client.Cluster
	req["app"]["token"] = client.Token
	req["user"] = make(map[string]interface{})
	req["user"]["uid"] = "uid"
	req["request"] = make(map[string]interface{})
	req["request"]["reqid"] = reqid
	req["request"]["nbest"] = 1
	req["request"]["workflow"] = client.Workflow
	req["request"]["result_type"] = "full"
	req["request"]["sequence"] = 1
	req["audio"] = make(map[string]interface{})
	req["audio"]["format"] = client.Format
	// For file based upload, it seems safer to keep codec as default (raw?) or empty if format implies it?
	// Demo uses "raw". If we upload flac file bytes, we probably should ensure codec makes sense.
	// Volcengine docs say: format: audio format, support wav, pcm, ogg, opus, m4a, mp3, aac, flac...
	// codec: compression format, support raw, opus, speex...
	// If format is flac, codec might be ignored or should be consistent.
	req["audio"]["codec"] = client.Codec
	reqStr, _ := json.Marshal(req)
	return reqStr
}

func (client *AsrClient) parseResponse(msg []byte) (AsrResponse, error) {
	//protocol_version := msg[0] >> 4
	headerSize := msg[0] & 0x0f
	messageType := msg[1] >> 4
	//message_type_specific_flags := msg[1] & 0x0f
	serializationMethod := msg[2] >> 4
	messageCompression := msg[2] & 0x0f
	//reserved := msg[3]
	//header_extensions := msg[4:header_size * 4]
	payload := msg[headerSize*4:]
	payloadMsg := make([]byte, 0)
	payloadSize := 0

	if messageType == byte(SERVER_FULL_RESPONSE) {
		payloadSize = int(int32(binary.BigEndian.Uint32(payload[0:4])))
		payloadMsg = payload[4:]
	} else if messageType == byte(SERVER_ACK) {
		// seq := int32(binary.BigEndian.Uint32(payload[:4]))
		if len(payload) >= 8 {
			payloadSize = int(binary.BigEndian.Uint32(payload[4:8]))
			payloadMsg = payload[8:]
		}
		// fmt.Println("SERVER_ACK seq: ", seq)
	} else if messageType == byte(SERVER_ERROR_RESPONSE) {
		code := int32(binary.BigEndian.Uint32(payload[:4]))
		// payloadSize = int(binary.BigEndian.Uint32(payload[4:8]))
		payloadMsg = payload[8:]
		return AsrResponse{}, fmt.Errorf("SERVER_ERROR_RESPONSE code: %d, msg: %s", code, string(payloadMsg))
	}
	if payloadSize == 0 {
		return AsrResponse{}, nil // ACK usually has no payload of interest for ASR result?
	}
	if messageCompression == byte(GZIP) {
		payloadMsg = gzipDecompress(payloadMsg)
	}

	var asrResponse = AsrResponse{}
	if serializationMethod == byte(JSON) {
		err := json.Unmarshal(payloadMsg, &asrResponse)
		if err != nil {
			return AsrResponse{}, fmt.Errorf("unmarshal error: %w", err)
		}
	}
	return asrResponse, nil
}

func gzipCompress(input []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(input)
	w.Close()
	return b.Bytes()
}

func gzipDecompress(input []byte) []byte {
	b := bytes.NewBuffer(input)
	r, _ := gzip.NewReader(b)
	out, _ := ioutil.ReadAll(r)
	r.Close()
	return out
}
