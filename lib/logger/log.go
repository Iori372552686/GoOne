package logger

import (
	"fmt"
	"runtime"
	"time"

	`GoOne/lib/error_capture`
	`GoOne/lib/logger/logs`

	"github.com/codeskyblue/dingrobot"
	"github.com/getsentry/sentry-go"
)

var beeLogger = logs.NewLogger()

func Init(nodeId int64, isConsole bool) {
	fpath := fmt.Sprintf("logs/%d", nodeId)
	filename := fpath + "/" + time.Now().Add(time.Second).Local().String()[0:10] + ".log"
	SetWriteFile(filename, isConsole)
	Info("initLog V1")

}

// Emergency logs a message at emergency level.
func Emergency(f string, v ...interface{}) {
	beeLogger.Emergency(f, v...)
}

// Alert logs a message at alert level.
func Alert(f string, v ...interface{}) {
	beeLogger.Alert(f, v...)
}

// Critical logs a message at critical level.
func Critical(f string, v ...interface{}) {
	beeLogger.Critical(f, v...)
}

// Error logs With UserInfo at error level.
func ErrorWithUserInfo(f string, userId, userName string, v ...interface{}) {
	beeLogger.Error(f, v...)
	error_capture.CaptureErrorToSentry(sentry.LevelError, userId, userName, f, v)
	error_capture.Error(userId, userName, f, v...)
}

// Error logs a message at error level.
// 调用此方法时请不要使用fmt方法
func Error(f string, v ...interface{}) {
	beeLogger.Error(f, v...)
	error_capture.CaptureErrorToSentry(sentry.LevelError, "", "", f, v...)
	error_capture.Error("", "", f, v...)
}

// 调用此方法时请不要使用fmt方法
func StackError(f string, v ...interface{}) {
	beeLogger.Error(f, v...)
	buf := make([]byte, 4096)
	buf = buf[:runtime.Stack(buf, false)]
	beeLogger.Info("%s", buf)

	error_capture.CaptureErrorToSentry(sentry.LevelError, "", "", f, v...)
	error_capture.Error("", "", f, v...)
}

// Warning logs a message at warning level.
func Warning(f string, v ...interface{}) {
	beeLogger.Warn(f, v...)
	buf := make([]byte, 4096)
	buf = buf[:runtime.Stack(buf, false)]
	event := sentry.NewEvent()
	event.Level = sentry.LevelWarning
	event.Message = fmt.Sprintf(f, v...) + fmt.Sprintf("%s", buf)
}

// Warn compatibility alias for Warning()
func Warn(f string, v ...interface{}) {
	beeLogger.Warn(f, v...)
}

// Notice logs a message at notice level.
func Notice(f string, v ...interface{}) {
	beeLogger.Notice(f, v...)
}

// Informational logs a message at info level.
func Informational(f string, v ...interface{}) {
	beeLogger.Info(f, v...)
}

// Info compatibility alias for Warning()
func Info(f string, v ...interface{}) {
	beeLogger.Info(f, v...)
}

// Debug logs a message at debug level.
func Debug(f string, v ...interface{}) {
	beeLogger.Debug(f, v...)
}

// Trace logs a message at trace level.
// compatibility alias for Warning()
func Trace(f string, v ...interface{}) {
	beeLogger.Trace(f, v...)
}

func Stack() []byte {
	buf := make([]byte, 4096)
	buf = buf[:runtime.Stack(buf, false)]
	beeLogger.Info("%s", buf)
	return buf
}

func SetWriteFile(filePath string, console bool) {
	beeLogger.Reset()
	if console {
		beeLogger.SetLogger("console")
	}
	config := `{"filename":"` + filePath + `","maxLines":500000,"daily":false,"maxDays":100,"rotate":true,"perm":"0600"}`
	beeLogger.SetLogger(logs.AdapterFile, config)
}

// 服务器警报
func AlertDingDing(f string, v ...interface{}) {
	beeLogger.Error(f, v...)
	error_capture.CaptureErrorToSentry(sentry.LevelFatal, "", "", f, v)
}

func AlertDingDingOnly(token string, f string, v ...interface{}) {
	robot := dingrobot.New(token)
	strMessage := "警报警报\n " + fmt.Sprintf(f, v...)
	robot.Text(strMessage)
}
