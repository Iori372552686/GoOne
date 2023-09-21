// log
package log

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

// 检查文件或目录是否存在
// 如果由 filename 指定的文件或目录存在则返回 true，否则返回 false
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

//log句柄对象
type LogHandle struct {
	FirstTag    string      //输出标记字符
	Logger      *log.Logger //logger对象
	File        *os.File    //log对应的文件对象
	displayFile bool        //是否显示文件及行数
	sign        string      //左边字符标记
}

func (this *LogHandle) Init(firstTag string, displayFile bool, s string) {

	var err error
	//创建logdata文件夹
	_, err = os.Stat("log_data")
	if err != nil {
		os.MkdirAll("log_data", 0777)
	}

	//获取当前时间
	dataTime := time.Now()

	//构建文件路径
	filePath := fmt.Sprintf("log_data/%d_%02d_%02d_%s.txt",
		dataTime.Year(), dataTime.Month(), dataTime.Day(),
		firstTag)

	var file *os.File
	var bExitst bool

	//判断文件夹是否存在，不存在就创建一个
	//判断文件是否存在
	bExitst = Exist(filePath)
	if bExitst == true {
		file, err = os.OpenFile(filePath, os.O_APPEND|os.O_RDWR, 0)
		if err != nil {
			fmt.Println(err)
			fmt.Println("打开", filePath, "失败")
			return
		}

	} else {
		file, err = os.Create(filePath)
		if err != nil {
			fmt.Println(err)
			fmt.Println("创建", filePath, "失败")
			return
		}
	}

	this.File = file
	this.FirstTag = firstTag
	this.displayFile = displayFile
	this.sign = s
	this.Logger = log.New(file, "", log.LstdFlags|log.Lmicroseconds)
}

func (this *LogHandle) Log(v ...interface{}) {

	//获取函数调用文件及调用行数
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}

	//获取短文件名
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}

	//输出到控制台及日志文件
	if this.displayFile == true {
		this.Logger.Println(this.sign, this.FirstTag, this.sign, fmt.Sprint(v...),
			"[file:", short, "line:", line, "]")
		consloeLogger.Println(this.sign, this.FirstTag, this.sign, fmt.Sprint(v...),
			"[file:", short, "line:", line, "]")
	} else {
		this.Logger.Println(this.sign, this.FirstTag, this.sign, fmt.Sprint(v...))
		consloeLogger.Println(this.sign, this.FirstTag, this.sign, fmt.Sprint(v...))
	}

}

var infoLogHandle LogHandle
var errorLogHandle LogHandle
var waringLogHandle LogHandle
var consloeLogger *log.Logger

func Init() {
	infoLogHandle.Init("infors", false, "|")
	errorLogHandle.Init("errors", true, "*")
	waringLogHandle.Init("waring", true, "-")

	consloeLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
}

//输出普通信息
func Info(v ...interface{}) {
	infoLogHandle.Log(fmt.Sprint(v...))
}

//输出错误信息
func Error(v ...interface{}) {
	errorLogHandle.Log(fmt.Sprint(v...))
}

//输出警告信息
func Waring(v ...interface{}) {
	waringLogHandle.Log(fmt.Sprint(v...))
}
