package main

import (
	"archive/zip"
	"context"
	"coredump-tools/config"
	"coredump-tools/types"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/shirou/gopsutil/disk"
)

var coredump_config types.Coredump_config

func argsJudge() error {

	if coredump_config.Initial_ns_pid == "" ||
		coredump_config.Process_ns_pid == "" ||
		coredump_config.Corepipe_config_path == "" ||
		coredump_config.Timestamp == 0 ||
		coredump_config.Process_exe_path == "" ||
		coredump_config.GID == "" ||
		coredump_config.Hostname == "" ||
		coredump_config.UID == "" ||
		coredump_config.Signal == -1 {
		err := fmt.Sprintf("Failed to initialize command line parameters. -P=%s -p=%s -E=%s -configpath=%s -t=%d -g=%s -h=%s -s=%d -u=%s",
			coredump_config.Initial_ns_pid,
			coredump_config.Process_ns_pid,
			coredump_config.Process_exe_path,
			coredump_config.Corepipe_config_path,
			coredump_config.Timestamp,
			coredump_config.GID,
			coredump_config.Hostname,
			coredump_config.Signal,
			coredump_config.UID)
		return errors.New(err)
	}
	return nil
}

// judge disk's space is sufficient or not
func isDiskSufficient(pipe_config types.Pipeconfig) (bool, error) {
	percent, err := strconv.ParseInt(pipe_config.Total_file_mem_limit[:len(pipe_config.Total_file_mem_limit)-1], 10, 32)
	if err != nil {
		return false, err
	}
	wd := pipe_config.Storage
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	diskinfo, err := disk.Usage(wd)
	if err != nil {
		return false, err
	}
	data, err := json.MarshalIndent(diskinfo, "", "  ")
	if err != nil {
		info := fmt.Sprintf("diskinfo:%s", string(data))
		journal.Print(journal.PriInfo, info)
	}
	usage := int64(math.Floor(diskinfo.UsedPercent + 0.5))
	info := fmt.Sprintf("usage:%d,limited:%d", usage, percent)
	journal.Print(journal.PriInfo, info)
	if usage >= percent {
		return false, nil
	}
	return true, nil
}

// create dir to storage coredump file and coredump info
func createCoreDumpDir(pipe_config *types.Pipeconfig, args types.Coredump_config) error {
	pipe_config.Storage = fmt.Sprintf("%s/coredump_%s_%s_%d", pipe_config.Storage, args.Initial_ns_pid, args.Process_ns_pid, args.Timestamp)
	dirName := pipe_config.Storage
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return err
		}
	} else {
		return errors.New("directory already exists")
	}
	return nil
}

// chmod to storage dir
func changeDirectory(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return nil
}

// get container id by search /proc dir
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

// get image name by container id
func getImageName(container_id string, sock_path string) (string, error) {
	// connect to containerd daemon
	client, err := containerd.New(sock_path)
	if err != nil {
		return "", err
	}
	defer client.Close()
	// according to container id to get container info
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	container, err := client.LoadContainer(ctx, container_id)
	if err != nil {
		return "", err
	}
	imageRef, err := container.Image(ctx)
	if err != nil {
		return "", err
	}
	return imageRef.Name(), nil
}

// write coredump info to file
func writeCoreConfig(config types.Coredump_config) error {
	filename := fmt.Sprintf("%s_%s_%d.info", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
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

// write coredump to file
func writeCoreDumpToFile(config types.Coredump_config) error {
	filename := fmt.Sprintf("%s_%s_%d.coredump", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// create a pipe to write file
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		io.Copy(writer, os.Stdin)
	}()

	io.Copy(file, reader)
	return nil
}
func writeMiniDumpToFile(config types.Coredump_config) error {
	filename := fmt.Sprintf("%s_%s_%d.minidump", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		io.Copy(writer, os.Stdin)
	}()

	io.Copy(file, reader)
	return nil
}
func compress(config types.Coredump_config) error {
	// Create a new zip archive.
	filename := fmt.Sprintf("%s_%s_%d.coredump", config.Initial_ns_pid, config.Process_ns_pid, config.Timestamp)
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
	flag.StringVar(&coredump_config.Corepipe_config_path, "C", "", "configfile's path")
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
	info = fmt.Sprintf("initialize command line parameters. -P=%s -p=%s -E=%s -C=%s -t=%d -g=%s -h=%s -s=%d -u=%s", coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Process_exe_path, coredump_config.Corepipe_config_path, coredump_config.Timestamp, coredump_config.GID, coredump_config.Hostname, coredump_config.Signal, coredump_config.UID)
	journal.Print(journal.PriInfo, info)
	coredump_config.Process_exe_path = strings.Replace(coredump_config.Process_exe_path, "!", "/", -1)
	//Judge agrs is correct or not.
	err = argsJudge()
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//read config file and judge config info is correct or not.
	pipe_config, err := config.PipeInit(coredump_config.Corepipe_config_path)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	if pipe_config.Storage_type == 0 {
		return
	}
	//create source dir
	if _, err := os.Stat(pipe_config.Storage); os.IsNotExist(err) {
		if err := os.MkdirAll(pipe_config.Storage, os.ModePerm); err != nil {
			journal.Print(journal.PriErr, err.Error())
			return
		}
	}
	//judge disk usage
	flag, err := isDiskSufficient(pipe_config)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	if !flag {
		journal.Print(journal.PriErr, "Disk space exceeds limit")
		return
	}
	//create dir to storage coredump
	err = createCoreDumpDir(&pipe_config, coredump_config)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	coredump_config.Storage = pipe_config.Storage
	//chmod to coredump dir
	err = changeDirectory(pipe_config.Storage)
	if err != nil {
		journal.Print(journal.PriErr, err.Error())
		return
	}
	//find coredump process's container id
	container_id, err := getContainerId(coredump_config.Initial_ns_pid)
	//find image name
	if err == nil && len(container_id) != 0 {
		coredump_config.Container_id = container_id
		coredump_config.Image_name, err = getImageName(container_id, pipe_config.Containered_sock_path)
		if err != nil {
			journal.Print(journal.PriInfo, err.Error())
		}
	}
	if coredump_config.Container_id == "" {
		coredump_config.Container_id = "NULL"
	}
	if coredump_config.Image_name == "" {
		coredump_config.Image_name = "NULL"
	}
	//write coredump file
	if pipe_config.Compress == false {
		err = writeCoreDumpToFile(coredump_config)
		if err != nil {
			journal.Print(journal.PriErr, err.Error())
		}
		coredump_config.Storage = fmt.Sprintf("%s/%s_%s_%d.coredump", pipe_config.Storage, coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Timestamp)
	} else {
		err = compress(coredump_config)
		if err != nil {
			journal.Print(journal.PriErr, err.Error())
		}
		coredump_config.Storage = fmt.Sprintf("%s/%s_%s_%d.coredump.zip", pipe_config.Storage, coredump_config.Initial_ns_pid, coredump_config.Process_ns_pid, coredump_config.Timestamp)
	}
	//write coredump info
	err = writeCoreConfig(coredump_config)
	if err != nil {
		journal.Print(journal.PriInfo, err.Error())
	}
}
