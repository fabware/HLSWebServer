package stat

import (
	"bytes"
	"fmt"
	//"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"utility/mylog"

	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

const (
	M1 = 1024 * 1024
)

type bandWidth struct {
	lastStamp int64
	bandwidth float64

	totalDataLen int64
	mutex        sync.RWMutex
}

func (this *bandWidth) AccDataLen(len int64) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.totalDataLen += len
}

func (this *bandWidth) getBandWidth() float64 {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	curStamp := time.Now().Unix()
	diff := curStamp - this.lastStamp
	if diff >= 5 {
		this.lastStamp = curStamp
		this.bandwidth = float64(this.totalDataLen) / float64(diff)
		this.totalDataLen = 0
	}
	return this.bandwidth
}

type networkDelay struct {
	delay float64

	totalDelay int64
	accCount   int64
	mutex      sync.Mutex
}

func (this *networkDelay) AccDelay(delay int64) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.totalDelay += delay
	this.accCount++
}

func (this *networkDelay) getDelay() float64 {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	this.delay = float64(this.totalDelay) / float64(this.accCount)
	this.totalDelay = 0

	return this.delay
}

// 业务统计
type localStatist struct {
	ResOnCount      int32
	TotalRes        int32
	LeaveTotalCount int32

	OpenResCount int32

	SendDataLen uint64
	RecvDataLen uint64
	SendBand    *bandWidth
	RecvBand    *bandWidth

	Delay *networkDelay

	RegisterResCount int32
}

func (stat *localStatist) Init(dir string, filename string, total int32) {
	mylog.GetLocalLogger().Init(dir, filename)
	stat.TotalRes = total

	stat.SendDataLen = 0
	stat.RecvDataLen = 0
	stat.SendBand = new(bandWidth)
	stat.RecvBand = new(bandWidth)

	stat.Delay = new(networkDelay)

}

func (stat *localStatist) On() {
	atomic.AddInt32(&stat.ResOnCount, 1)
}

func (stat *localStatist) Off() {

	atomic.AddInt32(&stat.LeaveTotalCount, 1)
	atomic.AddInt32(&stat.ResOnCount, -1)
}

func (stat *localStatist) OpenRes() {
	atomic.AddInt32(&stat.OpenResCount, 1)
}

func (stat *localStatist) CloseRes() {
	atomic.AddInt32(&stat.OpenResCount, -1)
}

func (stat *localStatist) SendData(len uint64) {
	stat.SendBand.AccDataLen(int64(len))
	atomic.AddUint64(&stat.SendDataLen, len)
}

func (stat *localStatist) RecvData(len uint64) {
	stat.RecvBand.AccDataLen(int64(len))
	atomic.AddUint64(&stat.RecvDataLen, len)
}

func (stat *localStatist) DelayValue(delay int64) {
	stat.Delay.AccDelay(delay)
}

func (stat *localStatist) RegisterRes() {
	atomic.AddInt32(&stat.RegisterResCount, 1)

}

func (stat *localStatist) UnRegisterRes() {
	atomic.AddInt32(&stat.RegisterResCount, -1)

}

func (stat *localStatist) Start() {
	//On : %v, Total : %v, TotalLeave : %v, Open : %v, ResData : %vk
	time.Sleep(10 * time.Second)
	for {
		logBuffer := new(bytes.Buffer)
		var gcStatus debug.GCStats
		debug.ReadGCStats(&gcStatus)
		if len(gcStatus.Pause) > 0 {
			fmt.Printf("******GCTime:(%v:%v),GCNum:%v, GCPause:%v,GCPT:%v******\n",
				gcStatus.LastGC.Minute(),
				gcStatus.LastGC.Second(),
				gcStatus.NumGC,
				gcStatus.Pause[0],
				gcStatus.PauseTotal.Seconds())
		}
		time.Sleep(2 * time.Second)
		fmt.Fprintf(logBuffer, "%v %v %v %v %v %v\n",
			atomic.AddInt32(&stat.ResOnCount, 0),
			atomic.AddInt32(&stat.TotalRes, 0),
			atomic.AddInt32(&stat.LeaveTotalCount, 0),
			atomic.AddInt32(&stat.OpenResCount, 0),
			atomic.AddUint64(&stat.RecvDataLen, 0)/M1,
			atomic.AddUint64(&stat.SendDataLen, 0)/M1)

		mylog.GetLocalLogger().Write(logBuffer.String())

		consoleBuffer := new(bytes.Buffer)
		fmt.Fprintf(consoleBuffer, "On:%v,Total:%v,TotalLeave:%v,Open:%v,RecvBand:%.2fmps,SendBand:%.2fmps,Delay:%.2fms\n",
			atomic.AddInt32(&stat.ResOnCount, 0),
			atomic.AddInt32(&stat.TotalRes, 0),
			atomic.AddInt32(&stat.LeaveTotalCount, 0),
			atomic.AddInt32(&stat.OpenResCount, 0),
			(stat.RecvBand.getBandWidth()*8)/M1,
			(stat.SendBand.getBandWidth()*8)/M1,
			(stat.Delay.getDelay()/1000000)/float64(stat.ResOnCount))
		curTime := time.Now()
		curBuffer := new(bytes.Buffer)
		fmt.Fprintf(curBuffer, "%02d:%02d:%02d",
			curTime.Hour(),
			curTime.Minute(),
			curTime.Second())
		fmt.Println(curBuffer.String(), consoleBuffer.String())
		time.Sleep(10 * time.Second)
		debug.FreeOSMemory()
	}

}

var oneLocalStatistInst *localStatist = nil

func GetLocalStatistInst() *localStatist {
	if oneLocalStatistInst == nil {

		oneLocalStatistInst = new(localStatist)
	}

	return oneLocalStatistInst
}

// 监控统计

func StartMonitorTask(dir string, filename string) {

	mylog.GetMonitorLogger().Init(dir, filename)

	mylog.GetMonitorLogger().Write("Time  UsedPercent HeapAlloc HeapObjects Alloc NumGC  NumGor\n")
	// alloc, frees
	// 内存

	for {
		logBuffer := new(bytes.Buffer)

		var runMem runtime.MemStats
		runtime.ReadMemStats(&runMem)
		memStat, _ := mem.VirtualMemory()

		fmt.Fprintf(logBuffer, "%v %v %v %v %v %v\n",
			memStat.UsedPercent,
			runMem.HeapAlloc/(M1),
			runMem.HeapObjects,
			runMem.Alloc/(M1),
			runMem.NumGC,
			runtime.NumGoroutine())

		mylog.GetMonitorLogger().Write(logBuffer.String())
		time.Sleep(10 * time.Second)

	}

}
