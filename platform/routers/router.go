package routers

import (
	"github.com/astaxie/beego"
	"platform/controllers"
)

func init() {
	beego.Router("/", &controllers.MainController{})
	beego.SetStaticPath("/hls", "/static/hls")
}
