package main

import (
	"bytes"
	"coredump-handler/config"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestIsMemorySufficient(t *testing.T) {
	type input struct {
		pipe_config config.Pipeconfig
	}
	tests := []struct {
		name    string
		input   input
		want    bool
		wantErr bool
	}{
		{
			name: "memory is sufficient",
			input: input{
				pipe_config: config.Pipeconfig{
					Total_file_mem_limit: "50%",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "memory is not sufficient",
			input: input{
				pipe_config: config.Pipeconfig{
					Total_file_mem_limit: "0.001%",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "invalid memory limit",
			input: input{
				pipe_config: config.Pipeconfig{
					Total_file_mem_limit: "invalid",
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isMemorySufficient(tt.input.pipe_config)
			if (err != nil) != tt.wantErr {
				t.Errorf("isMemorySufficient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isMemorySufficient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateCoreDumpDir(t *testing.T) {
	type input struct {
		pipe_config *config.Pipeconfig
		args        []string
	}
	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name: "directory does not exist",
			input: input{
				pipe_config: &config.Pipeconfig{
					File_base_path: "/tmp",
				},
				args: []string{"", "pid", "arg2", "arg3"},
			},
			wantErr: false,
		},
		{
			name: "directory already exists",
			input: input{
				pipe_config: &config.Pipeconfig{
					File_base_path: "/tmp",
				},
				args: []string{"", "pid", "arg2", "arg3"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := createCoreDumpDir(tt.input.pipe_config, tt.input.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("createCoreDumpDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChangeDirectory(t *testing.T) {
	type input struct {
		dir string
	}
	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name: "change directory successfully",
			input: input{
				dir: "/root",
			},
			wantErr: false,
		},
		{
			name: "fail to change directory",
			input: input{
				dir: "/invalid",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := changeDirectory(tt.input.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("changeDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetContainerId(t *testing.T) {
	type input struct {
		pid string
	}
	tests := []struct {
		name    string
		input   input
		want    string
		wantErr bool
	}{
		{
			name: "get container id successfully",
			input: input{
				pid: strconv.Itoa(os.Getpid()),
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "fail to get container id",
			input: input{
				pid: "invalid",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getContainerId(tt.input.pid)
			if (err != nil) != tt.wantErr {
				t.Errorf("getContainerId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) == "" {
				t.Errorf("getContainerId() got empty string")
			}
		})
	}
}

func TestGetImageId(t *testing.T) {
	type input struct {
		container_id string
		sock_path    string
	}
	tests := []struct {
		name    string
		input   input
		want    string
		wantErr bool
	}{
		{
			name: "get image id successfully",
			input: input{
				container_id: "invalid",
				sock_path:    "/run/containerd/containerd.sock",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getImageId(tt.input.container_id, tt.input.sock_path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getImageId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if strings.TrimSpace(got) == "" {
				t.Errorf("getImageId() got empty string")
			}
		})
	}
}

func TestWriteCoreConfig(t *testing.T) {
	type input struct {
		data string
	}
	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name: "write core config successfully",
			input: input{
				data: "test data",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeCoreConfig(tt.input.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeCoreConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			content, err := ioutil.ReadFile("coredump.config")
			if err != nil {
				t.Error("failed to read file")
				return
			}
			if !bytes.Equal(content, []byte(tt.input.data)) {
				t.Errorf("writeCoreConfig() content = %v, want = %v", string(content), tt.input.data)
			}
			os.Remove("coredump.config")
		})
	}
}

func TestCompress(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name: "compress successfully",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compress()
			if (err != nil) != tt.wantErr {
				t.Errorf("compress() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check if the compressed file exists
			if _, err := os.Stat("coredump.info.zip"); os.IsNotExist(err) {
				t.Errorf("compress() failed to create compressed file")
			} else {
				os.Remove("coredump.info.zip")
			}
		})
	}
}
