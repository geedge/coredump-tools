package main

import (
	"context"
	"coredump-tools/types"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var configs []types.Coredump_config

func terminalSize() (width, height int, err error) {
	file, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	width, height, err = term.GetSize(int(file.Fd()))
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}
func debug(config types.Coredump_config, command string) error {
	// 使用kubectl命令执行debug操作
	if config.Image_name != "NULL" {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}

		// Creates the kubernetes client using the specified kubeconfig
		conf, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}

		clientset, err := kubernetes.NewForConfig(conf)
		if err != nil {
			return err
		}
		podName, err := debugInpod(conf, clientset, config, command)
		if err != nil {
			fmt.Println(err)
		}
		// Delete the Pod
		fmt.Printf("Deleting Pod %q...\n", podName)
		err = clientset.CoreV1().Pods("default").Delete(context.Background(), podName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("Deleted Pod %q.\n", podName)
		return nil
	} else { // 使用gdb命令进行命令行交互分析
		cmd := exec.Command("gdb", config.Process_exe_path, config.Storage, "-cd", filepath.Dir(config.Process_exe_path))
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
func debugInpod(conf *rest.Config, clientset *kubernetes.Clientset, config types.Coredump_config, command string) (string, error) {
	// Define the Pod object
	id := uuid.New()
	fmt.Println(id.String())
	podName := fmt.Sprintf("node-debug-%s", id.String())
	containerName := "debug"
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "debug",
					Image:           config.Image_name,
					ImagePullPolicy: "IfNotPresent",
					Command: []string{
						"tail",
						"-f",
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "host-dir",
							MountPath: "/host",
						},
						{
							Name:      "lib-debuginfo-dir",
							MountPath: "/usr/lib/debug",
						},
						{
							Name:      "src-debuginfo-dir",
							MountPath: "/usr/src/debug",
						},
						{
							Name:      "mrzcpd",
							MountPath: "/opt/tsg/mrzcpd",
						},
					},
					SecurityContext: &v1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "host-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
				{
					Name: "lib-debuginfo-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/usr/lib/debug",
						},
					},
				},
				{
					Name: "src-debuginfo-dir",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/usr/src/debug",
						},
					},
				},
				{
					Name: "mrzcpd",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: "/opt/tsg/mrzcpd",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	// Create the Pod
	fmt.Println("Creating Pod...")
	fmt.Printf("Creating Pod %q...\n", podName)
	result, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return podName, err
	}
	fmt.Printf("Created Pod %q.\n", result.GetObjectMeta().GetName())

	// Wait for the Pod to be running and ready
	fmt.Printf("Waiting for Pod %q to be ready...\n", podName)
	ready := false
	for i := 0; i < 10; i++ {
		result, err := clientset.CoreV1().Pods("default").Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return podName, err
		}
		status := result.Status
		if status.Phase == v1.PodRunning && len(status.ContainerStatuses) > 0 && status.ContainerStatuses[0].Ready {
			ready = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !ready {
		return podName, errors.New("create pod timeout")
	}
	// Disable canonical mode and enable echo mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return podName, err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	// Create exec request
	var cmd []string
	if command == "gdb" {
		cmd = []string{"gdb", config.Process_exe_path, "/host" + config.Storage, "-cd", filepath.Dir(config.Process_exe_path)}
	} else {
		cmd = []string{command}
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").
		Name(podName).Namespace("default").
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// Create exec executor
	executor, err := remotecommand.NewSPDYExecutor(conf, "POST", req.URL())
	if err != nil {
		return podName, err
	}

	// Start exec
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})
	if err != nil {
		return podName, err
	}
	return podName, nil
}

// WalkDirectory 遍历文件目录并处理后缀名为.config的文件
func WalkDirectory(dir string) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), ".info") {
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
			c.Initial_ns_pid, c.UID, c.GID, c.Signal, coreTime, c.Process_exe_path, c.Hostname, c.Image_name, c.Storage)
		total += 1
	}
	fmt.Println()
	fmt.Println("Total", total, "coredumps")
}

// // Help 打印使用帮助
// func Help() {
// 	fmt.Println("Usage: coredump-ctl [list | debug | help] [options] ")
// 	fmt.Println("list options:")
// 	fmt.Println("  -dir string")
// 	fmt.Println("        Directory path")
// 	fmt.Println("  -p string")
// 	fmt.Println("        Pid to matching")
// 	fmt.Println("debug options:")
// 	fmt.Println("  -dir string")
// 	fmt.Println("        Directory path")
// 	fmt.Println("  -p string")
// 	fmt.Println("        Pid to matching")
// 	fmt.Println("  -command string")
// 	fmt.Println("        exe command when attach to pod")
// }

func main() {
	var (
		pid     string
		dirPath string
		command string
	)
	app := &cli.App{
		Name:  "coredump",
		Usage: "Manage coredump files in Kubernetes clusters",
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all coredump files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "pid",
						Aliases:     []string{"p"},
						Usage:       "Pid to match",
						Value:       "",
						Destination: &pid,
					},
					&cli.StringFlag{
						Name:        "dir",
						Usage:       "Directory path",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
				},
				Action: func(c *cli.Context) error {
					fmt.Println(dirPath)
					WalkDirectory(dirPath)
					list(pid)
					return nil
				},
			},
			{
				Name:  "debug",
				Usage: "Start a debugging session for a coredump",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "pid",
						Aliases:     []string{"p"},
						Usage:       "Pid to match",
						Value:       "",
						Destination: &pid,
					},
					&cli.StringFlag{
						Name:        "dir",
						Usage:       "Directory path",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
					&cli.StringFlag{
						Name:        "command",
						Usage:       "Debugger command (default: gdb)",
						Value:       "gdb",
						Destination: &command,
					},
				},
				Action: func(c *cli.Context) error {
					WalkDirectory(dirPath)
					for _, config := range configs {
						if strings.Compare(config.Initial_ns_pid, pid) == 0 || pid == "" {
							err := debug(config, command)
							if err != nil {
								fmt.Println(err)
							} else {
								break
							}
						}
					}
					return nil
				},
			},
			{
				Name:  "help",
				Usage: "Show help message",
				Action: func(c *cli.Context) error {
					cli.ShowAppHelp(c)
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
