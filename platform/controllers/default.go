package controllers

import (
	"github.com/astaxie/beego"
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
	c.TplNames = "example.html"
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
