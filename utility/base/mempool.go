package base

import (
	"sync"
)

type memModel struct {
	low int
	mem [][]byte
}

type memPool struct {
	mem   [5]memModel
	mutex sync.Mutex
}

var oneMemPool *memPool = nil

func GetMem(len int) []byte {
	oneMemPool.mutex.Lock()
	defer oneMemPool.mutex.Unlock()
	if oneMemPool == nil {
		oneMemPool = new(memPool)

		for k, v := range oneMemPool.mem {
			v.low = (2 ^ k) * 4096
		}
	}

	//if len
	return oneMemPool.mem[1].mem[1]

}

func PutMem([]byte) {

}
