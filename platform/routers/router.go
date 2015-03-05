package routers

import (
	"github.com/astaxie/beego"
	"platform/controllers"
)

func init() {

	beego.Router("/", &controllers.MainController{})
	beego.Router("/example", &controllers.ExampleHlsController{})
	beego.Router("/crossdomain.xml", &controllers.CrossDomainController{})
	beego.Router("/shao", &controllers.ShaoHlsController{})

	beego.Router("/hls/*", &controllers.HlsController{})
}
