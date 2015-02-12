// client.go
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"
	"utility/base"
	"utility/mylog"
	"utility/stat"
	"vpu/record"
)

/*

	1. 注册资源，注销资源
	2. 支持打开资源、关闭资源
	3. 支持在一条连接上注册多个资源
	4. 支持资源在不同连接注册
	5. 支持资源超时管理，一个资源在一定时间内没有访问，请求注销资源
	6. 心跳，双向产生，需要了解状态的都可以通过发送心跳，心跳接受方有义务回复心跳
	7. 每一个连接通道同时支持资源提供者和访问者
	8. CMDID最高位作为判断请求和回应
	9. 资源ID即使细化到数据流
*/

type EnvParam struct {
	ResourceDir  string
	ResourceFile string

	BeginID       uint
	ResourceCount uint

	TimeOut     int  // 重连超时
	SupportStat bool // 支持统计

	Url string
}

var oneEnvParam *EnvParam = nil

type PUDevice struct {
	connSocket net.Conn
	SendFlag   uint32
	SN         string
	Valid      uint32
	rwChan     chan *base.Proto
}

func (pu *PUDevice) handleError(err error) {
	mylog.GetErrorLogger().Println(err)
	if atomic.CompareAndSwapUint32(&pu.Valid, 0, 0) {
		return
	}

	atomic.CompareAndSwapUint32(&pu.Valid, 1, 0)
	pu.connSocket.Close()
	stat.GetLocalStatistInst().Off()
	if atomic.CompareAndSwapUint32(&pu.SendFlag, 1, 1) {
		stat.GetLocalStatistInst().CloseRes()
	}
	close(pu.rwChan)
	go pu.ReRun(pu.SN)
}

func (pu *PUDevice) readTask() {
	defer func() {
		if r := recover(); r != nil {
			mylog.GetErrorLogger().Println("readTask panic")
			return
		}
	}()
	for {
		proto := new(base.Proto)

		err := proto.ReadBinaryProto(pu.connSocket)
		if err != nil {
			pu.handleError(err)

			break
		}

		cmdID := proto.RD.BaseHD.CommandId & 0x7f

		if cmdID == base.HEART_CMD {
			mylog.GetErrorLogger().Println("Recv And Send Heart", proto.RD.BaseHD.CommandId)
			heartProto := new(base.Proto)
			heartProto.RD.BaseHD.CommandId = 0x80 | cmdID
			pu.rwChan <- heartProto

		} else {
			mylog.GetErrorLogger().Println(string(proto.BD.Data[4:]))
			if cmdID == base.OPEN_RESOURCE_CMD {
				go pu.openSource(proto.RD.HD.ClientIdent)
			} else if cmdID == base.CLOSE_RESOURCE_CMD {
				go pu.closeSource(proto.RD.HD.ClientIdent)
			} else if cmdID == base.REGISTER_RESOURCE {
				var reponseJson base.ResponseJson
				resErr := json.Unmarshal(proto.BD.Data[4:], &reponseJson)
				if resErr != nil {
					pu.handleError(resErr)
					return
				}

				if reponseJson.Error != base.OK {
					pu.handleError(base.DTerror{"Resoure Fail"})
					fmt.Println(reponseJson.Error)
					return
				}
				stat.GetLocalStatistInst().RegisterRes()
			}
		}

	}
}

func (pu *PUDevice) handleReadChnTask() {
	//fmt.Println("Begin handleReadChnTask")

	defer func() {
		if r := recover(); r != nil {
			pu.handleError(base.DTerror{"handleReadChnTask Panic"})
		}
	}()

	for atomic.CompareAndSwapUint32(&pu.Valid, 1, 1) {
		select {
		case proto, ok := <-pu.rwChan:
			if ok {
				pu.handleWriteSocket(proto)
			} else {
				pu.handleError(base.DTerror{"handleReadChnTask Panic"})

			}
		default:
			time.Sleep(10 * time.Millisecond)

		}
	}
}

func (pu *PUDevice) handleWriteSocket(proto *base.Proto) {

	if atomic.CompareAndSwapUint32(&pu.Valid, 0, 0) {
		return
	}
	cmdID := proto.RD.BaseHD.CommandId & 0x7f

	if cmdID == base.HEART_CMD {
		heartHdr := new(base.BaseHeader)
		heartHdr.CommandId = proto.RD.BaseHD.CommandId
		msg := heartHdr.Encode()
		_, err := pu.connSocket.Write(msg.Bytes())

		if err != nil {
			pu.handleError(base.DTerror{"Send Error " + err.Error()})

		}
		return
	}
	msg := proto.EncodeHdr()
	_, err := pu.connSocket.Write(msg.Bytes())
	if err != nil {
		pu.handleError(base.DTerror{"Send Error " + err.Error()})
		return
	}
	_, dErr := pu.connSocket.Write(proto.BD.Data)
	if dErr != nil {
		pu.handleError(base.DTerror{"Send Error" + err.Error()})
		return
	}

}

func (pu *PUDevice) handleSource(clientID uint32) {

	defer func() {
		if r := recover(); r != nil {
			pu.handleError(base.DTerror{"handleSource Panic"})
		}
	}()
	file := new(record.RecordFile)
	resourceFile := new(bytes.Buffer)
	resourceFile.WriteString(oneEnvParam.ResourceDir)
	resourceFile.WriteString("\\")
	resourceFile.WriteString(oneEnvParam.ResourceFile)
	fmt.Println("Open File ", resourceFile)
	if b := file.Open("C:\\1.hmv"); !b {
		return
	}
	defer file.Close()
	file.GetHeader()
	for {
		if !atomic.CompareAndSwapUint32(&pu.SendFlag, 1, 1) {
			break
		}
		if atomic.CompareAndSwapUint32(&pu.Valid, 0, 0) {
			break
		}

		proto := new(base.Proto)
		proto.RD.BaseHD.CommandId = 0x80 | base.DATA_STREAM
		proto.RD.HD.TransferType = base.TRANSFER_RAW
		proto.RD.HD.ContextType = base.CONTEXT_VIDEO
		proto.RD.HD.ClientIdent = clientID

		b := file.GetNextFrame()
		if b == nil {
			file.Seek()
			continue
		}
		proto.EncodeBody(b)
		pu.rwChan <- proto

		stat.GetLocalStatistInst().SendData(uint64(len(proto.BD.Data)))

		time.Sleep(100 * time.Millisecond)

	}
}

func (pu *PUDevice) openSource(clientID uint32) error {

	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = 0x80 | base.OPEN_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID
	proto.RD.HD.TransferType = base.TRANSFER_RAW
	proto.RD.HD.ContextType = base.CONTEXT_JSON

	//resourceJson := command.ResourceDes{id}

	//res, e := json.Marshal(resourceJson)
	//if e != nil {
	//	return e
	//}
	atomic.CompareAndSwapUint32(&pu.SendFlag, 0, 1)

	responseJson := base.ResponseJson{"ns", "open", "", "ok"}
	b, err := json.Marshal(responseJson)
	if err != nil {
		pu.handleError(err)
		return err
	}

	proto.EncodeBody(b)

	pu.rwChan <- proto

	go pu.handleSource(clientID)
	stat.GetLocalStatistInst().OpenRes()
	return err
}

func (pu *PUDevice) closeSource(clientID uint32) error {
	defer func() {
		if r := recover(); r != nil {
			pu.handleError(base.DTerror{"closeSource Panic"})
		}
	}()
	atomic.CompareAndSwapUint32(&pu.SendFlag, 1, 0)

	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = 0x80 | base.CLOSE_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID
	proto.RD.HD.TransferType = base.TRANSFER_RAW
	proto.RD.HD.ContextType = base.CONTEXT_JSON
	responseJson := base.ResponseJson{"ns", "close", "", "ok"}
	b, err := json.Marshal(responseJson)
	if err != nil {
		pu.handleError(err)
		return err
	}

	proto.EncodeBody(b)
	pu.rwChan <- proto

	stat.GetLocalStatistInst().CloseRes()
	return err
}

func (pu *PUDevice) run(sn string) {
	pu.SN = sn
	connSocket, err := net.Dial("tcp", oneEnvParam.Url)
	if err != nil {
		fmt.Println("Connect ", err)
		go pu.ReRun(sn)
		return
	}
	pu.rwChan = make(chan *base.Proto, 24)

	stat.GetLocalStatistInst().On()

	pu.SendFlag = 0
	pu.Valid = 1
	pu.connSocket = connSocket
	go pu.Register()
	go pu.handleReadChnTask()
	go pu.readTask()

}

func (pu *PUDevice) ReRun(sn string) {
	if oneEnvParam.TimeOut < 5 {
		oneEnvParam.TimeOut = 5
	}
	var duration time.Duration = time.Duration(oneEnvParam.TimeOut)
	time.Sleep(duration * time.Second)
	pu.run(sn)
}

func (pu *PUDevice) Register() {
	// 注册资源
	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = base.REGISTER_RESOURCE
	proto.RD.HD.TransferType = base.TRANSFER_RAW
	proto.RD.HD.ContextType = base.CONTEXT_JSON

	regsterJson := base.RegisterResParamJson{pu.SN, 1, false}
	requestJson := base.RequestJson{"", "ns", "register", regsterJson}

	b, err := json.Marshal(requestJson)
	if err != nil {
		pu.handleError(err)
		return
	}

	proto.EncodeBody(b)
	pu.rwChan <- proto

}

func crc16(buf []byte, len int) uint16 {

	var crc uint16 = 0xffff
	for i := 0; i < len; i++ {
		c := uint16(buf[i]) & 0x00ff
		crc ^= c
		for j := 0; j < 8; j++ {
			if crc&0x0001 == 1 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func testCrc() {
	var buf [6]byte
	buf[0] = 0x01
	buf[1] = 0x04
	buf[2] = 0x00
	buf[3] = 0x50
	buf[4] = 0x00
	buf[5] = 0x02
	fmt.Printf("%x", crc16(buf[:], 6))
}

func parseArg() {
	oneEnvParam = new(EnvParam)
	flag.StringVar(&oneEnvParam.ResourceDir, "D", ".", "Resource Dir")
	flag.StringVar(&oneEnvParam.ResourceFile, "F", "1.hmv", "Resource File")
	flag.UintVar(&oneEnvParam.BeginID, "B", 1, "Resource ID Begin")
	flag.UintVar(&oneEnvParam.ResourceCount, "C", 10, "Resource ID Count")
	flag.IntVar(&oneEnvParam.TimeOut, "T", 10, "Reconnect Time")
	flag.BoolVar(&oneEnvParam.SupportStat, "S", false, "Support Statist")
	flag.StringVar(&oneEnvParam.Url, "I", "localhost:9999", "DT URL CONFIG")

	flag.Parse()

	fmt.Printf("res path	: %s\n", oneEnvParam.ResourceDir)
	fmt.Printf("res file 	: %s\n", oneEnvParam.ResourceFile)
	fmt.Printf("res begin   : %v\n", oneEnvParam.BeginID)
	fmt.Printf("res count	: %v\n", oneEnvParam.ResourceCount)
	fmt.Printf("url addr	: %v\n", oneEnvParam.Url)
	fmt.Printf("-------------------------------------------------------\n")

}

func testSelect() {
	chn := make(chan int, 1)
	for {

		select {
		case <-chn:
			fmt.Println("recv msg")
		case <-time.After(10 * time.Second):
			fmt.Println("Timeout")

		}
	}
}

func main() {

	// 减少内存分配

	parseArg()
	devCount := oneEnvParam.ResourceCount
	stat.GetLocalStatistInst().Init("pu_llog", "logic.dat", int32(devCount))
	go stat.GetLocalStatistInst().Start()
	mylog.GetErrorLogger().Init("pu_elog", "error.log")

	go stat.StartMonitorTask("pu_mlog", "monitor.dat")

	for i := oneEnvParam.BeginID; i <= devCount; i++ {

		sn := new(bytes.Buffer)
		fmt.Fprintf(sn, "%v", strconv.Itoa(int(i)))
		var pu *PUDevice = new(PUDevice)
		pu.run(sn.String())
	}

	for {
		time.Sleep(10 * time.Minute)
	}

}
