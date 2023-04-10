package config

import (
	"encoding/json"
	"io/ioutil"
)

type Coreconfig struct {
	Core_pattern     string
	Core_limited     string
	Core__pipe_limit string
	Socket_path      string
}
type Pipeconfig struct {
	Save_model            int //0为文件保存 1为压缩保存 2为minidump保存
	File_base_path        string
	Total_file_mem_limit  string
	Containered_sock_path string
}

func Init() (Coreconfig, error) {
	var config Coreconfig
	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}
func PipeInit() (Pipeconfig, error) {
	var config Pipeconfig
	content, err := ioutil.ReadFile("./pipe-config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}
