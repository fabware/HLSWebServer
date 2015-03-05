// server.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"utility/mylog"
)

type hlsProxyPool struct {
	hlsProxy map[string]*CUClient
	mutex    sync.Mutex
}

var oneProxyPool *hlsProxyPool = nil
var oneProxyPoolOnce sync.Once

func initPool() {
	oneProxyPool = new(hlsProxyPool)
	oneProxyPool.hlsProxy = make(map[string]*CUClient)

}

func proxyPoolInst() *hlsProxyPool {
	oneProxyPoolOnce.Do(initPool)
	return oneProxyPool
}

func CreateProxy(id string) *CUClient {
	proxyPoolInst().mutex.Lock()
	defer proxyPoolInst().mutex.Unlock()

	proxy := &CUClient{}

	proxy.token = ""
	proxy.ffmpegChn = nil
	proxy.debugChn = nil
	proxy.hlsHandler = nil
	proxy.sn = id
	proxy.frameCount = 0
	proxyPoolInst().hlsProxy[id] = proxy

	return proxy
}

func BindFfmpegForProxy(chn net.Conn) *CUClient {
	proxyPoolInst().mutex.Lock()
	defer proxyPoolInst().mutex.Unlock()

	for _, v := range proxyPoolInst().hlsProxy {
		if v.ffmpegChn == nil {
			v.ffmpegChn = chn
			return v
		}
	}
	return nil
}

func CheckResourceIsExist(id string) bool {
	proxyPoolInst().mutex.Lock()
	defer proxyPoolInst().mutex.Unlock()

	_, ok := proxyPoolInst().hlsProxy[id]
	if ok {
		return true
	}
	return false

}

func ReleaseProxy(proxy *CUClient) error {

	proxyPoolInst().mutex.Lock()
	defer proxyPoolInst().mutex.Unlock()
	delete(proxyPoolInst().hlsProxy, proxy.sn)

	return nil
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
		//fileHandler, _ := os.OpenFile("my1.h264", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0777)
		proxy := BindFfmpegForProxy(connSocket)
		if proxy != nil {
			proxy.run()
		}
	}
	return nil
}

func ffmpegCmmand(id string) {
	cmd := exec.Command("hls.bat", id)

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

	method := rq.FormValue("method")
	id := rq.FormValue("resourceid")
	if CheckResourceIsExist(id) {
		return
	}
	if CheckResourceIsExist(id) {
		rw.Write([]byte("No,Sorry, Video Has Open"))
		return
	}

	proxy := CreateProxy(id)
	if method == "ffm" {
		os.Remove(id + ".m3u8")
		go ffmpegCmmand(id)
	} else if method == "dll" {
		proxy.hlsHandler = new(RawData2Hls)
		proxy.hlsHandler.Init(id)
		proxy.run()
	} else if method == "debug" {
		fileHandler, _ := os.OpenFile(id+".h264", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0777)
		proxy.debugChn = fileHandler
		proxy.run()
	}
	rw.Write([]byte("No,Sorry, Method Not Exist"))
}

func runHttp(port int) error {

	portStr := new(bytes.Buffer)
	portStr.WriteByte(':')
	portStr.WriteString(strconv.Itoa(int(port)))

	http.HandleFunc("/open/", handleHttpPost)
	e := http.ListenAndServe(portStr.String(), nil)
	if e != nil {
		return e
	}
	return nil
}

func main() {

	parseArg()
	go runHttp(9998) // 负责监听web服务器请求资源
	go runTcp()      // 负责数据代理

	exitChn := make(chan bool)
	<-exitChn
}
