// channel
package resource

import (
	"encoding/json"
	"utility/base"

	"fmt"
	"net"
	"strings"
	"sync"
	"utility/mylog"
	"utility/stat"

	"sync/atomic"
	"time"
)

type notifyEvent struct {
	resourceID string
	clientID   uint32
	event      int
}

type Channel struct {
	connSocket net.Conn

	rwChn     chan *base.Proto
	notifyChn chan notifyEvent
	heartChn  chan bool

	chnMutex sync.RWMutex // 保护资源相关通道

	openRes     map[uint32]*Resourcer //	关注的资源
	registerRes map[string]*Resourcer // 注册的资源
	resMutex    sync.RWMutex          // 保护资源map

	valid uint32 //连接通道的有效性，0无效，1有效

}

// 外部访问接口
func (chn *Channel) Send(proto *base.Proto) bool {
	// 检查通道否有效
	defer func() {
		if rc := recover(); rc != nil {
			chn.handleError(base.DTerror{"Send Panic"})
		}
	}()

	chn.chnMutex.Lock()
	defer chn.chnMutex.Unlock()

	if atomic.CompareAndSwapUint32(&chn.valid, 0, 0) {
		return false
	}

	chn.rwChn <- proto
	return true
}

// 外部访问接口，由资源通知资源订阅者相关事件
// 对外部访问接口枷锁保护，以后的版本采用消息机制，去除锁
func (chn *Channel) Notify(resourceID string, clientID uint32, event int) bool {
	defer func() {
		if r := recover(); r != nil {
			err := base.DTerror{"Notify Panic"}
			chn.handleError(err)
		}
	}()

	chn.chnMutex.Lock()
	defer chn.chnMutex.Unlock()

	if atomic.CompareAndSwapUint32(&chn.valid, 0, 0) {
		return false
	}

	chn.notifyChn <- notifyEvent{resourceID, clientID, event}
	mylog.GetErrorLogger().Println("Leave notify")
	return true
}

// 内部访问接口
// 错误处理
func (chn *Channel) handleError(err error) {

	defer func() {
		if r := recover(); r != nil {
			mylog.GetErrorLogger().Println("Channel handleError Panic")
		}
	}()

	if err != nil {
		mylog.GetErrorLogger().Println("handleError", err.Error())
	}

	if atomic.CompareAndSwapUint32(&chn.valid, 0, 0) {
		return
	}

	atomic.CompareAndSwapUint32(&chn.valid, 1, 0)

	stat.GetLocalStatistInst().Off()

	func() {
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		for _, r := range chn.openRes {
			stat.GetLocalStatistInst().CloseRes()
			mylog.GetErrorLogger().Println(" release chn res ", r.GetID())
			clientDataID := new(ResourceClient)
			clientDataID.ClientInf = chn
			r.Close(clientDataID, "", true)
			ReleaseResourcer(r)
		}

		for _, v := range chn.registerRes {

			v.Unregister()
			ReleaseResourcer(v)
		}
	}()

	func() {

		fmt.Println("Chn Close")
		chn.chnMutex.Lock()
		defer chn.chnMutex.Unlock()
		chn.connSocket.Close()
		close(chn.rwChn)
		close(chn.notifyChn)
	}()

}

// 心跳处理,主动发起心跳
func (chn *Channel) timerSendHeartTask() {

	mylog.GetErrorLogger().Println(" Begin Chn Heart Task")
	defer func() {
		if r := recover(); r != nil {
			chn.handleError(base.DTerror{"Notify Panic"})
		}
	}()

	proto := base.GetProto()
	proto.RD.BaseHD.CommandId = base.HEART_CMD

	var failCount int = 0

	for {

		if atomic.CompareAndSwapUint32(&chn.valid, 0, 0) {
			break
		}

		if !chn.Send(proto) {
			chn.handleError(base.DTerror{"Send Error"})
			break
		}
		select {
		case <-chn.heartChn:

			failCount = 0
		case <-time.After(2 * time.Second):
			failCount++
		}
		if failCount >= 5 {
			chn.handleError(base.DTerror{"Heart Send Fail"})
			break
		}
		time.Sleep(10 * time.Second)

	}
	mylog.GetErrorLogger().Println(" End Chn Heart Task")
}

func (chn *Channel) sendProto(cID uint32, cmd uint8, result string, err string, ns string) {

	var method string
	if cmd == base.OPEN_RESOURCE_CMD {
		method = "open"
	} else if cmd == base.CLOSE_RESOURCE_CMD {
		method = "close"
	} else if cmd == base.REGISTER_RESOURCE {
		method = "register"
	}
	proto := base.GetProto()

	proto.RD.BaseHD.CommandId = 0x80 | cmd
	proto.RD.HD.ClientIdent = cID
	proto.RD.HD.ContextType = base.CONTEXT_JSON
	proto.RD.HD.TransferType = base.TRANSFER_RAW

	responseJson := base.ResponseJson{ns, method, result, err}
	b, jsonE := json.Marshal(responseJson)
	if jsonE != nil {
		mylog.GetErrorLogger().Println("handleOpenResource : ", jsonE)
	}

	proto.EncodeBody(b)

	go chn.Send(proto)

}

func (chn *Channel) getReqParam(clientID uint32, data []byte) (*base.RequestJson, *base.TokenJson, error) {

	var request base.RequestJson

	err := json.Unmarshal(data, &request)
	if err != nil {

		mylog.GetErrorLogger().Println("getReqParam", err, string(data))

		return nil, nil, base.DTerror{base.PROTOERR400}
	}
	//token权限验证
	var rightJson base.TokenJson
	mylog.GetErrorLogger().Println("token ", string(data))
	if len(request.Token) <= 0 {
		return &request, nil, nil
	}
	rightErr := base.UnmarshalToken(request.Token, &rightJson)
	if rightErr != nil {
		mylog.GetErrorLogger().Println("getReqParam", err)
		return nil, nil, base.DTerror{base.PROTOERR400}
	}

	return &request, &rightJson, nil

}

func (chn *Channel) CheckResIsOpen(ID uint32) bool {

	chn.resMutex.RLock()
	defer chn.resMutex.RUnlock()
	if _, ok := chn.openRes[ID]; ok {
		return true
	}
	return false

}

func (chn *Channel) handleReqOpenRes(clientID uint32, data []byte) {

	defer func() {
		if r := recover(); r != nil {
			chn.handleError(base.DTerror{"handleReqOpenResr Panic"})
		}
	}()

	var responseErr string = ""
	var resourcer *Resourcer = nil
	var timeOut uint32 = 20 // 默认超时20s

	request, token, err := chn.getReqParam(clientID, data[4:])
	if err != nil {
		chn.sendProto(clientID,
			base.OPEN_RESOURCE_CMD,
			"", err.Error(),
			request.Ns)
		return
	}

	//获取资源
	param := request.Param.(map[string]interface{})
	openResParam := base.OpenResParamJson{param["resourceid"].(string),
		uint32(param["timeout"].(float64))}

	if chn.CheckResIsOpen(clientID) {
		chn.sendProto(clientID,
			base.OPEN_RESOURCE_CMD,
			"", base.OPEN303,
			request.Ns)
		return
	}

	mylog.GetErrorLogger().Println("Chn Open Resource ID ", openResParam.ID)

	if openResParam.TimeOut < 120 { //当超时时间大于120s，设置超时为默认

		timeOut = openResParam.TimeOut
	}

	for delayWait := uint32(0); delayWait < timeOut; delayWait++ {

		resourcer = GetResourcerByID(openResParam.ID)
		if resourcer != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if resourcer == nil {
		mylog.GetErrorLogger().Println("No Found Source ", openResParam.ID)

		chn.sendProto(clientID, base.OPEN_RESOURCE_CMD, "", base.NOFOUNF404, request.Ns)
		ReleaseResourcer(resourcer)

		return
	}

	clientDataID := new(ResourceClient)
	clientDataID.ClientInf = chn
	clientDataID.ClientID = clientID

	_, responseErr = resourcer.Open(clientDataID, request, token)
	responseJson := base.ResponseJson{request.Ns, request.Method, "", responseErr}
	b, err := json.Marshal(responseJson)
	if err != nil {
		mylog.GetErrorLogger().Println("handleOpenResource : ", err)
		ReleaseResourcer(resourcer)
		return
	}

	proto := new(base.Proto)
	proto.RD.BaseHD.CommandId = 0x80 | base.OPEN_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID

	proto.EncodeBody(b)

	if !chn.Send(proto) {
		ReleaseResourcer(resourcer)
		return
	}

	if !strings.EqualFold(responseErr, base.OK) {
		ReleaseResourcer(resourcer)
		return
	}

	{
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		chn.openRes[clientID] = resourcer
	}
	stat.GetLocalStatistInst().OpenRes()

}

func (chn *Channel) handleReqCloseRes(clientID uint32, data []byte) {

	mylog.GetErrorLogger().Println("handleReqCloseRes", string(data[4:]))
	defer func() {
		if r := recover(); r != nil {
			chn.sendProto(clientID,
				base.CLOSE_RESOURCE_CMD,
				"", base.PROTOERR400,
				"huamail.defualt")
			chn.handleError(base.DTerror{"handleReqCloseRes Panic"})
		}
	}()

	var responseResult, responseErr string = "", base.NOFOUNF404

	request, _, err := chn.getReqParam(clientID, data[4:])
	if err != nil {
		chn.sendProto(clientID,
			base.CLOSE_RESOURCE_CMD,
			responseResult, err.Error(),
			"huamail.defualt")
		return
	}

	func() {
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		resourcer, ok := chn.openRes[clientID]

		if ok {
			mylog.GetErrorLogger().Println("Chn Close Resource ID ", resourcer.GetID())
			clientDataID := new(ResourceClient)
			clientDataID.ClientInf = chn
			clientDataID.ClientID = clientID
			responseResult, responseErr = resourcer.Close(clientDataID, "", false)
			ReleaseResourcer(resourcer)
			delete(chn.openRes, clientID)
		}
	}()

	proto := base.GetProto()
	proto.RD.BaseHD.CommandId = 0x80 | base.CLOSE_RESOURCE_CMD
	proto.RD.HD.ClientIdent = clientID

	responseJson := base.ResponseJson{request.Ns, request.Method, responseResult, responseErr}
	b, err := json.Marshal(responseJson)
	if err != nil {
		mylog.GetErrorLogger().Println("handleCloseResource : ", err)
	}
	proto.EncodeBody(b)

	chn.Send(proto)
	stat.GetLocalStatistInst().CloseRes()
}

func (chn *Channel) handleEventNotify(event notifyEvent) {

	func() {
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		resourcer, ok := chn.openRes[event.clientID]

		if ok {
			ReleaseResourcer(resourcer)
			delete(chn.openRes, event.clientID)
		}
	}()

	proto := base.GetProto()

	proto.RD.BaseHD.CommandId = base.LEAVE_NOTIFY
	proto.RD.HD.ClientIdent = event.clientID
	proto.RD.HD.ContextType = base.CONTEXT_JSON
	proto.RD.HD.TransferType = base.TRANSFER_RAW
	leaveNotify := base.LeaveNotifyParamJson{event.resourceID}

	request := base.RequestJson{"", "ns", "notify", leaveNotify}
	requestJson, err := json.Marshal(request)
	if err != nil {
		mylog.GetErrorLogger().Println("Resource Unregister Json ", err)
	}
	proto.EncodeBody(requestJson)
	chn.Send(proto)

}

func (chn *Channel) handleRspRes(proto *base.Proto) {

	var res *Resourcer = nil
	clientID := proto.RD.HD.ClientIdent

	for _, r := range chn.registerRes {

		if r.SourceDataChn.ClientID == clientID {
			res = r
			break
		}
	}
	if res == nil {
		return
	}
	res.Parse(proto)
}

// 注册资源
func (chn *Channel) handleRegisterRes(clientID uint32, data []byte) {

	defer func() {
		if r := recover(); r != nil {
			chn.sendProto(clientID,
				base.REGISTER_RESOURCE,
				"", base.PROTOERR400,
				"huamail.defualt")
			chn.handleError(base.DTerror{"handleRegisterRes Panic"})
		}
	}()
	var resourcer *Resourcer = nil

	request, _, err := chn.getReqParam(clientID, data[4:])
	if err != nil {
		mylog.GetErrorLogger().Println("handleRegisterRes", err)
		chn.sendProto(clientID,
			base.REGISTER_RESOURCE,
			"",
			base.PROTOERR400,
			"huamai.default")
		return
	}

	param := request.Param.(map[string]interface{})
	registerResParam := base.RegisterResParamJson{param["resourceid"].(string),
		uint32(param["restype"].(float64)),
		param["repos"].(bool)}
	resourceID := registerResParam.ID
	if CheckResourceIsExist(resourceID) {
		chn.sendProto(clientID,
			base.REGISTER_RESOURCE,
			"",
			base.EXISTRES_400,
			request.Ns)

		return

	}

	mylog.GetErrorLogger().Println("Register Source")

	resourcer = CreateResource(registerResParam.ID)

	if resourcer == nil {
		chn.sendProto(clientID, base.REGISTER_RESOURCE, "", base.INNERR_500, request.Ns)
		return
	}

	sourceDataChn := new(ResourceClient)
	sourceDataChn.ClientID = clientID
	sourceDataChn.ClientInf = chn
	resourcer.Register(sourceDataChn, registerResParam.ResType, registerResParam.RePos)

	func() {
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		chn.registerRes[registerResParam.ID] = resourcer
	}()

	chn.sendProto(clientID, base.REGISTER_RESOURCE, "", base.OK, request.Ns)
	fmt.Println("Resgitser OK")
}

func (chn *Channel) handleUnregisterRes(clientID uint32, data []byte) {

	request, _, err := chn.getReqParam(clientID, data)
	if err != nil {
		mylog.GetErrorLogger().Println("handleUnregisterRes", err)
		chn.sendProto(clientID, base.UNREGISTER_RESOURCE, "", base.PROTOERR400, request.Ns)
		return
	}
	param := request.Param.(map[string]interface{})
	resourceID := param["resourceid"].(string)

	func() {
		chn.resMutex.Lock()
		defer chn.resMutex.Unlock()
		resourcer, ok := chn.registerRes[resourceID]
		if ok {
			mylog.GetErrorLogger().Fatalln("Release Resource ", resourceID)
			resourcer.Unregister()
			ReleaseResourcer(resourcer)
			delete(chn.registerRes, resourceID)
		}
	}()

	chn.sendProto(clientID, base.UNREGISTER_RESOURCE, "", base.OK, request.Ns)
}

func (chn *Channel) handleOtherCMD(proto *base.Proto) {

	cmdID := proto.RD.BaseHD.CommandId
	mylog.GetErrorLogger().Println("handleOtherCMD", cmdID)
	error := base.NOSUPPORT501
	responseJson := base.ResponseJson{"", "", "", error}
	b, err := json.Marshal(responseJson)
	if err != nil {
		mylog.GetErrorLogger().Println("handleOtherCMD : ", err)
	}

	proto.EncodeBody(b)
	chn.Send(proto)
}

func (chn *Channel) handleWriteSocket(proto *base.Proto) {

	if atomic.CompareAndSwapUint32(&chn.valid, 0, 0) {
		return
	}
	cmdID := proto.RD.BaseHD.CommandId & 0x7f

	if cmdID == base.HEART_CMD {
		heartHdr := new(base.BaseHeader)
		heartHdr.CommandId = proto.RD.BaseHD.CommandId
		msg := heartHdr.Encode()
		_, err := chn.connSocket.Write(msg.Bytes())

		if err != nil {
			chn.handleError(base.DTerror{"Send Error"})
		}
		return
	}
	fmt.Println("handleWriteSocket ", cmdID)
	msg := proto.EncodeHdr()
	_, err := chn.connSocket.Write(msg.Bytes())
	if err != nil {
		chn.handleError(base.DTerror{"Send Error"})

	}
	_, dErr := chn.connSocket.Write(proto.BD.Data)
	if dErr != nil {
		chn.handleError(base.DTerror{"Send Error"})
	}
	base.PutProto(proto)

}

func (chn *Channel) handleReadSocketTask() {

	var productID uint32 = 1

	defer func() {
		if r := recover(); r != nil {
			chn.handleError(base.DTerror{"handleReadSocketTask Panic"})
		}
	}()
	for {

		proto := base.GetProto()
		e := proto.ReadBinaryProto(chn.connSocket)
		if e != nil {

			chn.handleError(base.DTerror{"handleReadSocketTask"})
			break
		}

		consumerID := proto.RD.HD.ClientIdent
		cmdID := proto.RD.BaseHD.CommandId & 0x7f
		isRequest := proto.RD.BaseHD.CommandId & 0x80

		switch cmdID {

		case base.HEART_CMD:
			if isRequest == 0 {

				heartProto := base.GetProto()

				heartProto.RD.BaseHD.CommandId = base.HEART_CMD | 0x80

				chn.Send(heartProto)

			} else {
				chn.heartChn <- true
			}

		case base.OPEN_RESOURCE_CMD:
			// 打开资源
			if isRequest == 0 {
				//fmt.Println("打开资源")
				go chn.handleReqOpenRes(consumerID, proto.BD.Data)

			} else {
				//fmt.Println("回应资源")
				go chn.handleRspRes(proto)
			}

		case base.CLOSE_RESOURCE_CMD:
			// 关闭资源
			if isRequest == 0 {
				go chn.handleReqCloseRes(consumerID, proto.BD.Data)
			} else {
				go chn.handleRspRes(proto)
			}

		case base.CRTL_RESOURCE_CMD:
			// 控制资源
		case base.REGISTER_RESOURCE:
			// 注册资源
			go chn.handleRegisterRes(productID, proto.BD.Data)
			productID++
		case base.UNREGISTER_RESOURCE:
			// 注销资源

		case base.DATA_STREAM:
			// 数据流
			chn.handleRspRes(proto)

		default:
			go chn.handleOtherCMD(proto)
		}
		base.PutProto(proto)
	}

	//fmt.Println("End Cu Read Task")
}

func (chn *Channel) handleReadChnTask() {
	//fmt.Println("Begin handleReadChnTask")

	defer func() {
		if r := recover(); r != nil {
			chn.handleError(base.DTerror{"handleReadChnTask Panic"})
		}
	}()

	for atomic.CompareAndSwapUint32(&chn.valid, 1, 1) {

		proto, ok := <-chn.rwChn
		if ok {
			chn.handleWriteSocket(proto)
		} else {
			chn.handleError(base.DTerror{"handleReadChnTask Panic"})
		}

	}

}

func (chn *Channel) handleNotifyTask() {

	defer func() {
		if r := recover(); r != nil {
			chn.handleError(base.DTerror{"handleNotifyTask Panic"})
		}
	}()

	for atomic.CompareAndSwapUint32(&chn.valid, 1, 1) {

		msg, ok := <-chn.notifyChn
		if ok {
			chn.handleEventNotify(msg)
		} else {

			chn.handleError(base.DTerror{"Notify Chn Close"})

		}
	}
	fmt.Println("handleNotifyTask Exit")

}

func (chn *Channel) Run(connSocket net.Conn) error {

	chn.rwChn = make(chan *base.Proto, 24)
	chn.notifyChn = make(chan notifyEvent, 1)
	chn.heartChn = make(chan bool, 1)

	chn.openRes = make(map[uint32]*Resourcer)
	chn.registerRes = make(map[string]*Resourcer)

	chn.connSocket = connSocket
	chn.valid = 1 // CU有效

	stat.GetLocalStatistInst().On()

	go chn.timerSendHeartTask()
	go chn.handleReadChnTask()
	go chn.handleReadSocketTask()
	go chn.handleNotifyTask()

	return nil
}
