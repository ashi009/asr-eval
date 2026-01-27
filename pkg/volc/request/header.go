package request

import (
	"bytes"
	"net/http"

	"github.com/google/uuid"

	"asr-eval/pkg/volc/common"
	"asr-eval/pkg/volc/config"
)

type AsrRequestHeader struct {
	messageType              common.MessageType
	messageTypeSpecificFlags common.MessageTypeSpecificFlags
	serializationType        common.SerializationType
	compressionType          common.CompressionType
	reservedData             []byte
}

func (h *AsrRequestHeader) toBytes() []byte {
	header := bytes.NewBuffer([]byte{})
	header.WriteByte(byte(common.PROTOCOL_VERSION<<4 | 1))
	header.WriteByte(byte(h.messageType<<4) | byte(h.messageTypeSpecificFlags))
	header.WriteByte(byte(h.serializationType<<4) | byte(h.compressionType))
	header.Write(h.reservedData)
	return header.Bytes()
}

func (h *AsrRequestHeader) WithMessageType(messageType common.MessageType) *AsrRequestHeader {
	h.messageType = messageType
	return h
}

func (h *AsrRequestHeader) WithMessageTypeSpecificFlags(messageTypeSpecificFlags common.MessageTypeSpecificFlags) *AsrRequestHeader {
	h.messageTypeSpecificFlags = messageTypeSpecificFlags
	return h
}

func (h *AsrRequestHeader) WithSerializationType(serializationType common.SerializationType) *AsrRequestHeader {
	h.serializationType = serializationType
	return h
}

func (h *AsrRequestHeader) WithCompressionType(compressionType common.CompressionType) *AsrRequestHeader {
	h.compressionType = compressionType
	return h
}

func (h *AsrRequestHeader) WithReservedData(reservedData []byte) *AsrRequestHeader {
	h.reservedData = reservedData
	return h
}

func DefaultHeader() *AsrRequestHeader {
	return &AsrRequestHeader{
		messageType:              common.CLIENT_FULL_REQUEST,
		messageTypeSpecificFlags: common.POS_SEQUENCE,
		serializationType:        common.JSON,
		compressionType:          common.GZIP,
		reservedData:             []byte{0x00},
	}
}

// Model version constants
const (
	ModelV1 = "v1" // volc.bigasr.sauc.duration
	ModelV2 = "v2" // volc.seedasr.sauc.duration
)

var modelResourceIDs = map[string]string{
	ModelV1: "volc.bigasr.sauc.duration",
	ModelV2: "volc.seedasr.sauc.duration",
}

// CurrentModelVersion is the globally configured model version
var CurrentModelVersion = ModelV2

func SetModelVersion(version string) {
	if _, ok := modelResourceIDs[version]; ok {
		CurrentModelVersion = version
	}
}

func NewAuthHeader() http.Header {
	reqid := uuid.New().String()
	header := http.Header{}

	resourceID := modelResourceIDs[CurrentModelVersion]
	header.Add("X-Api-Resource-Id", resourceID)
	header.Add("X-Api-Connect-Id", reqid)
	header.Add("X-Api-Access-Key", config.AccessKey())
	header.Add("X-Api-App-Key", config.AppKey())
	return header
}
