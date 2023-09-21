package rest

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var allAuth = make(map[string]int64)

// 将auth存储起来
func AllAuth(auth ...map[string]int64) map[string]int64 {
	if len(auth) > 0 {
		allAuth = auth[0]
		return nil
	} else {
		return allAuth
	}
}

// 将auth存储起来
var mapRoleAuth = make(map[int]map[string]int64)

// 鉴权接口
func Auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.RequestURI
		isapi := !strings.Contains(uri, ".shtml")
		ispage := !isapi
		auths := AllAuth()
		var exist = true

		_, exist = auths[uri]
		//如果不存在,说明这个是不需要做权限校验的
		if !exist {
			ctx.Next()
			return
		}

		if ispage {
			//ctx.HTML(http.StatusOK, "public/error.html", gin.H{"msg": "你没有权限进行该操作"})
			ctx.Abort()
		} else {
			ResultFail(ctx, "鉴权失败")
			ctx.Abort()
		}
		return
	}
}

/**
 * @Description: 字符串转md5
 * @param str 待转字符串
 * @return string md5
 **/
func md5V2(str string) string {
	data := []byte(str)
	has := md5.Sum(data)
	md5str := fmt.Sprintf("%x", has)
	return md5str
}

/**
* @Description: 允許跨域限制
* @return: gin.HandlerFunc
* @Author: Iori
* @Date: 2022-07-26 16:04:04
**/
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}
