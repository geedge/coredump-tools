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

	"github.com/stretchr/testify/assert"
)

func TestArgsJudge(t *testing.T) {
	var coredump_config types.Coredump_config
	coredump_config.Initial_ns_pid = "123"
	coredump_config.Process_ns_pid = "456"
	coredump_config.Process_exe_path = "/path/to/executable"
	coredump_config.Corepipe_config_path = "/path/to/config"
	coredump_config.Timestap = 1618332579

	err := argsJudge()
	assert.Nil(t, err)

	// missing Initial_ns_pid
	coredump_config.Initial_ns_pid = ""
	err = argsJudge()
	assert.NotNil(t, err)
	coredump_config.Initial_ns_pid = "123"

	// missing Process_ns_pid
	coredump_config.Process_ns_pid = ""
	err = argsJudge()
	assert.NotNil(t, err)
	coredump_config.Process_ns_pid = "456"

	// missing Corepipe_config_path
	coredump_config.Corepipe_config_path = ""
	err = argsJudge()
	assert.NotNil(t, err)
	coredump_config.Corepipe_config_path = "/path/to/config"

	// missing Timestap
	coredump_config.Timestap = 0
	err = argsJudge()
	assert.NotNil(t, err)
	coredump_config.Timestap = 1618332579

	// missing Process_exe_path
	coredump_config.Process_exe_path = ""
	err = argsJudge()
	assert.NotNil(t, err)
	coredump_config.Process_exe_path = "/path/to/executable"
}

func TestIsDiskSufficient(t *testing.T) {
	pipe_config := types.Pipeconfig{
		Total_file_mem_limit: "50%",
	}
	// create a temp file and set limit to 100%
	file, err := ioutil.TempFile("", "")
	assert.Nil(t, err)
	defer os.Remove(file.Name())
	pipe_config.File_base_path = file.Name()
	pipe_config.Total_file_mem_limit = "100%"
	flag, err := isDiskSufficient(pipe_config)
	assert.Nil(t, err)
	assert.False(t, flag)

	// set limit to 0%
	pipe_config.Total_file_mem_limit = "0%"
	flag, err = isDiskSufficient(pipe_config)
	assert.Nil(t, err)
	assert.True(t, flag)
}

func TestCreateCoreDumpDir(t *testing.T) {
	pipe_config := types.Pipeconfig{
		File_base_path: "/tmp",
	}
	err := createCoreDumpDir(&pipe_config, coredump_config)
	assert.Nil(t, err)

	// try to create same directory again should return an error
	err = createCoreDumpDir(&pipe_config, coredump_config)
	assert.NotNil(t, err)
}

func TestGetContainerId(t *testing.T) {
	pid := "1"
	container_id, err := getContainerId(pid)
	if err != nil {
		t.Errorf("getContainerId() error = %v", err)
	}
	if len(container_id) == 0 {
		t.Errorf("getContainerId() = %v, want not empty string", container_id)
	}
}

func TestGetImageId(t *testing.T) {
	container_id := "123456"
	sock_path := "/var/run/containerd.sock"
	image_id, err := getImageId(container_id, sock_path)
	if err != nil {
		t.Errorf("getImageId() error = %v", err)
	}
	if len(image_id) == 0 {
		t.Errorf("getImageId() = %v, want not empty string", image_id)
	}
}

func TestWriteCoreConfig(t *testing.T) {
	config := types.Coredump_config{
		Initial_ns_pid:       "1",
		Process_ns_pid:       "2",
		Process_exe_path:     "/bin/bash",
		Corepipe_config_path: "/tmp/config.yaml",
		Timestap:             12345678,
	}
	err := writeCoreConfig(config)
	if err != nil {
		t.Errorf("writeCoreConfig() error = %v", err)
	}
	defer os.Remove("coredump.config")
}

func TestWriteCoreDumpToFile(t *testing.T) {
	config := types.Coredump_config{
		Initial_ns_pid:       "1",
		Process_ns_pid:       "2",
		Process_exe_path:     "/bin/bash",
		Corepipe_config_path: "/tmp/config.yaml",
		Timestap:             12345678,
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
		Timestap:             12345678,
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
