// client.go
package main

// 心跳，双向产生，需要了解状态的都可以通过发送心跳，心跳接受方有义务回复心跳
import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"
	"utility/base"
)

type CUClient struct {
	connSocket net.Conn
	debugChn   *os.File
	ffmpegChn  net.Conn

	frameCount uint32

	sn         string
	clientID   uint32
	Valid      uint32
	heartChn   chan bool
	token      string
	count      uint32
	hlsHandler *RawData2Hls
}

type EnvParam struct {
	ResourceDir  string
	ResourceFile string

	BeginID       uint
	ResourceCount uint

	TimeOut     int  // 重连超时
	SupportStat bool // 支持统计

	url string

	auto bool
}

var oneEnvParam *EnvParam = nil

func (cu *CUClient) handleError(err error) {

	if atomic.CompareAndSwapUint32(&cu.Valid, 0, 0) {
		return
	}
	fmt.Println(err)
	//stat.GetLocalStatistInst().Off()
	atomic.CompareAndSwapUint32(&cu.Valid, 1, 0)
	cu.connSocket.Close()
	close(cu.heartChn)

	if cu.ffmpegChn != nil {
		fmt.Println("Close ffmpegchn")
		cu.ffmpegChn.Close()
		cu.ffmpegChn = nil
	}
	ReleaseProxy(cu.sn)
}

func frameRateChange(data []byte) []byte {
	var back [4]byte
	var result [5]byte
	var num_units_in_tick uint32 = 0
	for i := 0; i < 4; i++ {
		back[i] = ((data[i] & 0x07) << 5) | ((data[i+1] & 0xF8) >> 3)
	}

	backBuf := bytes.NewBuffer(back[:])
	binary.Read(backBuf, binary.BigEndian, &num_units_in_tick)
	fmt.Println("num_units_in_tick ", num_units_in_tick)
	num_units_in_tick *= 2
	changeBuf := new(bytes.Buffer)
	binary.Write(changeBuf, binary.BigEndian, &num_units_in_tick)

	tmp := changeBuf.Bytes()
	result[0] = (data[0] | (tmp[0] >> 5))
	result[1] = ((tmp[0] << 3) | (tmp[1] >> 5))
	result[2] = ((tmp[1] << 3) | (tmp[2] >> 5))
	result[3] = ((tmp[2] << 3) | (tmp[3] >> 5))
	result[4] = ((tmp[3] << 3) | (data[4] & 0x07))
	fmt.Println("Change Buffer ", result)
	fmt.Println(tmp)

	return result[:]
}

func (cu *CUClient) heartTask() {
	defer func() {
		if r := recover(); r != nil {
			cu.handleError(base.DTerror{"heartTask Panic"})
		}
	}()

	baseHD := new(base.BaseHeader)
	baseHD.CommandId = base.HEART_CMD
	msg := baseHD.Encode()

	var failCount int = 0

	for {

		if atomic.CompareAndSwapUint32(&cu.Valid, 0, 0) {
			break
		}
		cu.connSocket.Write(msg.Bytes())
		select {
		case <-cu.heartChn:
			failCount = 0
		case <-time.After(10 * time.Second):
			failCount++
		}
		if failCount >= 3 {
			cu.handleError(base.DTerror{"CU　Heart"})
			break
		}
		time.Sleep(10 * time.Second)
	}
}

func (cu *CUClient) timerClear() {

	var handle *os.File = nil
	for handle == nil && atomic.CompareAndSwapUint32(&cu.Valid, 1, 1) {
		handle, _ = os.Open("../../" + cu.sn + ".m3u8")
		if handle == nil {
			fmt.Println("Open Fail")
			time.Sleep(10 * time.Second)
		}

	}
	for atomic.CompareAndSwapUint32(&cu.Valid, 1, 1) {
		var buf [128]byte
		handle.Seek(0, os.SEEK_SET)
		handle.Read(buf[:])
		splitByte := bytes.Split(buf[:], []byte("\n"))

		for i := 0; i < len(splitByte); i++ {

			if bytes.HasPrefix(splitByte[i], []byte("#EXT-X-MEDIA-SEQUENCE")) {
				countStr := bytes.Split(splitByte[i], []byte(":"))[1]
				count, _ := strconv.Atoi(string(countStr))
				for j := 0; j < count; j++ {
					filename := new(bytes.Buffer)

					fmt.Fprintf(filename, "../../%s_%d.ts",
						cu.sn,
						j)

					err := os.Remove(filename.String())
					fmt.Println("Remove File ", filename, " ", err)

				}
				break
			}
		}

		time.Sleep(10 * time.Second)
	}
	// 检查文件是否创建

}

func (cu *CUClient) readTask() {
	defer func() {
		if r := recover(); r != nil {
			cu.handleError(base.DTerror{})
		}
	}()

	for {
		proto := new(base.Proto)

		err := proto.ReadBinaryProto(cu.connSocket)
		if err != nil {
			cu.handleError(err)
			return
		}

		cmdID := proto.RD.BaseHD.CommandId & 0x7f
		//fmt.Println("Recv CommandID ", cmdID)
		if cmdID == base.HEART_CMD {
			isRequest := proto.RD.BaseHD.CommandId & 0x80
			if isRequest == 0 {

				heartHdr := new(base.BaseHeader)
				heartHdr.CommandId = proto.RD.BaseHD.CommandId | 0x80
				msg := heartHdr.Encode()
				_, err := cu.connSocket.Write(msg.Bytes())

				if err != nil {
					cu.handleError(base.DTerror{"Send Error " + err.Error()})
				}
				//fmt.Println("Send Heart")
			} else {
				cu.heartChn <- true
				//fmt.Println("Recv Heart")
			}

		} else if cmdID == base.DATA_STREAM {
			//	stat.GetLocalStatistInst().RecvData(uint64(proto.RD.HD.BodyLen))
			var frameType uint16 = 0
			var err error = nil
			err = binary.Read(bytes.NewBuffer(proto.BD.Data[10:12]), binary.LittleEndian, &frameType)
			if err != nil {
				fmt.Println(err)
			}

			// 由于我们公司的h264数据的sps中video_format没有指定yuv格式
			// 所以在这里直接指定为2,即yuv420p
			if frameType == 1 {
				cu.frameCount++
			}
			if cu.frameCount == 1 {
				//proto.BD.Data[32] = 0x5D
				proto.BD.Data[42] = 0x90
				result := frameRateChange(proto.BD.Data[33:38])
				for i := 0; i < 5; i++ {
					proto.BD.Data[33+i] = result[i]
				}
				backSPS := bytes.NewBuffer(proto.BD.Data[20:])

				if cu.ffmpegChn != nil {
					cu.ffmpegChn.Write(backSPS.Bytes())
				} else if cu.hlsHandler != nil {
					cu.hlsHandler.Write(backSPS.Bytes())
				} else if cu.debugChn != nil {
					cu.debugChn.Write(backSPS.Bytes())
				}

				continue
			} else if cu.frameCount == 3 {
				cu.frameCount = 0
			}

			if cu.ffmpegChn != nil {
				cu.ffmpegChn.Write(proto.BD.Data[20:])
			} else if cu.hlsHandler != nil {
				cu.hlsHandler.Write(proto.BD.Data[20:])
			} else if cu.debugChn != nil {
				cu.debugChn.Write(proto.BD.Data[20:])
			}

		} else if cmdID == base.OPEN_RESOURCE_CMD {
			var responseJson base.ResponseJson
			fmt.Println("Open Response :", string(proto.BD.Data[4:]))
			unerr := json.Unmarshal(proto.BD.Data[4:], &responseJson)
			if unerr != nil {
				cu.handleError(unerr)
				break
			}

			if !bytes.Equal([]byte("ok"), []byte(responseJson.Error)) {
				cu.handleError(unerr)
				break
			}
			//stat.GetLocalStatistInst().OpenRes()
		} else if cmdID == base.CLOSE_RESOURCE_CMD {
			var responseJson base.ResponseJson
			//stat.GetLocalStatistInst().CloseRes()
			fmt.Println("Close Response :", string(proto.BD.Data[4:]))
			unerr := json.Unmarshal(proto.BD.Data[4:], &responseJson)
			if unerr != nil {
				cu.handleError(unerr)
				break
			}

		} else if cmdID == base.LEAVE_NOTIFY {
			atomic.AddUint32(&cu.clientID, 1)
			//			stat.GetLocalStatistInst().CloseRes()
			go cu.openSource(cu.clientID, cu.sn)
			break
		}

	}
}

func (cu *CUClient) openSource(clientID uint32, id string) error {

	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = base.OPEN_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID
	proto.RD.HD.ContextType = base.CONTEXT_JSON
	proto.RD.HD.TransferType = base.TRANSFER_RAW

	resourceJson := base.OpenResParamJson{id, 10}

	requestJson := base.RequestJson{cu.token, "ns", "open", resourceJson}
	b, err := json.Marshal(requestJson)
	if err != nil {
		cu.handleError(err)
		return err
	}

	proto.EncodeBody(b)
	result := proto.EncodeHdr()
	n, err := cu.connSocket.Write(result.Bytes())
	if err != nil {
		cu.handleError(err)
		return err
	}
	n = n

	_, bErr := cu.connSocket.Write(proto.BD.Data)
	if err != nil {
		cu.handleError(bErr)
		return err
	}
	go cu.timerClear()
	return err
}

func (cu *CUClient) closeSource(clientID uint32, id string) {

	var err error
	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = base.CLOSE_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID
	proto.RD.HD.ContextType = base.CONTEXT_JSON
	proto.RD.HD.TransferType = base.TRANSFER_RAW

	resourceJson := base.CloseResParamJson{id}

	requestJson := base.RequestJson{"", "ns", "close", resourceJson}
	b, err := json.Marshal(requestJson)
	if err != nil {
		cu.handleError(err)
		return
	}

	proto.EncodeBody(b)
	result := proto.EncodeHdr()
	n, err := cu.connSocket.Write(result.Bytes())
	if err != nil {
		cu.handleError(err)
		return
	}
	n = n

	_, bErr := cu.connSocket.Write(proto.BD.Data)
	if err != nil {
		cu.handleError(bErr)
	}
	return
}

func (cu *CUClient) run() {

	fmt.Println(oneEnvParam.url)
	connSocket, err := net.DialTimeout("tcp", oneEnvParam.url, 2*time.Second)

	if err != nil {
		cu.handleError(err)
		return
	}

	//stat.GetLocalStatistInst().On()

	cu.connSocket = connSocket
	atomic.CompareAndSwapUint32(&cu.Valid, 0, 1)

	cu.heartChn = make(chan bool, 1)

	atomic.AddUint32(&cu.clientID, 1)
	go cu.readTask()
	go cu.openSource(cu.clientID, cu.sn)
}

func (cu *CUClient) Stop() {
	cu.handleError(base.DTerror{"Close Video"})
}
