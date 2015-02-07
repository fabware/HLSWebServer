package mylog

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"time"
)

/*
	1. 错误日志， 		error.log
	2. 系统统计日志，		monitor.csv
	3. 业务统计日志，		logic.csv

	使用gnuplot把csv日志数据分解绘图
*/

func createFile(dir string, name string) *bytes.Buffer {
	os.Mkdir(dir, 0777)
	os.Chdir(dir)
	Dir, err := os.Getwd()
	if err != nil {
		fmt.Println("InitLog", err)
		return nil
	}

	timeNow := new(bytes.Buffer)
	fmt.Fprintf(timeNow, "%04v%02v%02v_%02v%02v%02v_",
		time.Now().Year(),
		time.Now().Month(),
		time.Now().Day(),
		time.Now().Hour(),
		time.Now().Minute(),
		time.Now().Second())

	logFileName := new(bytes.Buffer)
	logFileName.WriteString(Dir)

	logFileName.WriteString("\\")
	logFileName.Write(timeNow.Bytes())
	logFileName.WriteString(name)
	fmt.Println(Dir)
	os.Chdir("..")
	return logFileName

}

// 错误日志

type errorLogger struct {
	//fileHandler *os.File
	logger *log.Logger
	log.Logger
}

var oneErrorLogger *errorLogger = nil

func (logg *errorLogger) Init(dir string, filename string) {

	logFileName := createFile(dir, filename)

	fileHandler, err := os.OpenFile(logFileName.String(),
		os.O_RDWR|os.O_CREATE,
		0)
	if err != nil {
		fmt.Printf("%s\r\n", err.Error())
		os.Exit(-1)
	}
	logg.logger = log.New(fileHandler, "\r\n", log.Ldate|log.Ltime)
}

func GetErrorLogger() *errorLogger {
	if oneErrorLogger == nil {
		oneErrorLogger = new(errorLogger)
	}
	return oneErrorLogger
}

func (logg *errorLogger) Printf(format string, v ...interface{}) {
	logg.logger.Printf(format, v)
}

func (logg *errorLogger) Println(v ...interface{}) {
	logg.logger.Println(v)
}

type LogFileLogger struct {
	logFileHandler *os.File
}

func (logger *LogFileLogger) Init(dir string, fileName string) {

	logFileName := createFile(dir, fileName)
	logfile, err := os.OpenFile(logFileName.String(), os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		fmt.Printf("%s\r\n", err.Error())
		os.Exit(-1)
	}
	logger.logFileHandler = logfile
}

func (logger *LogFileLogger) WriteHDR(hdr string) {
	buf := new(bytes.Buffer)
	buf.WriteString("#")
	buf.WriteString(hdr)
	logger.logFileHandler.Write(buf.Bytes())
}

func (logger *LogFileLogger) Write(log string) {

	curTime := time.Now()
	curBuffer := new(bytes.Buffer)
	fmt.Fprintf(curBuffer, "%v/%v/%v/%v",
		curTime.Day(),
		curTime.Hour(),
		curTime.Minute(),
		curTime.Second())

	logger.logFileHandler.WriteString(curBuffer.String())
	logger.logFileHandler.WriteString(" ")
	logger.logFileHandler.WriteString(log)

}

func (logger *LogFileLogger) Close() {
	logger.logFileHandler.Close()
}

// 系统日志
var oneMonitorLogger *LogFileLogger = nil

func GetMonitorLogger() *LogFileLogger {
	if oneMonitorLogger == nil {
		oneMonitorLogger = new(LogFileLogger)
	}
	return oneMonitorLogger
}

// 业务日志
var oneLocalLogger *LogFileLogger = nil

func GetLocalLogger() *LogFileLogger {
	if oneLocalLogger == nil {
		oneLocalLogger = new(LogFileLogger)

	}
	return oneLocalLogger
}
