// hls.go
package main

/*
#cgo linux CFLAGS: -DLINUX=1
#cgo LDFLAGS: -lTsMux  -L.

void* createH264MuxTs();
void setTsDataCB(void* inst, void* f);
int rawH264Data2Ts(void* inst, void* user_data,void* data, unsigned int len);
void releaseH264MuxTs(void* inst);

extern void goDataTsCallBack();

void setCB(void* handle)
{
    setTsDataCB(handle,goDataTsCallBack);
}

*/
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"time"
	"unsafe"
)

const (
	TSFILEMAX_SIZE = 6 * 1024 * 1024
)

type hlsFile struct {
	fileHandler *os.File
	fileSize    uint32
}

func (thsi *hlsFile) generateFileName() string {

	filename := new(bytes.Buffer)
	now := time.Now()
	fmt.Fprintf(filename, "%04d_%02d_%2d_%2d_%2d_%2d.ts",
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second())
	return filename.String()
}
func (this *hlsFile) write(data []byte) {
	if this.fileHandler == nil {
		var err error
		this.fileHandler, err = os.OpenFile(this.generateFileName(), os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			fmt.Println("write", err)
			return
		}
	}
	this.fileHandler.Write(data)
	this.fileSize += uint32(len(data))
	if this.fileSize >= TSFILEMAX_SIZE {
		this.fileHandler.Close()
		this.fileHandler = nil
		this.fileSize = 0
	}
}

/*
 1. 负责接收原始H264数据
 2. 负责把H264数据转换为ts文件
 3. 负责生成m3u8文件
 4. 支持把相关文件存储在本地磁盘，通过web服务器访问
     也支持直接把相关文件存储NoSql数据库中，目前考虑的数据库：
	 mongdb，Riak
*/
type RawData2Hls struct {
	fileHandler hlsFile
	muxTsHandle unsafe.Pointer
}

func (this *RawData2Hls) Init() {
	this.muxTsHandle = C.createH264MuxTs()
	C.setCB(this.muxTsHandle)
}

func (this *RawData2Hls) Uninit() {
	C.releaseH264MuxTs(this.muxTsHandle)
}

func (this *RawData2Hls) goRawH264Data2Ts(data []byte) {
	C.rawH264Data2Ts(this.muxTsHandle, unsafe.Pointer(this), unsafe.Pointer(&data[0]), C.uint(len(data)))
}
