package main

import (
	"coredump-handler/logger"
	"coredump-handler/types"
	"net"
	"os/exec"
	"syscall"
)

func coredumpInit(coreconfig types.Coreconfig) error {
	cmd := exec.Command("sysctl", "-w", "kernel.core_pattern='"+coreconfig.Core_pattern+"'")
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("sysctl", "-w", "kernel.core_pipe_limit="+coreconfig.Core__pipe_limit)
	err = cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("sysctl", "-w", "kernel.suid_dumpable=2")
	err = cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("ulimit", "-c", coreconfig.Core_limited)
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func coredumpHandler(coreconfig types.Coreconfig) error {
	syscall.Unlink(coreconfig.Socket_path)
	laddr, err := net.ResolveUnixAddr("unix", coreconfig.Socket_path)
	if err != nil {
		return err
	}
	listener, err := net.ListenUnix("unix", laddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		c, err := listener.Accept()
		if err != nil {
			logger.Logger.Println(err)
		}
		go HandleServerConn(c)
	}
	return nil
}
func HandleServerConn(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 1024)
	nr, err := c.Read(buf)
	if err != nil {
		logger.Logger.Println(err)
		return
	}
	logger.Logger.Println(buf[0:nr])
}
