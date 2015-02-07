// record
package record

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// 录像文件头
type recordFileHeader struct {
	MagicNum     [32]byte
	HeadSize     uint32
	VideoFmt     uint32
	Videofps     uint32
	VideoWidth   uint32
	VideoHeight  uint32
	AudioFmt     uint32
	AudioChannel uint32
	AudioSample  uint32
	TimeCost     uint32
	StartTime    [24]byte
	EndTime      [24]byte
	RecordType   uint32
	DevSn        [68]byte
	DevName      [68]byte
}

// 录像帧头
type recordFrame struct {
	FrameType uint32
	TimeStamp uint32
	FrameLen  uint32
}

type RecordFile struct {
	fileName        string
	fileHandle      *os.File
	isLowVer        bool //1.0 版本
	h264FileHandler *os.File
}

func (this *RecordFile) Open(fileName string) bool {
	this.fileName = fileName
	var err error
	this.fileHandle, err = os.Open(fileName)
	if err != nil {
		fmt.Println("Open", err)
		return false
	}

	this.h264FileHandler, err = os.OpenFile("my264.h264", os.O_CREATE|os.O_RDWR, 0777)

	return true
}

func (this *RecordFile) GetHeader() {

	var recordFileHead recordFileHeader
	var headerbuf [256]byte
	binary.Read(bytes.NewBuffer(headerbuf[:]), binary.LittleEndian, &recordFileHead)
	this.fileHandle.Read(headerbuf[:])
}

func (this *RecordFile) GetNextFrame() []byte {
	var frameHeaderBuf [12]byte
	_, readHeaderErr := this.fileHandle.Read(frameHeaderBuf[:])
	if readHeaderErr != nil {
		fmt.Println(readHeaderErr)
		return nil
	}
	var rf recordFrame
	binary.Read(bytes.NewBuffer(frameHeaderBuf[:]), binary.LittleEndian, &rf)
	if rf.FrameLen > 1000*0x1024 {
		return nil
	}
	frameBuf := make([]byte, rf.FrameLen)
	readLen, Err := this.fileHandle.Read(frameBuf)
	if Err != nil {
		fmt.Println(Err)
		return nil
	}

	fmt.Println("Read File Len", readLen, "Read Frame Len", rf.FrameLen)
	timeStamp := time.Now().UnixNano()
	databuf := new(bytes.Buffer)
	binary.Write(databuf, binary.LittleEndian, timeStamp)
	databuf.Write(frameBuf)
	this.h264FileHandler.Write(frameBuf)
	return databuf.Bytes()
}

func (this *RecordFile) Seek() {

	this.fileHandle.Seek(256, os.SEEK_SET)
}

func (this *RecordFile) Close() {
	this.fileHandle.Close()
}
