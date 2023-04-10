package main

import (
	"coredump-handler/config"
	"coredump-handler/logger"
)

func main() {
	err := logger.Init()
	if err != nil {
		return
	}
	logger.Logger.Println("logger init ok")
	coreconfig, err := config.Init()
	logger.Logger.Println("config init ok")
	if err != nil {
		logger.Logger.Println(err)
		return
	}
	err = coredumpInit(coreconfig)
	if err != nil {
		logger.Logger.Println(err)
		return
	}
	logger.Logger.Println("coredump init ok")
	coredumpHandler(coreconfig)
}
