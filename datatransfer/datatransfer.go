// main.go
/* 开发原则
资源：资源ID唯一性，资源具有属性描述
	 录像文件看作是时间资源tmpBaseHdSlice
数据传输服务看做是资源的容器，以资源作为管理对象，对于数据传输服务来说，不关心设备


*/

package main

import (
	"fmt"
	"utility/base"

	"datatransfer/server"
	"math/rand"
	"runtime"
	"utility/mylog"
	"utility/stat"

	"time"
)

type GcTest struct {
	back []int
}

func poolTest() {

	var size = 128
	var Alloc = 1024 * 1024
	gcTest := make([]GcTest, 819200)
	var total uint64 = 0
	for i := 1; i < 1024000; i++ {
		fmt.Println()
		for k, v := range gcTest {
			size = (rand.Int()%(Alloc) + Alloc)

			v.back = make([]int, size)
			v.back[size-100] = 100

			for kk, _ := range v.back {
				if kk%3000 == 0 {
					v.back[kk] = rand.Int()
					fmt.Println(v.back[kk])
				}

				//fmt.Println(kk)
				//time.Sleep(1 * time.Microsecond)
			}

			total += uint64(size)
			if k%1000 == 0 {
				fmt.Println(size/Alloc, total/uint64(Alloc))
			}

		}

		for _, v := range gcTest {
			for _, vv := range v.back {
				fmt.Println(vv)
			}
		}

		//time.Sleep(2 * time.Millisecond)

	}

}

func print_mem() {
	for {
		var runMem runtime.MemStats
		runtime.ReadMemStats(&runMem)
		fmt.Println(runMem)
		time.Sleep(2 * time.Second)
	}

}

func main() {

	fmt.Println("DataTransfer Running!")

	server.ParseArg()

	right := base.VIDEO_RIGHT | base.AUDIO_RIGHT
	fmt.Println(right)
	var PKS1024 string = "s5DrLk2RE355BcO7FZ49xNdfAQtwKsMaEJa8yOnX7IjkRiOnmOoXCiCsRl0vpe23eZ7SWUW47DvQ1UxEpkGsFv/LOeOxMh06oXeH0zqlRPCw074q0s+IfRtcbdqGLvzvCz8ZAaoQrEIfwq2hm+7ueWKjWecqqQlQ5cu+dg3gd80="
	var E string = "AQAB"
	base.InitRsaBig(PKS1024, E)

	stat.GetLocalStatistInst().Init("dt_llog", "logic.dat", 0)
	mylog.GetErrorLogger().Init("dt_elog", "error.log")
	go stat.StartMonitorTask("dt_mlog", "monitor.dat")
	go stat.GetLocalStatistInst().Start()
	runtime.GOMAXPROCS(runtime.NumCPU())
	server.Run()

	exitChn := make(chan bool, 1)
	exit := <-exitChn
	fmt.Println("DataTransfer Running Over!", exit)
}
