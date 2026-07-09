package web_gin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	logzap "github.com/Iori372552686/GoOne/lib/api/logger/zap"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
)

/**
* @Description: Run gin start the server
* @param: http_port
* @param: mode
* @param: session_name
* @param: load_routers
* @return: error
* @Author: Iori
* @Date: 2022-02-28 11:27:27
**/
func RunGin(conf Config, loadRouters func(router *gin.Engine)) error {
	_, err := StartGin(conf, loadRouters)
	return err
}

func StartGin(conf Config, loadRouters func(router *gin.Engine)) (*http.Server, error) {
	srv, err := NewServer(conf, loadRouters)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("Http Service Start error !! err=%v", err)
		}
	}()
	logger.Infof("------ Http Gin Server Running by %v ------", srv.Addr)
	return srv, nil
}

func NewServer(conf Config, loadRouters func(router *gin.Engine)) (*http.Server, error) {
	if conf.Port <= 0 {
		return nil, errors.New("port args err!")
	}

	//mode
	switch conf.Mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(Cors())
	router.Use(
		ginzap.Ginzap(logzap.ZapLogger, time.RFC3339, true), // 使用 Zap 替换默认日志中间件
		ginzap.RecoveryWithZap(logzap.ZapLogger, true),      // 替换 gin.Recovery()
	)

	//loadRoutes
	loadRouters(router)

	return &http.Server{
		Addr:    conf.IP + ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}, nil
}
