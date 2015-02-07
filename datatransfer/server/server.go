// server.go
/*
参数初始化：
	1. 读取配置文件
	2. 传入参数
	3. 通过socket传输
	4. 并发accept
*/
package server

import (
	"bytes"
	"datatransfer/resource"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"utility/mylog"
)

// 读取配置

type EnvParam struct {
	SupportStat bool // 支持统计
	url         string
}

var oneEnvParam *EnvParam = nil

func ParseArg() {
	oneEnvParam = new(EnvParam)
	flag.BoolVar(&oneEnvParam.SupportStat, "S", false, "Support Statist")
	flag.StringVar(&oneEnvParam.url, "I", "localhost:9999", "DT IP")
	flag.Parse()
	fmt.Printf("DT Server IP 	: %v\n", oneEnvParam.url)
	fmt.Printf("-------------------------------------------------------\n")
}

func runTcp() error {

	lnSocket, err := net.Listen("tcp", oneEnvParam.url)
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
		mylog.GetErrorLogger().Println("Remote Connect Addr : ", connSocket.RemoteAddr().String())

		go handleConnect(connSocket)
	}

	return nil
}

func runHttp(port int) error {

	portStr := new(bytes.Buffer)
	portStr.WriteByte(':')
	portStr.WriteString(strconv.Itoa(int(port)))

	http.HandleFunc("/POST", handleHttpPost)
	e := http.ListenAndServe(portStr.String(), nil)
	if e != nil {
		return e
	}

	return nil
}

func handleHttpPost(rw http.ResponseWriter, rq *http.Request) {
	fmt.Println(rq.Header)
	rw.Write([]byte("No,Sorry, Method Not Exist"))
}

func handleConnect(connSocket net.Conn) {

	chn := new(resource.Channel)
	chn.Run(connSocket)

}

func Run() error {

	go runTcp()
	go runHttp(10000)
	return nil
}
