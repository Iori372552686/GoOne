/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package zap

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger  Logger
	logLock sync.RWMutex

	// currentLevel mirrors the active core level so hot paths can cheaply
	// check DebugEnabled() before boxing log arguments.
	currentLevel atomic.Int32

	// ZapLogger is kept for backward compatibility with existing integrations
	// (e.g. gin middleware expecting *zap.Logger).
	ZapLogger *zap.Logger
)

var levelMap = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"fatal": zapcore.FatalLevel,
}

type Config struct {
	Level            string
	LogFileName      string
	Sampling         *SamplingConfig
	LogRollingConfig *lumberjack.Logger
	LogDir           string
	CustomLogger     Logger
	LogStdout        bool
}

type SamplingConfig struct {
	Initial    int
	Thereafter int
	Tick       time.Duration
}

type NacosLogger struct {
	Logger
}

// Logger is the interface for Logger types
type Logger interface {
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Debug(args ...interface{})
	Fatal(args ...interface{})
	Sync() error

	Infof(fmt string, args ...interface{})
	Warnf(fmt string, args ...interface{})
	Errorf(fmt string, args ...interface{})
	Debugf(fmt string, args ...interface{})
	Fatalf(fmt string, args ...interface{})
}

func init() {
	// Default logger: stdout only.
	zlogger := newZapLogger(zapcore.InfoLevel, zapcore.AddSync(os.Stdout), 1)
	ZapLogger = zlogger
	currentLevel.Store(int32(zapcore.InfoLevel))
	setLogger(&NacosLogger{zlogger.Sugar()})
}

// DebugEnabled reports whether debug-level logs are active. Hot paths should
// guard verbose logging with this to avoid fmt boxing when debug is off.
func DebugEnabled() bool {
	return zapcore.Level(currentLevel.Load()) <= zapcore.DebugLevel
}

// InfoEnabled reports whether info-level logs are active.
func InfoEnabled() bool {
	return zapcore.Level(currentLevel.Load()) <= zapcore.InfoLevel
}

// InitLogger is init global logger for nacos
func InitLogger(config Config) (ins Logger, err error) {
	ins, err = initNacosLogger(config)
	if err != nil {
		return
	}

	setLogger(ins)
	return
}

// InitNacosLogger is init nacos default logger
func initNacosLogger(config Config) (Logger, error) {
	if config.CustomLogger != nil {
		return &NacosLogger{config.CustomLogger}, nil
	}

	logLevel := getLogLevel(config.Level)
	writer := config.getLogWriter()

	zlogger := newZapLogger(logLevel, writer, 2)
	ZapLogger = zlogger
	currentLevel.Store(int32(logLevel))
	return &NacosLogger{zlogger.Sugar()}, nil
}

func newZapLogger(level zapcore.Level, writer zapcore.WriteSyncer, callerSkip int) *zap.Logger {
	encCfg := newConsoleEncoderConfig()
	enc := zapcore.NewConsoleEncoder(encCfg)

	core := zapcore.NewCore(enc, writer, level)
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(callerSkip))
}

func newConsoleEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "time"
	cfg.LevelKey = "level"
	cfg.CallerKey = "caller"
	cfg.MessageKey = "msg"
	cfg.StacktraceKey = "stacktrace"
	cfg.FunctionKey = zapcore.OmitKey

	cfg.EncodeCaller = zapcore.ShortCallerEncoder
	cfg.EncodeDuration = zapcore.SecondsDurationEncoder
	cfg.EncodeTime = encodeLocalTimeMillis
	cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	return cfg
}

func getLogLevel(level string) zapcore.Level {
	if level == "" {
		return zapcore.InfoLevel
	}
	if zapLevel, ok := levelMap[strings.ToLower(level)]; ok {
		return zapLevel
	}
	return zapcore.InfoLevel
}

func encodeLocalTimeMillis(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// e.g. 2025-12-30 10:40:10.235
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// SetLogger sets logger for sdk
func setLogger(log Logger) {
	logLock.Lock()
	defer logLock.Unlock()
	logger = log
}

func GetLogger() Logger {
	logLock.RLock()
	defer logLock.RUnlock()
	return logger
}

// getLogWriter get Lumberjack writer by LumberjackConfig
func (c *Config) getLogWriter() zapcore.WriteSyncer {
	if c.LogRollingConfig == nil {
		c.LogRollingConfig = &lumberjack.Logger{}
	}
	c.LogRollingConfig.Filename = c.LogDir + string(os.PathSeparator) + c.LogFileName

	file := zapcore.AddSync(c.LogRollingConfig)
	if c.LogStdout {
		// Single writer that writes to both file and stdout (no tee in zap core -> no duplicates).
		return zapcore.NewMultiWriteSyncer(file, zapcore.AddSync(os.Stdout))
	}
	return file
}
