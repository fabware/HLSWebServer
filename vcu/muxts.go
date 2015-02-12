package main

import (
	"unsafe"
)

import "C"

//export goDataTsCallBack
func goDataTsCallBack(userData unsafe.Pointer,
	data unsafe.Pointer, len uint32) {
	dst := make([]byte, len)

	var offset uintptr = uintptr(data)
	for i := uint32(0); i < len; i++ {
		var pData *uint8 = (*uint8)(unsafe.Pointer(offset))
		dst[i] = *pData
		offset++
	}
	(*RawData2Hls)(userData).fileHandler.write(dst)
	//TsFileHandler.Write(dst)

}
