package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Result(ctx *gin.Context, code int32, data interface{}, msg string) {
	ctx.JSON(http.StatusOK, gin.H{"code": code, "data": data, "msg": msg})
}

func ResultErrCode(ctx *gin.Context, code int32, data interface{}, msg string) {
	ctx.JSON(http.StatusOK, gin.H{"code": code, "data": data, "msg": msg})
}

func ResultOk(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": data, "msg": ""})
}
func ResultList(ctx *gin.Context, data interface{}, total int64) {
	ctx.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "rows": data, "msg": "", "total": total})
}
func ResultOkMsg(ctx *gin.Context, data interface{}, msg string) {
	ctx.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "data": data, "msg": msg})
}

func ResultFail(ctx *gin.Context, err interface{}) {
	ctx.JSON(http.StatusOK, gin.H{"code": http.StatusBadRequest, "data": nil, "msg": err})
}

func ResultFailData(ctx *gin.Context, data interface{}, err interface{}) {
	ctx.JSON(http.StatusOK, gin.H{"code": http.StatusBadRequest, "data": data, "msg": err})
}
