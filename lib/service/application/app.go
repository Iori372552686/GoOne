package application

// mainloop boot,  如有问题飞书联系 to: Iori
import (
	"fmt"
	"os"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
)

type AppInterface interface {
	OnInit() error
	OnReload() error
	OnProc() bool // return: isIdle
	OnTick(lastMs, nowMs int64)
	OnExit()
}

type Application struct {
	appHandler AppInterface

	idleLoopCnt int

	tickInterval int64
	lastTickTime int64
}

var sig = make(chan os.Signal, 1)
var app Application

func Init(handler AppInterface) *Application {
	app.appHandler = handler
	err := app.appHandler.OnInit()
	if err != nil {
		fmt.Errorf("Initialized fail | ", err)
		os.Exit(1)
		return nil
	}

	app.tickInterval = 10

	SignalNotify()
	return &app
}

// 每秒执行多少帧
func (a *Application) SetTickInterval(interval int64) {
	if interval > 0 && interval < 1000 {
		a.tickInterval = interval
	}
}

func (a *Application) exit() {
	a.appHandler.OnExit()
}

func (a *Application) reload() error {
	return a.appHandler.OnReload()
}

func (a *Application) loopOnce() bool {
	return a.appHandler.OnProc()
}

func (a *Application) tick(lastMs, nowMs int64) {
	a.appHandler.OnTick(lastMs, nowMs)
}

func Run() {
	fmt.Println("-----------  SvrImpl  is  Runing ------------ ")

	for {
		app.checkSysSignal()
		datetime.Tick()
		nowMs := datetime.NowMs()

		if nowMs*app.tickInterval/1000 != app.lastTickTime*app.tickInterval/1000 {
			app.tick(app.lastTickTime, nowMs)
		}

		isIdle := app.loopOnce()
		if isIdle {
			app.idleLoopCnt += 1
		} else {
			app.idleLoopCnt = 0
		}

		if app.idleLoopCnt > 1000 {
			app.idleLoopCnt = 0
			time.Sleep(5 * time.Millisecond)
		}

		app.lastTickTime = nowMs
	}
}
