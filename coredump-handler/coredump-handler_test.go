package main

import (
	"bufio"
	"bytes"
	"coredump-handler/types"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	// 初始化测试数据
	coredump_config.Initial_ns_pid = "123"
	coredump_config.Process_ns_pid = "456"
	coredump_config.Process_exe_path = "/usr/bin/test"
	coredump_config.Corepipe_config_path = "/etc/test/config.json"
	coredump_config.Timestamp = 1615478400
	coredump_config.GID = "1000"
	coredump_config.Hostname = "localhost"
	coredump_config.Signal = 11
	coredump_config.UID = "1000"

	// 执行测试
	retCode := m.Run()

	// 清理测试数据
	os.Remove("123_456_1615478400_coredump.info")
	os.Remove("123_456_1615478400_coredump.info.zip")
	os.Remove("123_456_1615478400_coredump.config")

	os.Exit(retCode)
}

func TestArgsJudge(t *testing.T) {
	err := argsJudge()
	if err == nil {
		t.Errorf("argsJudge() error = %v; want err", err)
	}
}

func TestIsDiskSufficient(t *testing.T) {
	pipe_config := types.Pipeconfig{
		Total_file_mem_limit: "50%",
	}
	res, err := isDiskSufficient(pipe_config)
	if err != nil || !res {
		t.Errorf("isDiskSufficient() error = %v; want res = true", err)
	}
}

func TestCreateCoreDumpDir(t *testing.T) {
	pipe_config := types.Pipeconfig{
		File_base_path: "/tmp",
	}
	err := createCoreDumpDir(&pipe_config, coredump_config)
	if err != nil {
		t.Errorf("createCoreDumpDir() error = %v; want err = nil", err)
	}
}

func TestChangeDirectory(t *testing.T) {
	err := changeDirectory("/tmp")
	if err != nil {
		t.Errorf("changeDirectory() error = %v; want err = nil", err)
	}
}

func TestGetContainerId(t *testing.T) {
	container_id, err := getContainerId("1")
	if err == nil && len(container_id) == 0 {
		t.Errorf("getContainerId() error = %v; want container_id != \"\"", err)
	}
}

func TestGetImageId(t *testing.T) {
	image_id, err := getImageId("1234567890abcdef", "/var/run/containerd.sock")
	if err != nil || len(image_id) == 0 {
		t.Errorf("getImageId() error = %v; want image_id != \"\"", err)
	}
}

func TestWriteCoreConfig(t *testing.T) {
	err := writeCoreConfig(coredump_config)
	if err != nil {
		t.Errorf("writeCoreConfig() error = %v; want err = nil", err)
	}
}

func TestWriteCoreDumpToFile(t *testing.T) {
	config := types.Coredump_config{
		Initial_ns_pid:       "1",
		Process_ns_pid:       "2",
		Process_exe_path:     "/bin/bash",
		Corepipe_config_path: "/tmp/config.yaml",
		Timestamp:            12345678,
	}
	cmd := exec.Command("echo", "test")
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		t.Errorf("WriteCoreDumpToFile() error = %v", err)
	}
	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("reading output: %s\n", scanner.Text())
		}
	}()
	err = cmd.Start()
	if err != nil {
		t.Errorf("WriteCoreDumpToFile() error = %v", err)
	}
	err = writeCoreDumpToFile(config)
	if err != nil {
		t.Errorf("WriteCoreDumpToFile() error = %v", err)
	}
	b, err := ioutil.ReadFile("coredump.info")
	if err != nil {
		t.Errorf("WriteCoreDumpToFile() error = %v", err)
	}
	expected := "test\n"
	if string(b) != expected {
		t.Errorf("WriteCoreDumpToFile() got = %v, want = %v", string(b), expected)
	}
	defer os.Remove("coredump.info")
}

func TestCompress(t *testing.T) {
	config := types.Coredump_config{
		Initial_ns_pid:       "1",
		Process_ns_pid:       "2",
		Process_exe_path:     "/bin/bash",
		Corepipe_config_path: "/tmp/config.yaml",
		Timestamp:            12345678,
	}
	cmd := exec.Command("echo", "test")
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		t.Errorf("compress() error = %v", err)
	}
	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			fmt.Printf("reading output: %s\n", scanner.Text())
		}
	}()
	err = cmd.Start()
	if err != nil {
		t.Errorf("compress() error = %v", err)
	}
	err = compress(config)
	if err != nil {
		t.Errorf("compress() error = %v", err)
	}
	b, err := ioutil.ReadFile("coredump.info.zip")
	if err != nil {
		t.Errorf("compress() error = %v", err)
	}
	expected := []byte{80, 75, 3, 4, 20, 0, 8, 8, 8, 0, 170, 114, 38, 99, 102, 0, 0, 0, 102, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 116, 101, 115, 116, 10, 0, 82, 105, 105, 90, 85, 50, 84, 110, 88, 70, 90, 65, 48, 108, 108, 81, 0, 116, 157, 80, 10, 128, 163, 239, 120, 53, 2, 190, 127, 7, 98, 131, 137, 51, 189, 206, 149, 14, 54, 126, 254, 152, 119, 177, 209, 155, 201, 23, 37, 4, 72, 113, 39, 46, 179, 144, 106, 184, 44, 251, 47, 88, 97, 2, 141, 11, 129, 71, 109, 187, 124, 32, 63, 22, 111, 181, 59, 30, 58, 184, 40, 203, 205, 3, 113, 165, 117, 232, 6, 228, 240, 132, 94, 137, 43, 95, 218, 221, 90, 203, 173, 43, 92, 216, 226, 65, 118, 222, 208, 59, 185, 250, 56, 70, 63, 135, 72, 44, 250, 215, 59, 36, 139, 74, 75, 112, 0, 0, 0}
	if !bytes.Equal(b, expected) {
		t.Errorf("compress() got = %v, want = %v", b, expected)
	}
	defer os.Remove("coredump.info.zip")
}
