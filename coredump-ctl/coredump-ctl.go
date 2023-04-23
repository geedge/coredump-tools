package main

import (
	"coredump-handler/types"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

var configs []types.Coredump_config

func terminalSize() (width, height int, err error) {
	file, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	width, height, err = terminal.GetSize(int(file.Fd()))
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}
func debug(config types.Coredump_config) error {
	// 使用kubectl命令执行debug操作
	if config.Image_id != "" {
		if config.Hostname == "" {
			return errors.New("hostname must not be empty")
		}
		cmd := exec.Command("kubectl", "debug", "node/"+config.Hostname, "-it", "--image="+config.Image_id, "--image-pull-policy=Never", "-- bash")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println(cmd.String())
		input := fmt.Sprintf("you can exec   gdb %s /host%s get stack info", config.Process_exe_path, config.Storage)
		fmt.Println(input)
		if err := cmd.Start(); err != nil {
			return err
		}
		err := cmd.Wait()
		if err != nil {
			return err
		}
	} else { // 使用gdb命令进行命令行交互分析
		cmd := exec.Command("gdb", config.Process_exe_path, config.Storage)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println(cmd.String())
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// WalkDirectory 遍历文件目录并处理后缀名为.config的文件
func WalkDirectory(dir string) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), ".config") {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				var config types.Coredump_config
				err = json.Unmarshal(data, &config)
				if err != nil {
					return err
				}
				configs = append(configs, config)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory %s: %v\n", dir, err)
	}
}
func list() {
	defaultWidth := 80
	if width, _, err := terminalSize(); err == nil {
		defaultWidth = int(width)
	}
	// 输出表头信息
	fmt.Printf("%-6s %-6s %-6s %-15s %-20s %-15s %-10s %-20s %-20s\n",
		"PID", "UID", "GID", "SIGNATURE", "TIMESTAMP", "FILE", "HOSTNAME", "IMAGE_NAME", "storge")
	fmt.Println(strings.Repeat("-", defaultWidth))

	// 输出配置信息
	for _, c := range configs {
		processExe := filepath.Base(c.Process_exe_path)
		coreTime := time.Unix(c.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Printf("%-6s %-6s %-6s %-15d %-20s %-15s %-10s %-20s %-20s\n",
			c.Process_ns_pid, c.UID, c.GID, c.Signal, coreTime, processExe, c.Hostname, c.Image_id, c.Storage)
	}
	fmt.Println()
	fmt.Println("Total", len(configs), "coredumps")
}

// Help 打印使用帮助
func Help() {
	fmt.Println("Usage: coredump-ctl [list | debug | help] [options] ")
	fmt.Println("list options:")
	fmt.Println("  -dir string")
	fmt.Println("        Directory path")
}

func main() {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	debugCmd := flag.NewFlagSet("debug", flag.ExitOnError)
	helpCmd := flag.NewFlagSet("help", flag.ExitOnError)

	dirPath := flag.String("dir", "", "Directory path")

	if len(os.Args) == 1 {
		helpCmd.Usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "list":
		listCmd.Parse(os.Args[2:])
	case "debug":
		debugCmd.Parse(os.Args[2:])
	case "help":
		helpCmd.Parse(os.Args[2:])
		Help()
	default:
		fmt.Printf("%q is not a valid command.\n", os.Args[1])
		os.Exit(1)
	}
	if listCmd.Parsed() {
		if *dirPath == "" {
			*dirPath = "/var/tmp"
		}
		WalkDirectory(*dirPath)
		list()
		return
	}
	if debugCmd.Parsed() {
		if *dirPath == "" {
			*dirPath = "/var/tmp"
		}
		WalkDirectory(*dirPath)
		fmt.Println(configs[0])
		if err := debug(configs[0]); err != nil {
			fmt.Println(err)
		}
		return
	}
}
