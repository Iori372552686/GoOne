package rest

import (
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"fmt"

	"strings"
)

type Controller struct {
	Auth bool
	Data interface{}
}

//对未定义的路由规则进行处理
func NoRoute(ctx *gin.Context) {
	// todo
	//如果不存在则跳转出错页面
}

func NoMethod(ctx *gin.Context) {
	uri := ctx.Request.RequestURI
	fmt.Printf("NoMethod" + uri)
	uri = strings.TrimLeft(uri, "/")
	uri = strings.TrimSuffix(uri, ".shtml")
	//ctx.HTML(http.StatusOK, model+"/"+action+".html", gin.H{"title": "test"})
	ctx.HTML(200, uri+".html", "Q")
}
