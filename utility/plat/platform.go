package plat

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
)

type LoginInfo struct {
	UserName   string `json:"UserName"`
	Password   string `json:"Password"`
	RememberMe bool   `json:"RememberMe"`
}

type RegisterInfo struct {
	UserName   string `json:"UserName"`
	Password   string `json:"Password"`
	ConfirmPwd string `json:"ConfirmPassword"`
}

type TokenInfo struct {
	ResourceID string `json:"resourceId"`
	Scope      []byte `json:"scope"`
}

type ResponseToken struct {
	AccessToken string `json:"AccessToken"`
	Version     string `json:"Version"`
	ExpiresIn   uint32 `json:"ExpiresIn"`
	ResourceId  string `json:"ResourceId"`
	Scope       []byte `json:"Scope"`
	ClientId    string `json:"ClientId"`
}

type Platform struct {
	Url      string
	userName string
	password string
	jar      http.CookieJar
}

func (this *Platform) operPlatform(method string, param []byte, buf *bytes.Buffer) (int, error) {
	if this.jar == nil {
		this.jar, _ = cookiejar.New(nil)
	}

	tr := &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		DisableCompression: true,
	}

	client := &http.Client{Transport: tr, Jar: this.jar}

	postStr := new(bytes.Buffer)
	postStr.WriteString(this.Url)
	postStr.WriteString(method)

	requestStr := bytes.NewBuffer(param)
	fmt.Println(requestStr, postStr)

	resp, rspErr := client.Post(postStr.String(), "application/json", requestStr)
	if rspErr != nil {
		fmt.Println(rspErr)
		return http.StatusBadRequest, rspErr
	}

	defer resp.Body.Close()
	for _, v := range resp.Cookies() {
		fmt.Println("Cookie ", v.Name, v.Value)

	}

	if buf != nil {
		tmp := make([]byte, 512)
		n, _ := resp.Body.Read(tmp)
		buf.Write(tmp[:n])

	}
	return resp.StatusCode, nil
}

// 账号注册
func (this *Platform) Register(info RegisterInfo) (int, error) {

	requestJson, err := json.Marshal(info)
	if err != nil {
		fmt.Println(err)
		return http.StatusBadRequest, err
	}
	return this.operPlatform("/api/Register", requestJson, nil)
}

// 登陆平台
func (this *Platform) Login(info LoginInfo) (int, error) {

	requestJson, err := json.Marshal(info)
	if err != nil {
		fmt.Println(err)
		return http.StatusBadRequest, err
	}
	return this.operPlatform("/api/Login", requestJson, nil)

}

// 登出平台
/*func (this *Platform) LogOut() (int, error) {

	requestJson, err := json.Marshal(info)
	if err != nil {
		fmt.Println(err)
		return http.StatusBadRequest, err
	}
	return this.operPlatform("api/LogOut", requestJson)

}*/

// 获取资源访问token
func (this *Platform) GetToken(info TokenInfo, res *ResponseToken) error {

	fmt.Println("GetToken")
	requestJson, err := json.Marshal(info)
	if err != nil {
		fmt.Println(err)
		return err
	}
	buf := new(bytes.Buffer)
	code, err := this.operPlatform("/api/AccessToken", requestJson, buf)
	if err != nil {
		fmt.Println(code)
		return err
	}

	err = json.Unmarshal(buf.Bytes(), res)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}
