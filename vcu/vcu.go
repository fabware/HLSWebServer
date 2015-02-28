// client.go
package main

// 心跳，双向产生，需要了解状态的都可以通过发送心跳，心跳接受方有义务回复心跳
import (
	"bytes"

	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	//"unsafe"
	"utility/mylog"
	//"plat"

	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
	"utility/base"
	"utility/stat"
)

type CUClient struct {
	connSocket   net.Conn
	ffmpegSocket net.Conn

	sn       string
	clientID uint32
	Valid    uint32
	heartChn chan bool
	token    string

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
	fmt.Println(err)

	if atomic.CompareAndSwapUint32(&cu.Valid, 0, 0) {
		return
	}
	stat.GetLocalStatistInst().Off()
	atomic.CompareAndSwapUint32(&cu.Valid, 1, 0)
	cu.connSocket.Close()
	close(cu.heartChn)

	go cu.ReRun()
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

func (cu *CUClient) readTask() {

	for {
		proto := new(base.Proto)

		err := proto.ReadBinaryProto(cu.connSocket)
		if err != nil {
			cu.handleError(err)
			break
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
			stat.GetLocalStatistInst().RecvData(uint64(proto.RD.HD.BodyLen))
			//timeStampBuf := bytes.NewBuffer(proto.BD.Data[4:12])
			//var timeStamp int64 = 0
			//binary.Read(timeStampBuf, binary.LittleEndian, &timeStamp)
			//timeNow := time.Now().UnixNano()
			//delay := timeNow - timeStamp
			var frameType uint16 = 0

			var err error = nil
			err = binary.Read(bytes.NewBuffer(proto.BD.Data[10:12]), binary.LittleEndian, &frameType)
			if err != nil {
				fmt.Println(err)
			}

			if cu.ffmpegSocket != nil {
				cu.ffmpegSocket.Write(proto.BD.Data[20:])
			}
			cu.hlsHandler.goRawH264Data2Ts(frameType, proto.BD.Data[20:])

		} else if cmdID == base.OPEN_RESOURCE_CMD {
			var responseJson base.ResponseJson
			fmt.Println("Open Response :", string(proto.BD.Data[4:]))
			unerr := json.Unmarshal(proto.BD.Data[4:], &responseJson)
			if unerr != nil {
				cu.handleError(unerr)
				continue
			}

			if !bytes.Equal([]byte("ok"), []byte(responseJson.Error)) {

				cu.handleError(unerr)

				continue
			}
			stat.GetLocalStatistInst().OpenRes()
		} else if cmdID == base.CLOSE_RESOURCE_CMD {
			var responseJson base.ResponseJson
			stat.GetLocalStatistInst().CloseRes()
			fmt.Println("Close Response :", string(proto.BD.Data[4:]))
			unerr := json.Unmarshal(proto.BD.Data[4:], &responseJson)
			if unerr != nil {
				cu.handleError(unerr)
				continue
			}

		} else if cmdID == base.LEAVE_NOTIFY {
			atomic.AddUint32(&cu.clientID, 1)
			stat.GetLocalStatistInst().CloseRes()
			go cu.openSource(cu.clientID, cu.sn)
			continue
		}

	}
}

func help() {
	str := `
			h : Print Help Info
			o : Open  Resource
			c : Close Resource
			`
	fmt.Println(str)
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

func (cu *CUClient) consoleTask() {
	fmt.Println("Client Console GUI Task Running!")
	help()

	for {
		var cmd byte
		fmt.Println("Please Your Select : ")
		fmt.Scanf("%c\n", &cmd)
		fmt.Println("Your Select Is: ", cmd)
		switch cmd {
		case 'o':

			var sourceID uint32 = 0
			fmt.Println("Please Input Your Resource ")
			fmt.Scanf("%d\n", &sourceID)
			sourceSN := new(bytes.Buffer)
			fmt.Fprintf(sourceSN, "%v", sourceID)
			fmt.Println("Open Source : ", sourceSN)
			atomic.AddUint32(&cu.clientID, 1)
			go cu.openSource(cu.clientID, sourceSN.String())
		case 'c':
			var sourceID uint32 = 0
			fmt.Println("Please Input Your Resource ")
			fmt.Scanf("%d\n", &sourceID)
			sourceSN := new(bytes.Buffer)
			fmt.Fprintf(sourceSN, "%v", sourceID)
			fmt.Println("Open Source : ", sourceSN)
			go cu.closeSource(cu.clientID, sourceSN.String())
		case 'h':
			help()
		default:
			fmt.Println("Not Support Cmd")
			help()
		}

	}
}

func (cu *CUClient) ReRun() {
	if oneEnvParam.TimeOut < 10 {
		oneEnvParam.TimeOut = 10
	}
	var duration time.Duration = time.Duration(oneEnvParam.TimeOut)
	time.Sleep(duration * time.Second)
	cu.run()
}

func (cu *CUClient) run() {

	fmt.Println(oneEnvParam.url)
	connSocket, err := net.DialTimeout("tcp", oneEnvParam.url, 2*time.Second)

	if err != nil {
		fmt.Println(err)
		go cu.run()
		return
	}

	stat.GetLocalStatistInst().On()
	cu.hlsHandler = new(RawData2Hls)
	cu.hlsHandler.Init()

	cu.connSocket = connSocket
	atomic.CompareAndSwapUint32(&cu.Valid, 0, 1)

	cu.heartChn = make(chan bool, 1)

	go cu.readTask()
	atomic.AddUint32(&cu.clientID, 1)
	if oneEnvParam.auto {
		go cu.openSource(cu.clientID, cu.sn)
	} else {
		go cu.consoleTask()
	}
}

func parseArg() {
	oneEnvParam = new(EnvParam)
	//flag.StringVar(&oneEnvParam.ResourceDir, "D", ".", "Resource Dir")
	//flag.StringVar(&oneEnvParam.ResourceFile, "F", "1.hmv", "Resource File")
	flag.UintVar(&oneEnvParam.BeginID, "B", 1, "Resource ID Begin")
	flag.UintVar(&oneEnvParam.ResourceCount, "C", 1, "Resource ID Count")
	flag.IntVar(&oneEnvParam.TimeOut, "T", 10, "Reconnect Time")
	flag.BoolVar(&oneEnvParam.SupportStat, "S", false, "Support Statist")
	flag.StringVar(&oneEnvParam.url, "I", "localhost:9999", "DT IP")
	flag.BoolVar(&oneEnvParam.auto, "A", false, "AUTO RUN")
	flag.Parse()

	//fmt.Printf("res path	: %s\n", oneEnvParam.ResourceDir)
	fmt.Printf("AUTO 	: %v\n", oneEnvParam.auto)
	fmt.Printf("res begin   : %v\n", oneEnvParam.BeginID)
	fmt.Printf("res count	: %v\n", oneEnvParam.ResourceCount)
	fmt.Printf("-------------------------------------------------------\n")
}

func runTcp() error {

	lnSocket, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		mylog.GetErrorLogger().Println(err)
		return err
	}

	defer lnSocket.Close()
	for {

		connSocket, err := lnSocket.Accept()
		if err != nil {
			mylog.GetErrorLogger().Println(err)
			return err
		}
		fmt.Println("Recv Connect", connSocket.LocalAddr().String())
		var cu *CUClient = &CUClient{sn: "1", token: ""}
		cu.ffmpegSocket = connSocket
		go cu.run()
	}

	return nil
}

func ffmpegCmmand() {
	cmd := exec.Command("hls.bat")

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Start()
	if err != nil {
		fmt.Println("handleHttpPost ", err)
	}

	/*out, err := exec.Command("date").Output()
	if err != nil {
		fmt.Print(err)
	}
	fmt.Printf("The date is %s\n", out)*/

}
func handleHttpPost(rw http.ResponseWriter, rq *http.Request) {

	go ffmpegCmmand()
	fmt.Println(rq.Header)
	rw.Write([]byte("No,Sorry, Method Not Exist"))
}

func runHttp(port int) error {

	portStr := new(bytes.Buffer)
	portStr.WriteByte(':')
	portStr.WriteString(strconv.Itoa(int(port)))

	http.HandleFunc("/GET", handleHttpPost)
	e := http.ListenAndServe(portStr.String(), nil)
	if e != nil {
		return e
	}

	return nil
}

func main() {

	go func() {
		http.ListenAndServe("localhost:6001", nil)
	}()
	parseArg()
	go runHttp(9998) // 负责监听web服务器请求资源
	go runTcp()      // 负责数据代理

	//platform := &plat.Platform{Url: "https://192.168.20.194:44312"}
	//platform.Register(plat.RegisterInfo{"shaoshengr1", "shaosheng123", "shaosheng123"})
	//code, err := platform.Login(plat.LoginInfo{"shaoshengr1", "shaosheng123", true})
	//if err != nil {
	//fmt.Println(err)
	//return
	//}
	//fmt.Println("Login", code)

	devCount := oneEnvParam.ResourceCount
	stat.GetLocalStatistInst().Init("cu_llog", "logic.dat", int32(devCount))
	mylog.GetErrorLogger().Init("cu_elog", "error.log")
	go stat.GetLocalStatistInst().Start()
	go stat.StartMonitorTask("cu_mlog", "monitor.dat")
	for {
		time.Sleep(10 * time.Second)
	}

	for i := oneEnvParam.BeginID; i <= oneEnvParam.BeginID+devCount-1; i++ {

		sn := new(bytes.Buffer)
		fmt.Fprintf(sn, "%v", strconv.Itoa(int(i)))
		/*var rightValue [1]byte
		rightValue[0] = base.AUDIO_RIGHT | base.VIDEO_RIGHT
		var resToken plat.ResponseToken
		err := platform.GetToken(plat.TokenInfo{sn.String(), rightValue[:]},
			&resToken)
		if err != nil {
			mylog.GetErrorLogger().Println(err)
			continue
		}

		var cu *CUClient = &CUClient{sn: sn.String(), token: resToken.AccessToken}*/
		var cu *CUClient = &CUClient{sn: sn.String(), token: ""}
		go cu.run()
	}
	for {
		time.Sleep(10 * time.Second)
	}
}
