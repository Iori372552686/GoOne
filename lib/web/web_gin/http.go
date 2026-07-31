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
// 使返回 nil 即代表端口已绑定且可服务（Listen-before-Serve）。
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
	// 请求体上限。在路由绑定前注册，使超大请求在业务读取前即返回 413，
	// 内存不随输入无限增长。仅当 MaxBodyBytes>0 时启用。
	if conf.MaxBodyBytes > 0 {
		router.Use(maxBodyBytesMiddleware(conf.MaxBodyBytes))
	}

	//loadRoutes
	loadRouters(router)

	srv := &http.Server{
		Addr:    conf.IP + ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	// 应用超时与大小限制。零值保留 http.Server 默认行为。
	srv.ReadHeaderTimeout = conf.ReadHeaderTimeout
	srv.ReadTimeout = conf.ReadTimeout
	srv.WriteTimeout = conf.WriteTimeout
	srv.IdleTimeout = conf.IdleTimeout
	if conf.MaxHeaderBytes > 0 {
		srv.MaxHeaderBytes = conf.MaxHeaderBytes
	}
	return srv, nil
}

// maxBodyBytesMiddleware 用 http.MaxBytesReader 包裹请求体，超限返回 413
// （输入保护，在业务读取前拒绝超大 Body）。
//
// handler 在读取时遇到 *http.MaxBytesError 会得到错误；本中间件在 Next 前已用
// LimitedReader 设限，超限的 Body 读取会在 handler 内以 io.EOF/ErrBodyReadAfterClose
// 形式失败，并由上层 Recovery 记录。为给出明确的 413 响应，这里主动校验。
func maxBodyBytesMiddleware(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}
