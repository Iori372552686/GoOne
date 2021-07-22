package error_capture

import (
	"fmt"
	"runtime"
	"time"

	`GoOne/common/module/datetime`

	"github.com/getsentry/sentry-go"


)

type sentryConfig struct {
	Dsn         string
	Environment string
	ServerName  string
	Release     string
	PlayId      string
}

var sentryCnf = sentryConfig{}

func InitSentry(dsn, serviceVersion, serviceName, release, playId string) error {
	sentryCnf.Dsn = dsn
	sentryCnf.Environment = serviceVersion
	sentryCnf.ServerName = serviceName
	sentryCnf.Release = release
	sentryCnf.PlayId = playId
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: serviceVersion,
		ServerName:  serviceName,
		Release:     release,
	})
	sentry.Flush(time.Second * 5)
	return err
}

func CaptureErrorToSentry(level sentry.Level, userId, userName, errTile string, v ...interface{}) {
	buf := make([]byte, 4096)
	buf = buf[:runtime.Stack(buf, false)]
	stack := fmt.Sprintf("%s", buf)
	errMsg := fmt.Sprintf(errTile, v...)

	event := sentry.NewEvent()
	event.Level = level
	event.Message = errTile
	event.Logger = errMsg
	event.Timestamp = datetime.NowMs()
	event.User.ID = userId
	event.User.Username = userName
	if event.User.ID == "" {
		event.User.ID = sentryCnf.PlayId
		event.User.Username = sentryCnf.ServerName
	}

	event.Breadcrumbs = []*sentry.Breadcrumb{
		&sentry.Breadcrumb{
			Level:     level,
			Message:   fmt.Sprintf("\n%s\n%s", errMsg, stack),
			Timestamp: datetime.NowMs(),
		},
	}

	sentry.CaptureEvent(event)
}
