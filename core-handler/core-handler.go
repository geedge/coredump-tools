package main

//#cgo CXXFLAGS: -std=c++11
// #cgo CFLAGS: -I /root/breakpad/src/tools/linux/core_handler/coredump_handler_wrapper
// #cgo LDFLAGS: /root/breakpad/src/tools/linux/core_handler/coredump_handler_wrapper/coredump_handler_wrapper.so
//#include "/root/breakpad/src/tools/linux/core_handler/coredump_handler_wrapper/coredump_handler_wrapper.h"
import "C"
import "fmt"

func HandleCrash(pid int, procfsDir string, mdFilename string) bool {
	fmt.Println("start")
	return bool(C.HandleCrash(C.pid_t(pid), C.CString(procfsDir), C.CString(mdFilename)))
}
func main() {
	// 调用 coredump_handler_wrapper 动态链接库中的 HandleCrash 函数
	HandleCrash(123, "/proc/123", "/var/tmp")
}
