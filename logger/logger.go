package logger

import (
	"log"
	"os"
)

var Logger *log.Logger

func Init() error {
	//创建输出日志文件
	logFile, err := os.Create("./main.log")
	if err != nil {
		return err
	}
	//创建一个Logger
	//参数1：日志写入目的地
	//参数2：每条日志的前缀
	//参数3：日志属性
	Logger = log.New(logFile, "coredump-handler_", log.Ldate|log.Ltime|log.Lshortfile)

	//SetFlags设置输出选项
	Logger.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return nil
}
