package routers

import (
	"fmt"
	"github.com/astaxie/beego"
	"net/http"

	"platform/controllers"
	"strings"
	"time"
)

func monitorM3U8() {

	beego.CareStatFile = make(map[string]int64)
	for {

		now := time.Now().Unix()
		for k, v := range beego.CareStatFile {
			if !strings.HasSuffix(k, "m3u8") {
				continue
			}
			inter := (now - v)
			if inter > int64(15) {
				fmt.Println("Close File ", k)
				delete(beego.CareStatFile, k)

				resourcePath := strings.Split(k, ".")[0]
				resourceid := strings.Split(resourcePath, "/")[2]
				hlsRquest := &http.Client{}
				hlsRquest.Get("http://localhost:9998/close/real?" +
					"resourceid=" + resourceid)
				fmt.Println("resourceid ", resourceid)

			} else {
				fmt.Println("Close File ", inter)
			}
		}
		time.Sleep(10 * time.Second)
	}

}

func init() {
	go monitorM3U8()
	beego.Router("/", &controllers.MainController{})
	beego.Router("/example", &controllers.ExampleHlsController{})
	beego.Router("/crossdomain.xml", &controllers.CrossDomainController{})

	beego.Router("/hls/*", &controllers.HlsController{})
}
