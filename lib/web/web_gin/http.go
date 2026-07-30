package web_gin

import (
	"errors"
	"net"
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
	_, _, err := StartGin(conf, loadRouters)
	return err
}

// StartGin 创建 server，先 net.Listen 绑定端口成功后再启动 Serve goroutine，
// 使返回 nil 即代表端口已绑定且可服务（V3-P0-06：Listen-before-Serve）。
// 返回的 Serve 错误通过返回的 errCh 投递（首个非 ErrServerClosed 错误）。
func StartGin(conf Config, loadRouters func(router *gin.Engine)) (*http.Server, <-chan error, error) {
	srv, err := NewServer(conf, loadRouters)
	if err != nil {
		return nil, nil, err
	}
	// 先绑定端口：失败立即返回，不启动 Serve goroutine。
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, nil, err
	}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("Http Service Start error !! err=%v", err)
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	logger.Infof("------ Http Gin Server Running by %v ------", srv.Addr)
	return srv, serveErr, nil
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

	srv := &http.Server{
		Addr:    conf.IP + ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	// 应用超时与大小限制（V3-P0-06）。零值保留 http.Server 默认行为。
	srv.ReadHeaderTimeout = conf.ReadHeaderTimeout
	srv.ReadTimeout = conf.ReadTimeout
	srv.WriteTimeout = conf.WriteTimeout
	srv.IdleTimeout = conf.IdleTimeout
	if conf.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = conf.MaxHeaderBytes
	}
	return srv, nil
}
