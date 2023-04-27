package main

import (
	"archive/zip"
	"context"
	"coredump-handler/config"
	"coredump-handler/types"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/coreos/go-systemd/v22/journal"
)

const chunkSize = 1024 * 1024 * 1024 // 1GB
var coredump_config types.Coredump_config

func argsJudge() error {
	if coredump_config.Initial_ns_pid == "" || coredump_config.Process_ns_pid == "" || coredump_config.Corepipe_config_path == "" || coredump_config.Timestamp == 0 || coredump_config.Process_exe_path == "" || coredump_config.GID == "" || coredump_config.Hostname == "" || coredump_config.UID == "" || coredump_config.Signal == -1 {
		err := fmt.Sprintf("Failed to initialize command line parameters. -P=%s -p=%s -E=%s -configpath=%s -t=%d -g=%s -h=%s -s=%d -u=%s", coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Process_exe_path, coredump_config.Corepipe_config_path, coredump_config.Timestamp, coredump_config.GID, coredump_config.Hostname, coredump_config.Signal, coredump_config.UID)
		return errors.New(err)
	}
	return nil
}
func isDiskSufficient(pipe_config types.Pipeconfig) (bool, error) {
	percent, err := strconv.ParseFloat(pipe_config.Total_file_mem_limit[:len(pipe_config.Total_file_mem_limit)-1], 64)
	if err != nil {
		return false, err
	}
	percent = percent / 100.0
	var stat syscall.Statfs_t
	wd, err := syscall.Getwd()
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	syscall.Statfs(wd, &stat)
	// 剩余空间的大小为块的数量 * 每个块的大小
	// stat.Bfree表示可用的块的数量，stat.Bsize表示每个块的大小
	usedSpace := (int64(stat.Blocks) - int64(stat.Bfree))
	totalSpace := int64(stat.Blocks)
	usage := float64(usedSpace) / float64(totalSpace)
	if usage >= percent {
		return false, nil
	}
	return true, nil
}
func createCoreDumpDir(pipe_config *types.Pipeconfig, args types.Coredump_config) error {
	pipe_config.Storage = fmt.Sprintf("%s/%s_%s_%d_coredump", pipe_config.Storage, args.Initial_ns_pid, args.Process_ns_pid, args.Timestamp)
	dirName := pipe_config.Storage
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		// 目录不存在，创建目录
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return err
		}
	} else {
		return errors.New("directory already exists")
	}
	return nil
}
func changeDirectory(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return nil
}
func getContainerId(pid string) (string, error) {
	cgroup_path := fmt.Sprintf("/proc/%s/cgroup", pid)
	content, err := os.ReadFile(cgroup_path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`([a-f\d]{64})`)
	match := re.FindStringSubmatch(string(content))
	if len(match) < 2 {
		return "", errors.New("failed to extract container ID from cgroup file")
	}
	containerID := match[1]
	return containerID, nil
}
func getImageId(container_id string, sock_path string) (string, error) {
	// 连接 containerd daemon
	client, err := containerd.New(sock_path)
	if err != nil {
		return "", err
	}
	defer client.Close()
	// 根据容器 ID 获取容器信息
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	container, err := client.LoadContainer(ctx, container_id)
	if err != nil {
		return "", err
	}
	// 获取容器关联的镜像信息
	imageRef, err := container.Image(ctx)
	if err != nil {
		return "", err
	}
	return imageRef.Name(), nil
}
func writeCoreConfig(config types.Coredump_config) error {
	filename := fmt.Sprintf("%s_%s_%d_coredump.config", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	err = encoder.Encode(&config)
	if err != nil {
		return err
	}
	return nil
}
func writeCoreDumpToFile(config types.Coredump_config) error {
	filename := fmt.Sprintf("%s_%s_%d_coredump.info", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 创建一个管道
	reader, writer := io.Pipe()
	// 将标准输入连接到管道的写入端
	go func() {
		defer writer.Close()
		io.Copy(writer, os.Stdin)
	}()

	// 从管道的读取端将内容写入文件
	io.Copy(file, reader)
	return nil
}
func compress(config types.Coredump_config) error {
	// Create a new zip archive.
	filename := fmt.Sprintf("%s_%s_%d_coredump.info", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
	zipfile, err := os.Create(filename + ".zip")
	if err != nil {
		return err
	}
	defer zipfile.Close()

	// Create a new zip writer.
	zipwriter := zip.NewWriter(zipfile)
	defer zipwriter.Close()

	// Create a zip file header.
	header := &zip.FileHeader{
		Name:   filename,
		Method: zip.Deflate,
	}

	// Write the header to the zip file.
	writer, err := zipwriter.CreateHeader(header)
	if err != nil {
		return err
	}
	io.Copy(writer, os.Stdin)

	return nil
}
func main() {
	info := fmt.Sprintf("start handle coredump")
	journal.Print(journal.PriInfo, info)
	flag.StringVar(&coredump_config.Initial_ns_pid, "P", "", "initial ns pid")
	flag.StringVar(&coredump_config.Process_ns_pid, "p", "", "process ns pid")
	flag.StringVar(&coredump_config.Process_exe_path, "E", "", "pathname of executable process")
	flag.StringVar(&coredump_config.Corepipe_config_path, "configpath", "", "configfile's path")
	flag.Int64Var(&coredump_config.Timestamp, "t", 0, "the time of coredump")
	flag.StringVar(&coredump_config.GID, "g", "", "Numeric real GID of dumped process.")
	flag.IntVar(&coredump_config.Signal, "s", -1, "Number of signal causing dump")
	flag.StringVar(&coredump_config.UID, "u", "", "Numeric real UID of dumped process.")
	flag.Parse()
	var err error
	coredump_config.Hostname, err = os.Hostname()
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
	}
	info = fmt.Sprintf("initialize command line parameters. -P=%s -p=%s -E=%s -configpath=%s -t=%d -g=%s -h=%s -s=%d -u=%s", coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Process_exe_path, coredump_config.Corepipe_config_path, coredump_config.Timestamp, coredump_config.GID, coredump_config.Hostname, coredump_config.Signal, coredump_config.UID)
	journal.Print(journal.PriInfo, info)
	coredump_config.Process_exe_path = strings.Replace(coredump_config.Process_exe_path, "!", "/", -1)
	//判断参数读取是否正确
	err = argsJudge()
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//读取config文件并初始化
	pipe_config, err := config.PipeInit(coredump_config.Corepipe_config_path)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//判断硬盘使用率
	flag, err := isDiskSufficient(pipe_config)
	if err != nil && !flag {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//创建存储coredump内容文件夹
	err = createCoreDumpDir(&pipe_config, coredump_config)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	coredump_config.Storage = pipe_config.Storage
	//切换至存储coredump目录
	err = changeDirectory(pipe_config.Storage)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//查找发生coredump进程对应的container id
	container_id, err := getContainerId(coredump_config.Initial_ns_pid)
	//根据查找到的container id查找对应的image name
	if err == nil && len(container_id) != 0 {
		coredump_config.Image_id, err = getImageId(container_id, pipe_config.Containered_sock_path)
		if err != nil {
			journal.Print(journal.PriInfo, err.Error())
		}
	}
	if coredump_config.Image_id == "" {
		coredump_config.Image_id = "NULL"
	}
	//根据配置项选择存储coredump文件方式
	if pipe_config.Compress == false {
		err = writeCoreDumpToFile(coredump_config)
		if err != nil {
			journal.Print(journal.PriErr, err.Error())
		}
		coredump_config.Storage = fmt.Sprintf("%s/%s_%s_%d_coredump.info", pipe_config.Storage, coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Timestamp)
	} else if pipe_config.Compress == true {
		err = compress(coredump_config)
		if err != nil {
			journal.Print(journal.PriErr, err.Error())
		}
		coredump_config.Storage = fmt.Sprintf("%s/%s_%s_%d_coredump.info.zip", pipe_config.Storage, coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Timestamp)
	}
	//将image name写入coredump config
	err = writeCoreConfig(coredump_config)
	if err != nil {
		journal.Print(journal.PriInfo, err.Error())
	}
}
