package routers

import (
	"github.com/astaxie/beego"
	"platform/controllers"
)

func init() {

	beego.Router("/", &controllers.MainController{})

	beego.Router("/example", &controllers.HlsController{})
	beego.Router("/crossdomain.xml", &controllers.CrossDomainController{})
	beego.SetStaticPath("/hls/", "/static/hls")
	beego.SetStaticPath("/apple/*.m3u8", "/static/hls")
	beego.Router("/shao", &controllers.ShaoHlsController{})
}
