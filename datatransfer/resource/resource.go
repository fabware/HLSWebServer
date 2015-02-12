// resource.go
// 资源提供者
package resource

import (
	"bytes"
	"encoding/json"
	"utility/base"

	"strings"
	"utility/mylog"
	"utility/stat"

	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	PUSH = iota
	PULL
)

const (
	LEAVE_EVENT = iota + 1
)

type ResourceInf interface {
	SetID(id string)
	GetID() string

	Open() error
	Close() error

	ReSetPosBytime() error
	GetMoreData() error
}

type ResourceClientInf interface {
	Send(proto *base.Proto) bool
	Notify(resourceID string, clientID uint32, event int) bool
}

type ResourceClient struct {
	ClientInf ResourceClientInf
	ClientID  uint32
}

type Resourcer struct {
	ID string
	Ns string

	ResType  uint32 //资源类型，PUSH, PULL
	RePosSup bool   // 是否支持重定位

	ClientOpenChn chan []byte

	ClientDataChn []*ResourceClient

	SourceDataChn *ResourceClient

	DtClientID uint32 //DT到pu的ClientID
	Result     string
	Error      string //返回结果

	Ref   int32 // 资源引用计数， CU每次请求资源的时候，增加引用计数，释放资源减少引用计数
	valid uint32
	mutex sync.Mutex
}

// 关注资源
func (r *Resourcer) attent(chn *ResourceClient) {

	r.ClientDataChn = append(r.ClientDataChn, chn)
	mylog.GetErrorLogger().Println(r.GetID(), " attent Resource Len ", len(r.ClientDataChn))
}

// 取消资源
func (r *Resourcer) unattent(chn *ResourceClient, all bool) int {

	resultChn := make([]*ResourceClient, 0)
	for _, v := range r.ClientDataChn {
		if all {
			if v.ClientInf != chn.ClientInf {
				resultChn = append(resultChn, v)
			}

		} else if (v.ClientID != chn.ClientID) || (v.ClientInf != chn.ClientInf) {
			resultChn = append(resultChn, v)
		} else {
			mylog.GetErrorLogger().Println("unattent Resource error", r.GetID())
		}
	}

	r.ClientDataChn = resultChn
	mylog.GetErrorLogger().Println(r.GetID(), " unattent Resource Len ", len(r.ClientDataChn))
	return len(r.ClientDataChn)
}

// 注册资源
func (r *Resourcer) Register(chn *ResourceClient, resType uint32, rePos bool) {
	r.ResType = resType
	r.RePosSup = rePos
	r.SourceDataChn = chn
	r.DtClientID = chn.ClientID
	r.ClientDataChn = make([]*ResourceClient, 0)
	r.ClientOpenChn = make(chan []byte)
	atomic.CompareAndSwapUint32(&r.valid, 0, 1)
}

// 注销资源
func (r *Resourcer) Unregister() {

	defer func() {
		if re := recover(); re != nil {

			mylog.GetErrorLogger().Println(r.GetID(), "unregister Resource panic", re)

		}
	}()
	atomic.CompareAndSwapUint32(&r.valid, 1, 0)

	r.mutex.Lock()
	defer r.mutex.Unlock()
	mylog.GetErrorLogger().Println("Unregister Close")
	close(r.ClientOpenChn)

	{
		for _, chn := range r.ClientDataChn {

			chn.ClientInf.Notify(r.GetID(), chn.ClientID, LEAVE_EVENT)
		}

	}

	r.Result = ""
	r.Error = ""
}

func (r *Resourcer) add() int32 {

	return atomic.AddInt32(&r.Ref, 1)
}

func (r *Resourcer) release() int32 {
	return atomic.AddInt32(&r.Ref, -1)
}

func (r *Resourcer) GetID() string {
	return r.ID
}

func (r *Resourcer) SetID(id string) {
	r.ID = id
}

func (r *Resourcer) verificate(token *base.TokenJson) string {
	// 权限验证
	if !bytes.Equal([]byte(r.GetID()), []byte(token.ResourceID)) {
		return base.ERRRIGHT_403
	}

	pos := r.ResType / 8
	if (uint8(token.Scope[pos]) & uint8(r.ResType%256)) == 0 {
		return base.ERRRIGHT_403
	}
	return ""
}

func (r *Resourcer) Open(chn *ResourceClient, request *base.RequestJson, token *base.TokenJson) (string, string) {
	fmt.Println("1 Resourcer Open Resource ")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	fmt.Println("2 Resourcer Open Resource ")
	/*verErr := r.verificate(token)
	if verErr != "" {
		return "", verErr
	}*/
	if !atomic.CompareAndSwapUint32(&r.valid, 1, 1) {
		return "", base.NOFOUNF404
	}

	if bytes.Equal([]byte(r.Error), []byte(base.OK)) {
		mylog.GetErrorLogger().Println("Resource Has Opend")

		r.attent(chn)
		return r.Result, r.Error
	}

	{
		proto := base.GetProto()
		proto.RD.BaseHD.CommandId = base.OPEN_RESOURCE_CMD

		proto.RD.HD.ClientIdent = r.DtClientID

		proto.RD.HD.ContextType = base.CONTEXT_JSON
		proto.RD.HD.TransferType = base.TRANSFER_RAW

		b, err := json.Marshal(request)
		if err != nil {
			mylog.GetErrorLogger().Println("Resource Open Json ", err)

		}

		proto.EncodeBody(b)
		if !r.SourceDataChn.ClientInf.Send(proto) {
			close(r.ClientOpenChn)
			r.ClientOpenChn = nil
			mylog.GetErrorLogger().Println("Resource Open Send Error ", err)
			return r.Result, base.NOFOUNF404
		}

	}

	select {

	case msg, ok := <-r.ClientOpenChn: // msg只是包含数据
		if !ok {
			mylog.GetErrorLogger().Println("ClientOpenChn Open ")
			return r.Result, base.NOFOUNF404
		}

		var responseJson base.ResponseJson
		e := json.Unmarshal(msg[4:], &responseJson)
		if e != nil {
			mylog.GetErrorLogger().Println(e)
			return r.Result, base.PROTOERR400
		}

		r.Result = strings.ToLower(responseJson.Result)
		r.Error = strings.ToLower(responseJson.Error)

		if strings.EqualFold(r.Error, base.OK) {

			r.attent(chn)
		}
		return r.Result, r.Error

	case <-time.After(20 * time.Second): //TimeOut
		return r.Result, base.NOFOUNF404
	}

	return r.Result, r.Error
}

func (r *Resourcer) Close(chn *ResourceClient, ns string, all bool) (string, string) {
	mylog.GetErrorLogger().Println("Close Resource")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	mylog.GetErrorLogger().Println("Close unattent")
	openCount := r.unattent(chn, all)

	if openCount != 0 {
		return "", base.OK
	}

	{
		r.Result = ""
		r.Error = ""

		proto := base.GetProto()
		proto.RD.BaseHD.CommandId = base.CLOSE_RESOURCE_CMD
		proto.RD.HD.ClientIdent = r.SourceDataChn.ClientID
		proto.RD.HD.ContextType = base.CONTEXT_JSON
		proto.RD.HD.TransferType = base.TRANSFER_RAW

		/*todo
		1. 生成token
		2.
		*/
		var closeResParam = base.CloseResParamJson{r.GetID()}

		request := base.RequestJson{"", ns, "close", closeResParam}

		b, e := json.Marshal(request)

		if e != nil {
			mylog.GetErrorLogger().Println("Resource Close Json ", e)

		}
		proto.EncodeBody(b)

		if !r.SourceDataChn.ClientInf.Send(proto) {
			mylog.GetErrorLogger().Println("Resource Close Send Error ")
			return "", base.NOFOUNF404
		}

	}

	select {

	case msg, ok := <-r.ClientOpenChn:
		fmt.Println("ClientOpenChn Close ")
		if !ok {
			mylog.GetErrorLogger().Println("ClientOpenChn Close ")
			return "", base.NOFOUNF404
		}

		var responseJson base.ResponseJson
		e := json.Unmarshal(msg[4:], &responseJson)
		if e != nil {
			mylog.GetErrorLogger().Println("Resource Close ", e, string(msg[4:]))
			return r.Result, base.PROTOERR400
		}

		return responseJson.Result, responseJson.Error

	case <-time.After(2 * time.Second): //TimeOut
		return r.Result, base.NOFOUNF404
	}

	return r.Result, r.Error
}

func (r *Resourcer) Parse(proto *base.Proto) error {

	cmdID := proto.RD.BaseHD.CommandId & 0x7f

	if cmdID == base.OPEN_RESOURCE_CMD || cmdID == base.CLOSE_RESOURCE_CMD {
		r.ClientOpenChn <- proto.BD.Data

	} else if cmdID == base.DATA_STREAM {
		r.broadcastData(proto)

	} else {
		mylog.GetErrorLogger().Println("Now No Support CMD", cmdID)
	}
	return nil
}

func (r *Resourcer) broadcastData(proto *base.Proto) {

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !atomic.CompareAndSwapUint32(&r.valid, 1, 1) {
		return
	}

	tmpClientDataChn := make([]*ResourceClient, 0)
	for _, chn := range r.ClientDataChn {

		sendLen := uint64(proto.RD.HD.BodyLen)
		stat.GetLocalStatistInst().RecvData(sendLen)
		sendP := base.GetProto()
		sendP.RD = proto.RD
		sendP.RD.BaseHD.CommandId |= 0x80
		sendP.BD = proto.BD
		sendP.RD.HD.ClientIdent = chn.ClientID

		if chn.ClientInf.Send(sendP) {
			tmpClientDataChn = append(tmpClientDataChn, chn)
			stat.GetLocalStatistInst().SendData(sendLen)
		}
	}
	r.ClientDataChn = tmpClientDataChn
}
