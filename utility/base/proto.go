// proto.go
package base

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"reflect"
	"sync"
	"utility/mylog"
)

const (
	OK           = "ok"
	OPEN303      = "303 : Res Has Open"
	PROTOERR400  = "400 : ProtoError"
	NOFOUNF404   = "404 : No Found Resource"
	NOSUPPORT501 = "501 : No Support Command"
	EXISTRES_400 = "400 : Resource Has Registered"
	ERRRIGHT_403 = "404 : Forbid"
	INNERR_500   = "500 : Server Inner Error"
)

const (
	REGISTER_RESOURCE   = 0x10 // 注册资源
	UNREGISTER_RESOURCE = 0x11 // 注销资源

	OPEN_RESOURCE_CMD  = 0x20 // 打开资源
	CRTL_RESOURCE_CMD  = 0x21 // 控制资源
	DATA_STREAM        = 0x22 // 数据流
	CLOSE_RESOURCE_CMD = 0x23 // 关闭资源
	LEAVE_NOTIFY       = 0x24 // 事件通知

	HEART_CMD = 0x7f //心跳
)

const (
	M1 = 1024 * 1024
)

const (
	BaseHeaderLenC = 8
	HeaderLenC     = 12
)

const (
	CONTEXT_TEXT  = 0x0000 //般文本信息
	CONTEXT_BIN   = 0x0100 //一般二进制信息
	CONTEXT_JSON  = 0x0200 //json编码的远程调用消息
	CONTEXT_XML   = 0x0201 //xml编码的远程调用消息
	CONTEXT_VIDEO = 0x0300 //视频帧
	CONTEXT_AUDIO = 0x0400 //音频帧
)

const (
	TRANSFER_RAW = 0x0000 //原始数据传输
	CONTEXT_GZIP = 0x0100 //gzip压缩传输
	CONTEXT_DES  = 0x0001 //DES加密传输
	CONTEXT_GD   = 0x0101 //压缩加密传输

)

// right 权限表
const (
	VIDEO_RIGHT = 1 << iota
	AUDIO_RIGHT = 1 << iota
)

type BaseHeader struct {
	MagicIdent   [2]byte //  'hd'
	MagicVersion uint8   //  1
	CommandId    uint8   //  0xFFU保留为心跳指令使用
	HeaderLen    uint16  //  头部长度
	CheckSum     uint16  //  头部校验

}

type Header struct {
	BodyLen      uint32 //  消息总长度
	ContextType  uint16 //  消息体编码格式
	TransferType uint16 //  消息体传输格式
	ClientIdent  uint32 //  客户的资源请求ID. 透传回客户端.
}

type RealHeader struct {
	BaseHD BaseHeader
	HD     Header
}

type Body struct {
	Data []byte
}

type Proto struct {
	RD RealHeader
	BD *Body
}

// 协议内存池
var ProtoPool sync.Pool = sync.Pool{
	New: func() interface{} {
		return new(Proto)
	},
}

type TokenJson struct {
	Ver        string `json:"v"`
	Expire     int32  `json:"expire"`
	ResourceID string `json:"rid"`
	Scope      string `json:"scope"`
	ClietID    string `json:"cid"`
}

var PKS1024BIG *rsa.PublicKey

func InitRsaBig(pKSModule string, pKSExp string) error {
	module, mE := base64.StdEncoding.DecodeString(pKSModule)
	if mE != nil {
		fmt.Println(mE)
		return mE
	}
	exp, eE := base64.StdEncoding.DecodeString(pKSExp)
	if eE != nil {
		fmt.Println(eE)
		return eE
	}

	moduleBig := new(big.Int)
	moduleBig.SetBytes(module)
	expBig := new(big.Int)
	expBig.SetBytes(exp)
	PKS1024BIG = &rsa.PublicKey{moduleBig, int(expBig.Int64())}
	fmt.Printf("module : %v, exp : %v\n", module, expBig.Int64())
	return nil
}

func DecryptPKCS1v15ByPub(source []byte) (out []byte, err error) {

	biText := new(big.Int)
	fmt.Println("source 1 ", source)
	biText.SetBytes(source)
	fmt.Println("source 2 ", biText)
	eBig := big.NewInt(int64(PKS1024BIG.E))

	resultBig := new(big.Int)
	resultBig.Exp(biText, eBig, PKS1024BIG.N)

	return resultBig.Bytes(), nil
}

func UnmarshalToken(token string, data *TokenJson) error {
	//1:将密文令牌T使用base64解码转换为字节数组C
	byteC, base64Err := base64.StdEncoding.DecodeString(token)
	if base64Err != nil {
		return base64Err
	}
	fmt.Println("Base64Err", byteC)
	ver := byteC[0]
	if ver == 0x1 {
		zeroSplite := 0
		Res, _ := DecryptPKCS1v15ByPub(byteC[1:])
		for k, v := range Res {
			if v == 0 {
				zeroSplite = k + 1
				break
			}

		}
		fmt.Println(string(Res[zeroSplite:]))
		err := json.Unmarshal(Res[zeroSplite:], data)
		if err != nil {
			return err
		}

	}

	return nil
}

type RequestJson struct {
	Token  string      `json:"token"`
	Ns     string      `json:"ns"`
	Method string      `json:"method"`
	Param  interface{} `json:"params"`
}

type ResponseJson struct {
	Ns     string `json:"ns"`
	Method string `json:"method"`
	Result string `json:"result"` // SUCCESS,FAIL
	Error  string `json:"error"`  // 错误原因
}

type OpenResParamJson struct {
	ID      string `json:"resourceid"`
	TimeOut uint32 `json:"timeout"` //请求资源超时时间设置
}

type CloseResParamJson struct {
	ID string `json:"resourceid"`
}

type RegisterResParamJson struct {
	ID      string `json:"resourceid"`
	ResType uint32 `json:"restype"`
	RePos   bool   `json:"repos"`
}

type UnregisterResParamJson struct {
	ID string `json:"resourceid"`
}

type LeaveNotifyParamJson struct {
	ID string `json:"resourceid"`
}

func (p *BaseHeader) Decode(buf []byte) error {

	bytesB := bytes.NewBuffer(buf)

	e := binary.Read(bytesB, binary.LittleEndian, p)
	if e != nil {
		return e
	}

	return nil
}

func (p *BaseHeader) Encode() *bytes.Buffer {
	bytesB := new(bytes.Buffer)

	p.MagicIdent[0] = 'd'
	p.MagicIdent[1] = 'h'

	baseHdElem := reflect.ValueOf(p).Elem()
	for i := 0; i < baseHdElem.NumField(); i++ {
		f := baseHdElem.Field(i)
		binary.Write(bytesB, binary.LittleEndian, f.Interface())
	}
	return bytesB
}

func (p *Header) Decode(buf []byte) error {

	bytesB := bytes.NewBuffer(buf)
	//fmt.Println("Header Decode", buf)
	e := binary.Read(bytesB, binary.LittleEndian, p)
	if e != nil {
		return e
	}

	//fmt.Println("Header", p.BodyLen, p.ClientIdent)
	return nil
}

func GetProto() *Proto {
	//sendP := ProtoPool.Get().(*Proto)
	sendP := new(Proto)
	return sendP
}

func PutProto(p *Proto) {
	//ProtoPool.Put(p)
}

func (p *Proto) EncodeHdr() *bytes.Buffer {

	p.RD.BaseHD.MagicIdent[0] = 'd'
	p.RD.BaseHD.MagicIdent[1] = 'h'
	p.RD.BaseHD.MagicVersion = 1
	p.RD.BaseHD.HeaderLen = HeaderLenC
	p.RD.HD.BodyLen = uint32(len(p.BD.Data))

	msg := new(bytes.Buffer)

	baseHdElem := reflect.ValueOf(&p.RD.BaseHD).Elem()
	for i := 0; i < baseHdElem.NumField(); i++ {
		f := baseHdElem.Field(i)
		binary.Write(msg, binary.LittleEndian, f.Interface())
	}

	hdElem := reflect.ValueOf(&p.RD.HD).Elem()
	for i := 0; i < hdElem.NumField(); i++ {
		f := hdElem.Field(i)
		binary.Write(msg, binary.LittleEndian, f.Interface())
	}

	return msg
}

func (p *Proto) EncodeBody(buf []byte) {
	msg := new(bytes.Buffer)
	var bodyLen uint32 = uint32(len(buf))
	binary.Write(msg, binary.LittleEndian, bodyLen)

	binary.Write(msg, binary.LittleEndian, buf)
	p.BD = new(Body)
	p.BD.Data = msg.Bytes()

	return

}

func (p *Proto) ReadBinaryProto(connSocket net.Conn) error {

	tmpBaseHdSlice := make([]byte, BaseHeaderLenC)
	{
		n, e := io.ReadFull(connSocket, tmpBaseHdSlice)
		if e != nil {
			mylog.GetErrorLogger().Println("311", e)
			return e
		}

		if n != BaseHeaderLenC {
			return DTerror{PROTOERR400}
		}
		(&p.RD.BaseHD).Decode(tmpBaseHdSlice)

		if p.RD.BaseHD.CommandId == HEART_CMD || p.RD.BaseHD.HeaderLen == 0 {

			return nil
		}
	}

	if p.RD.BaseHD.HeaderLen != HeaderLenC {
		mylog.GetErrorLogger().Println("Recv Msg Error 1", p.RD.BaseHD.HeaderLen)
		return DTerror{PROTOERR400}
	}

	tmpHdSlice := make([]byte, HeaderLenC)

	{
		n, e := io.ReadFull(connSocket, tmpHdSlice)
		if e != nil {
			return e
		}
		if n != HeaderLenC {
			return DTerror{PROTOERR400}
		}
		(&p.RD.HD).Decode(tmpHdSlice)
		if p.RD.HD.BodyLen <= 4 || p.RD.HD.BodyLen > M1 {
			mylog.GetErrorLogger().Println("Recv Msg Error 2", p.RD.HD.BodyLen)
			return nil
		}
	}

	p.BD = new(Body)
	p.BD.Data = make([]byte, p.RD.HD.BodyLen)
	{

		n, e := io.ReadFull(connSocket, p.BD.Data)
		if e != nil {
			return e
		}
		if n != int(p.RD.HD.BodyLen) {
			return DTerror{PROTOERR400}
		}
		//fmt.Println("Recv Msg Error", body.DataLen)
	}

	return nil

}
