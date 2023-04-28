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
func debug(config types.Coredump_config, command string) error {
	// 使用kubectl命令执行debug操作
	if config.Image_id != "" {
		// 定义要运行的 kubectl 命令
		command := "kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name} {end}'"

		// 在 shell 中运行 kubectl 命令并读取输出
		cmd := exec.Command("bash", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			return err
		}

		// 将输出字符串按空格拆分为字符串切片
		nodes := strings.Split(strings.TrimSpace(string(output)), " ")
		if len(nodes) == 0 {
			return errors.New("the node list is empty")
		}
		if command == "gdb" {
			chmod := "cd" + filepath.Dir(config.Process_exe_path)
			cmd := exec.Command("kubectl", "debug", "node/"+nodes[0], "-it", "--image="+config.Image_id, "--image-pull-policy=Never", "--", "gdb", config.Process_exe_path, "/host"+config.Storage, "-ex", chmod)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			fmt.Println(cmd.String())
			if err := cmd.Start(); err != nil {
				return err
			}
			err := cmd.Wait()
			if err != nil {
				return err
			}
		} else {
			cmd := exec.Command("kubectl", "debug", "node/"+config.Hostname, "-it", "--image="+config.Image_id, "--image-pull-policy=Never", "--", command)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			fmt.Println(cmd.String())
			fmt.Println("exe=", config.Process_exe_path, "coredump=/host", config.Storage)
			if err := cmd.Start(); err != nil {
				return err
			}
			err := cmd.Wait()
			if err != nil {
				return err
			}
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
func list(pid string) {
	defaultWidth := 80
	if width, _, err := terminalSize(); err == nil {
		defaultWidth = int(width)
	}
	// 输出表头信息
	fmt.Printf("%-8s %-3s %-3s %-3s %-20s %-30s %-30s %-100s %-50s\n",
		"PID", "UID", "GID", "SIG", "TIMESTAMP", "EXE", "HOSTNAME", "IMAGE", "STORGE")
	fmt.Println(strings.Repeat("-", defaultWidth))
	total := 0
	// 输出配置信息
	for _, c := range configs {
		if pid != "" && strings.Compare(c.Initial_ns_pid, pid) != 0 {
			continue
		}
		coreTime := time.Unix(c.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Printf("%-8s %-3s %-3s %-6d %-20s %-30s %-30s %-100s %-50s\n",
			c.Initial_ns_pid, c.UID, c.GID, c.Signal, coreTime, c.Process_exe_path, c.Hostname, c.Image_id, c.Storage)
		total += 1
	}
	fmt.Println()
	fmt.Println("Total", total, "coredumps")
}

// Help 打印使用帮助
func Help() {
	fmt.Println("Usage: coredump-ctl [list | debug | help] [options] ")
	fmt.Println("list options:")
	fmt.Println("  -dir string")
	fmt.Println("        Directory path")
	fmt.Println("  -p string")
	fmt.Println("        Pid to matching")
	fmt.Println("  -command string")
	fmt.Println("        exe command when attach to pod")
}

func main() {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	debugCmd := flag.NewFlagSet("debug", flag.ExitOnError)
	helpCmd := flag.NewFlagSet("help", flag.ExitOnError)
	var dirPath, pid, command string
	listCmd.StringVar(&pid, "p", "", "Pid to matching")
	debugCmd.StringVar(&pid, "p", "", "Pid to matching")
	listCmd.StringVar(&dirPath, "dir", "", "Directory path")
	debugCmd.StringVar(&dirPath, "dir", "", "Directory path")
	debugCmd.StringVar(&command, "command", "", "")
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
		if dirPath == "" {
			dirPath = "/var/tmp"
		}
		WalkDirectory(dirPath)
		list(pid)
		return
	}
	if debugCmd.Parsed() {
		if dirPath == "" {
			dirPath = "/var/tmp"
		}
		if command == "" {
			command = "gdb"
		}
		WalkDirectory(dirPath)
		for _, config := range configs {
			fmt.Println(config)
			if strings.Compare(config.Initial_ns_pid, pid) == 0 || pid == "" {
				err := debug(config, command)
				if err != nil {
					fmt.Println(err)
				} else {
					break
				}
			}
		}
		return
	}
}
