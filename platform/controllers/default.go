package controllers

import (
	"fmt"
	"github.com/astaxie/beego"
	"net/http"
)

type MainController struct {
	beego.Controller
}

func (c *MainController) Get() {
	c.Data["Website"] = "beego.me"
	c.Data["Email"] = "astaxie@gmail.com"
	c.TplNames = "index.tpl"
}

type HlsController struct {
	beego.Controller
}

func (c *HlsController) Get() {

	// 通过ffmpeg作为切片脚本
	// /hls/real?method=ffm&resourceid=1

	//通过dll切片
	// /hls/real?method=dll&resourceid=1

	// 不切片 生成文件
	// /hls/real?method=debug&resourceid=1

	method := c.GetString("method")
	resourceid := c.GetString("resourceid")
	fmt.Println("Begin Request Open video", method, " ", resourceid)

	hlsRquest := &http.Client{}

	res, err := hlsRquest.Get("http://localhost:9998/open/real?method=" +
		method +
		"&resourceid=" + resourceid)
	fmt.Println("Request Open Video", res, err)
	c.Data["ID"] = resourceid
	c.TplNames = "hls.tpl"
}

type CrossDomainController struct {
	beego.Controller
}

func (c *CrossDomainController) Get() {
	c.Ctx.Output.ContentType("application/xml")
	c.Data["xml"] = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" +
		"<cross-domain-policy>" +
		"<allow-access-from domain=\"*\"/>" +
		"</cross-domain-policy>"
	c.ServeXml()
}

type ShaoHlsController struct {
	beego.Controller
}

func (c *ShaoHlsController) Get() {
	c.TplNames = "shao.html"
}

type ExampleHlsController struct {
	beego.Controller
}

func (c *ExampleHlsController) Get() {
	c.TplNames = "example.html"
}
