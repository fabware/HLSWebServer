// queue
package base

import (
	"sync"
)

type Queue struct {
	data    []interface{}
	Compare func(e1 interface{}, e2 interface{}) bool
	mutex   sync.RWMutex
}

func (this *Queue) Append(elem interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.data = append(this.data, elem)
}

func (this *Queue) Delete(elem []interface{}) {

	this.mutex.Lock()
	defer this.mutex.Unlock()

	tmpSlice := make([]interface{}, 0, len(this.data))
	for _, tmpElem1 := range this.data {
		for _, tmpElem2 := range elem {
			if !this.Compare(tmpElem1, tmpElem2) {

				tmpSlice = append(tmpSlice, tmpElem1)

			}
		}
	}
	this.data = tmpSlice
}

func (this *Queue) GetData() interface{} {
	return this.GetData()
}

func (this *Queue) GetMutex() sync.RWMutex {
	return this.mutex
}
