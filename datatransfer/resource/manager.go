// manager.go
package resource

import (
	"fmt"
	//"mylog"

	"sync"
)

type resourcerPool struct {
	poolAlloc map[string]*Resourcer
	poolFree  []*Resourcer
	mutex     sync.Mutex
}

var oneResPool *resourcerPool = nil
var oneResPoolOnce sync.Once

func initPool() {
	oneResPool = new(resourcerPool)
	oneResPool.poolAlloc = make(map[string]*Resourcer)

}

func resPoolInst() *resourcerPool {
	oneResPoolOnce.Do(initPool)
	return oneResPool
}

func CreateResource(id string) *Resourcer {
	resPoolInst().mutex.Lock()
	defer resPoolInst().mutex.Unlock()

	if len(resPoolInst().poolFree) == 0 {
		r := new(Resourcer)
		r.SetID(id)
		r.add()
		resPoolInst().poolAlloc[id] = r
		return r
	}

	r := resPoolInst().poolFree[0]
	r.SetID(id)
	resPoolInst().poolFree = resPoolInst().poolFree[1:]
	resPoolInst().poolAlloc[id] = r
	r.add()

	return r
}

func CheckResourceIsExist(id string) bool {
	resPoolInst().mutex.Lock()
	defer resPoolInst().mutex.Unlock()

	_, ok := resPoolInst().poolAlloc[id]
	if ok {
		return true
	}
	return false

}

func GetResourcerByID(id string) *Resourcer {

	resPoolInst().mutex.Lock()
	defer resPoolInst().mutex.Unlock()

	value, ok := resPoolInst().poolAlloc[id]
	if ok {
		value.add()
		return value
	}
	return nil
}

/*func GetResourcerByClientID(id uint32) *Resourcer {

	resPoolInst().mutex.Lock()
	defer resPoolInst().mutex.Unlock()

	for _, r := range resPoolInst().poolAlloc {
		if r.GetClientID() == id {
			r.add()
			return r
		}
	}

	if len(resPoolInst().poolFree) == 0 {
		r := new(Resourcer)
		r.SetClientID(id)
		r.add()
		resPoolInst().poolAlloc = append(resPoolInst().poolAlloc, r)
		return r
	}

	r := resPoolInst().poolFree[0]
	r.SetClientID(id)
	resPoolInst().poolFree = resPoolInst().poolFree[1:]
	resPoolInst().poolAlloc = append(resPoolInst().poolAlloc, r)
	r.add()
	return r
}*/

func ReleaseResourcer(res *Resourcer) error {

	resPoolInst().mutex.Lock()
	defer resPoolInst().mutex.Unlock()

	for _, r := range resPoolInst().poolAlloc {

		if r == res && r.release() == 0 {

			resPoolInst().poolFree = append(resPoolInst().poolFree, r)
			delete(resPoolInst().poolAlloc, res.GetID())
		}
	}
	fmt.Println("Resource Pool ", len(resPoolInst().poolAlloc))
	return nil
}
