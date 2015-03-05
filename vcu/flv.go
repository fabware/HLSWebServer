// flv.go
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Header
const (
	SIGNATURE = "FLV"
	VERSION   = byte(0x01)

	TYPE_FLAGS_AUDIO = byte(0x04)
	TYPE_FLAGS_VIDEO = byte(0x01)

	DATA_OFFSET = uint32(9)
)

const TAG_HEADER_SIZE = 11
const AVC_HEADER_SIZE = 4

type TagType uint8

const (
	TAG_TYPE_AUDIO       = TagType(8)
	TAG_TYPE_VIDEO       = TagType(9)
	TAG_TYPE_SCRIPT_DATA = TagType(18)
)

type FrameType uint8

const (
	FRAME_TYPE_MAKS = byte(0xF0)

	FRAME_TYPE_KEYFRAME   = FrameType(1 << 4)
	FRAME_TYPE_INTER      = FrameType(2 << 4)
	FRAME_TYPE_DISPOSABLE = FrameType(3 << 4)
	FRAME_TYPE_GENERATED  = FrameType(4 << 4)
	FRAME_TYPE_COMMAND    = FrameType(5 << 4)
)

type CodecId uint8

const (
	CODEC_ID_MASK = byte(0x0F)

	CODEC_JPEG          = CodecId(1)
	CODEC_SORENSON_H263 = CodecId(2)
	CODEC_SCREEN_VIDEO  = CodecId(3)
	CODEC_VP6           = CodecId(4)
	CODEC_VP62_ALPHA    = CodecId(5)
	CODEC_SCREEN_VIDEO2 = CodecId(6)
	CODEC_AVC           = CodecId(7)
)

type AvcPacketType uint8

const (
	AVC_PACKET_SEQUENCE_HEADER = AvcPacketType(0)
	AVC_PACKET_NALU            = AvcPacketType(1)
	AVC_PACKET_END             = AvcPacketType(2)
)

type uint24 [3]byte

const MAX_UINT24 = (1 << 24) - 1

func uint24from32(source uint32) uint24 {
	if source > MAX_UINT24 {
		panic(fmt.Sprint("uint24 should be less or equal than ", MAX_UINT24))
	}
	var buffer [4]byte
	binary.BigEndian.PutUint32(buffer[:], source)
	var value uint24
	copy(value[:], buffer[1:4])
	return value
}

type FlvWriter struct {
	writer          io.Writer
	audio           bool
	video           bool
	previousTagSize uint32
}

func NewFlvWriter(writer io.Writer, audio bool, video bool) *FlvWriter {
	return &FlvWriter{
		writer: writer,
		audio:  audio,
		video:  video,
	}
}

func (flvWriter *FlvWriter) WriteHeader() error {
	var header [DATA_OFFSET]byte

	copy(header[0:3], []byte(SIGNATURE))
	header[3] = VERSION

	var flags = &header[4]
	if flvWriter.audio {
		*flags = *flags | TYPE_FLAGS_AUDIO
	}
	if flvWriter.video {
		*flags = *flags | TYPE_FLAGS_VIDEO
	}

	binary.BigEndian.PutUint32(header[5:9], DATA_OFFSET)

	_, writeErr := flvWriter.writer.Write(header[:])
	return writeErr
}

func (flvWriter *FlvWriter) WritePreviousTagSize() error {
	previousTagSizeData := make([]byte, binary.Size(flvWriter.previousTagSize))
	binary.BigEndian.PutUint32(previousTagSizeData, flvWriter.previousTagSize)

	if _, writeErr := flvWriter.writer.Write(previousTagSizeData); nil != writeErr {
		return writeErr
	}

	return nil
}

func (flvWriter *FlvWriter) WriteTag(tagType TagType, timestamp uint32, data []byte) error {
	var tagHeader [TAG_HEADER_SIZE]byte

	// TagType
	tagHeader[0] = byte(tagType)

	// DataSize
	var dataSize uint24 = uint24from32(uint32(len(data)))
	copy(tagHeader[1:4], dataSize[:])

	// Timestamp + TimestampExtended
	var timestampBuffer [4]byte
	binary.BigEndian.PutUint32(timestampBuffer[:], timestamp)
	copy(tagHeader[4:7], timestampBuffer[1:4])
	tagHeader[7] = timestampBuffer[0]

	// StreamID
	var streamId uint24 // 0
	copy(tagHeader[8:11], streamId[:])

	tagData := append(tagHeader[:], data...)

	flvWriter.previousTagSize = uint32(len(tagData))

	if _, writeErr := flvWriter.writer.Write(tagData); nil != writeErr {
		return writeErr
	}

	return nil
}

func (flvWriter *FlvWriter) WriteVideoTag(frameType FrameType, codecId CodecId, timestamp uint32, videoData []byte) error {
	var videoFlags uint8 = uint8(frameType) | uint8(codecId)

	data := append([]byte{videoFlags}, videoData...)

	return flvWriter.WriteTag(TAG_TYPE_VIDEO, timestamp, data)
}

var unsupportedAvcPacketType = errors.New("Unsupported AVC packet type")

func (flvWriter *FlvWriter) WriteAvcVideoTag(avcPacketType AvcPacketType, frameType FrameType, timestamp uint32, nal []byte) error {

	var avcHeader [AVC_HEADER_SIZE]byte
	avcHeader[0] = byte(avcPacketType)

	if AVC_PACKET_NALU == avcPacketType {
		var compositionTime uint24 = uint24from32(timestamp)
		copy(avcHeader[1:4], compositionTime[:])
	}

	data := append(avcHeader[:], nal...)

	return flvWriter.WriteVideoTag(frameType, CODEC_AVC, timestamp, data)
}
