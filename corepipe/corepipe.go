package main

import (
	"archive/zip"
	"context"
	"coredump-handler/config"
	"coredump-handler/logger"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
)

const chunkSize = 1024 * 1024 * 1024 // 1GB

func judgeMemory(pipe_config config.Pipeconfig) (bool, error) {
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
func createCoreDumpDir(pipe_config *config.Pipeconfig, args []string) error {
	pipe_config.File_base_path = fmt.Sprintf("%s/%s_%s_%s", pipe_config.File_base_path, args[1], args[2], args[3])
	dirName := pipe_config.File_base_path
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
func Chdir(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return nil
}
func getContainerId(pid string) (string, error) {
	cgroup_path := fmt.Sprintf("/proc/%s/cgroup", pid)
	content, err := ioutil.ReadFile(cgroup_path)
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
		logger.Logger.Println(err)
		return "", err
	}
	defer client.Close()
	// 根据容器 ID 获取容器信息
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	container, err := client.LoadContainer(ctx, container_id)
	if err != nil {
		logger.Logger.Println(err)
		return "", err
	}
	// 获取容器关联的镜像信息
	imageRef, err := container.Image(ctx)
	if err != nil {
		logger.Logger.Println(err)
		return "", err
	}
	return imageRef.Name(), nil
}
func writeCoreConfig(data string) error {
	err := ioutil.WriteFile("coredump.config", []byte(data), 0666)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %d bytes to file\n", len(data))
	return nil
}
func writeCoreDump() error {
	file, err := os.Create("coredump.info")
	if err != nil {
		return err
	}
	defer file.Close()

	// 从标准输入流中读取数据，并将其写入文件中
	buf := make([]byte, 1024)
	for {
		n, err := io.ReadAtLeast(os.Stdin, buf, 1)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if _, err := file.Write(buf[:n]); err != nil {
			return err
		}
	}
	return nil
}
func compress() error {
	// Create a new zip archive.
	dest := "coredump.info"
	zipfile, err := os.Create(dest + ".zip")
	if err != nil {
		return err
	}
	defer zipfile.Close()

	// Create a new zip writer.
	zipwriter := zip.NewWriter(zipfile)
	defer zipwriter.Close()

	// Create a zip file header.
	header := &zip.FileHeader{
		Name:   dest,
		Method: zip.Deflate,
	}

	// Write the header to the zip file.
	writer, err := zipwriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Copy the dataStream to the zip file in chunks.
	for i := int64(0); ; i += chunkSize {
		// Calculate the size of this chunk.
		data := make([]byte, chunkSize)
		n, err := os.Stdin.Read(data)
		if err != nil && err != io.EOF {
			return err
		}

		// Compress this chunk.
		_, err = writer.Write(data[:n])
		if err != nil {
			return err
		}

		// Check if we've reached the end of the dataStream.
		if n < chunkSize {
			break
		}
	}

	return nil
}
func main() {
	err := Chdir("/root/pipe/corepipe")
	if err != nil {
		fmt.Println(err)
		return
	}
	pipe_config, err := config.PipeInit()
	if err != nil {
		fmt.Println(err)
		return
	}
	flag, err := judgeMemory(pipe_config)
	if err != nil && !flag {
		fmt.Println(err)
		return
	}
	if len(os.Args) < 4 {
		fmt.Println("parameter passing exception")
		return
	}
	err = createCoreDumpDir(&pipe_config, os.Args)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = Chdir(pipe_config.File_base_path)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = logger.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Logger.Println("logger init ok!")
	container_id, err := getContainerId(os.Args[1])
	var image_id string
	if err == nil && len(container_id) != 0 {
		image_id, err = getImageId(container_id, pipe_config.Containered_sock_path)
		if err != nil {
			logger.Logger.Println(err)
		}
	}
	err = writeCoreConfig(image_id)
	if err != nil {
		logger.Logger.Println(err)
	}
	if pipe_config.Save_model == 0 {
		err = writeCoreDump()
		if err != nil {
			logger.Logger.Println(err)
		}
	} else if pipe_config.Save_model == 1 {
		err = compress()
		if err != nil {
			logger.Logger.Println(err)
		}
	}
}
