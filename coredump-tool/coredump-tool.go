package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"coredump-tools/types"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexeyco/simpletable"
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

var (
	pid            string
	dirPath        string
	command        string
	prestartPath   string
	outFilePath    string
	importFilePath string
)

// WalkDirectory search file with suffix name ".info"

func WalkDirectory(dir string) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), "coredump_") {
			err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() && strings.HasSuffix(info.Name(), ".info") {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}

					var config types.Coredump_config
					err = json.Unmarshal(data, &config)
					if err != nil {
						return err
					}
					if config.Container_id == "" {
						config.Container_id = "NULL"
					}
					if config.Image_name == "" {
						config.Image_name = "NULL"
					}
					if _, err := os.Stat(config.Storage); err != nil {
						config.Storage = "MISS"
					}
					configs = append(configs, config)
				}

				return nil
			})

			if err != nil {
				fmt.Printf("Error walking directory %s: %v\n", path, err)
			}
		}

		return nil
	})
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Timestamp < configs[j].Timestamp
	})
	if err != nil {
		fmt.Printf("Error walking directory %s: %v\n", dir, err)
	}
}
func list(pid string) {
	table := simpletable.New()

	table.Header = &simpletable.Header{
		Cells: []*simpletable.Cell{
			{Text: "PID"},
			{Text: "EXE"},
			{Text: "IMAGE"},
			{Text: "TIMESTAMP"},
		},
	}
	total := 0
	// output the config's info
	for _, c := range configs {
		if pid != "" && strings.Compare(c.Initial_ns_pid, pid) != 0 {
			continue
		}
		coreTime := time.Unix(c.Timestamp, 0).Format("2006-01-02 15:04:05")
		r := []*simpletable.Cell{
			{Text: c.Initial_ns_pid},
			{Text: c.Process_exe_path},
			{Text: c.Image_name},
			{Text: coreTime},
		}
		table.Body.Cells = append(table.Body.Cells, r)
		total += 1
	}
	fmt.Println(table.String())
	fmt.Println("Total", total, "coredumps")
}

func info(pid string) {
	total := 0
	// output the config's info
	for _, c := range configs {
		if pid != "" && strings.Compare(c.Initial_ns_pid, pid) != 0 {
			continue
		}
		coreTime := time.Unix(c.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Println("PID: " + c.Initial_ns_pid)
		fmt.Println("UID: " + c.UID)
		fmt.Println("GID: " + c.GID)
		fmt.Println("SIG: " + strconv.Itoa(c.Signal))
		fmt.Println("EXE: " + c.Process_exe_path)
		fmt.Println("POD: " + c.Pod_name)
		fmt.Println("IMAGE: " + c.Image_name)
		fmt.Println("HOSTNAME: " + c.Hostname)
		fmt.Println("STORAGE: " + c.Storage)
		fmt.Println("TIMESTAMP: " + coreTime)
		fmt.Println()
		fmt.Println()
		total += 1
	}
	fmt.Println("Total", total, "coredumps")
}

func getImageVersion() (string, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VERSION=") {
			version := strings.TrimPrefix(line, "VERSION=")
			version = strings.Trim(version, "\"")
			imageVersion := strings.Split(version, " ")
			fmt.Println("imageVersion:", imageVersion[0])
			if len(imageVersion[0]) > 0 {
				return imageVersion[0], nil
			}
			break
		}
	}
	return "", errors.New("get empty version info")
}

func debug(config types.Coredump_config, command, prestartPath string) error {
	if strings.HasSuffix(config.Storage, ".minidump") {
		corefile := strings.Replace(config.Storage, ".minidump", ".coredump", -1)
		cmd := exec.Command("minidump-2-core", "-o", corefile, config.Storage)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return (errors.New(err.Error() + ":" + stderr.String()))
		}
		config.Storage = corefile
		defer os.Remove(corefile)
	}
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
	podName, err := debugInpod(conf, clientset, config, command, prestartPath)
	if err != nil {
		fmt.Println(err)
	}
	// Delete the Pod
	if podName != "" {
		fmt.Printf("Deleting Pod %q...\n", podName)
		err = clientset.CoreV1().Pods("default").Delete(context.Background(), podName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("Deleted Pod %q.\n", podName)
	}
	return nil
}
func debugInpod(conf *rest.Config, clientset *kubernetes.Clientset, config types.Coredump_config, command, prestartPath string) (string, error) {
	// Define the Pod object
	id := uuid.New()
	fmt.Println(id.String())
	podName := fmt.Sprintf("coredump-debug-%s", id.String())
	containerName := "debug"
	volumeMounts := []v1.VolumeMount{
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
	}
	volumes := []v1.Volume{
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
	}
	var containerCommands []string
	var containerCommand string
	var gdbcommand []string
	var image string
	if _, err := os.Stat(config.Binary_file); err != nil {
		fmt.Println(err)
		containerCommand = "tail -f"
		gdbcommand = []string{"gdb", config.Process_exe_path, "/host" + config.Storage, "-cd", filepath.Dir(config.Process_exe_path)}
		image = config.Image_name
	} else {
		containerCommand = fmt.Sprintf("mkdir -p /tmp/sysroot && cd /tmp/sysroot ; unzip -o /host%s ; ldconfig -r /tmp/sysroot ; tail -f ", config.Binary_file)
		gdbcommand = []string{"gdb", "/tmp/sysroot" + config.Process_exe_path, "/host" + config.Storage, "-cd", filepath.Dir("/tmp/sysroot" + config.Process_exe_path), "-iex", "set sysroot /tmp/sysroot", "-iex", "set solib-search-path /tmp/sysroot/usr/lib:/tmp/sysroot/usr/lib64:/tmp/sysroot/opt/tsg/mrzcpd/icelake-server/lib:/tmp/sysroot/opt/tsg/mrzcpd/corei7/lib:/tmp/sysroot/opt/tsg/mrzcpd/znver1/lib"}
		imageversion, err := getImageVersion()
		if err != nil {
			return "", err
		}
		image = fmt.Sprintf("registry.gdnt-cloud.website/tsg/os/base:%s", imageversion)
	}
	if prestartPath != "" {
		prestartVolumeMount := v1.VolumeMount{
			Name:      "prestart-dir",
			MountPath: "/prestart.sh",
		}
		prestartVolume := v1.Volume{
			Name: "prestart-dir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: prestartPath,
				},
			},
		}
		volumeMounts = append(volumeMounts, prestartVolumeMount)
		volumes = append(volumes, prestartVolume)
		containerCommands = []string{
			"sh",
			"-c",
			"chmod +x /prestart.sh && /prestart.sh && " + containerCommand,
		}
	} else {
		containerCommands = []string{
			"sh",
			"-c",
			containerCommand,
		}
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "debug",
					Image:           image,
					ImagePullPolicy: "IfNotPresent",
					Command:         containerCommands,
					VolumeMounts:    volumeMounts,
					SecurityContext: &v1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
				},
			},
			Volumes:       volumes,
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	// Create the Pod
	fmt.Println("Creating Pod...")
	fmt.Printf("Creating Pod %q...\n", podName)
	result, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
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
		time.Sleep(3 * time.Second)
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
		cmd = gdbcommand
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

func importCoredump(tarGzFile, destDir string) {
	file, err := os.Open(tarGzFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		fmt.Printf("can not create gzip.Reader: %v", err)
		return
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("read tar file's head failed: %v", err)
			return
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				fmt.Printf("create directory %s failed: %v", target, err)
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				fmt.Printf("create file %s failed: %v", target, err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				fmt.Printf("write file %s failed: %v", target, err)
			}
		default:
			fmt.Printf("a unsupport file type: %v", header.Typeflag)
		}
	}

}

func export(source string, target string) {

	dir := filepath.Base(source)
	if target == "" {
		target = dir + ".tar.gz"
	}
	tarfile, err := os.Create(target)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer tarfile.Close()

	gzipWriter := gzip.NewWriter(tarfile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(filepath.Join(dir, strings.TrimPrefix(path, source)))
		if info.IsDir() {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	fmt.Println(target + " tarball created successfully!")
}

func command_init() cli.App {
	return cli.App{
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
						Aliases:     []string{"d"},
						Usage:       "Coredump directory path(default: /var/lib/coredump)",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
				},
				Action: func(c *cli.Context) error {
					WalkDirectory(dirPath)
					list(pid)
					return nil
				},
			},
			{
				Name:    "info",
				Aliases: []string{"i"},
				Usage:   "Display all coredump files's info",
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
						Aliases:     []string{"d"},
						Usage:       "Coredump directory path(default: /var/lib/coredump)",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
				},
				Action: func(c *cli.Context) error {
					WalkDirectory(dirPath)
					info(pid)
					return nil
				},
			},
			{
				Name:  "import",
				Usage: "import coredump file from an archive",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "dir",
						Aliases:     []string{"d"},
						Usage:       "Coredump directory path(default: /var/lib/coredump)",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
					&cli.StringFlag{
						Name:        "file",
						Aliases:     []string{"f"},
						Usage:       "archive file path",
						Value:       "",
						Destination: &importFilePath,
					},
				},
				Action: func(c *cli.Context) error {
					if importFilePath == "" {
						fmt.Println("Please using -f to indicate the archive file path.Also you can use --help to check all the command")
						return nil
					}
					importCoredump(importFilePath, dirPath)
					return nil
				},
			},
			{
				Name:  "export",
				Usage: "store coredump file in an archive",
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
						Aliases:     []string{"d"},
						Usage:       "Coredump directory path(default: /var/lib/coredump)",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
					&cli.StringFlag{
						Name:        "output",
						Aliases:     []string{"o"},
						Usage:       "store file path",
						Value:       "",
						Destination: &outFilePath,
					},
				},
				Action: func(c *cli.Context) error {
					if pid == "" {
						export(dirPath, outFilePath)
					} else {
						WalkDirectory(dirPath)
						for _, c := range configs {
							if strings.Compare(c.Initial_ns_pid, pid) == 0 {
								dirPath, _ := filepath.Split(c.Storage)
								export(dirPath, outFilePath)
							}
						}
					}
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
						Aliases:     []string{"d"},
						Usage:       "Coredump directory path(default: /var/lib/coredump)",
						Value:       "/var/lib/coredump",
						Destination: &dirPath,
					},
					&cli.StringFlag{
						Name:        "command",
						Aliases:     []string{"c"},
						Usage:       "Debugger command (default: gdb)",
						Value:       "gdb",
						Destination: &command,
					},
					&cli.StringFlag{
						Name:        "prestart",
						Aliases:     []string{"P"},
						Usage:       "Prestart script path just used in Pod's coredump debug",
						Value:       "",
						Destination: &prestartPath,
					},
				},
				Action: func(c *cli.Context) error {
					if prestartPath != "" {
						_, err := os.Stat(prestartPath)
						if err != nil {
							if os.IsNotExist(err) {
								fmt.Println("prestartfile is not exist!")
							} else {
								fmt.Println(err)
							}
							return nil
						}
					}
					WalkDirectory(dirPath)
					for _, config := range configs {
						if strings.Compare(config.Initial_ns_pid, pid) == 0 || pid == "" {
							err := debug(config, command, prestartPath)
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
}

func main() {
	app := command_init()
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
