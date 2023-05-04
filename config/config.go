package config

import (
	"coredump-tools/types"
	"encoding/json"
	"io/ioutil"
)

func Init() (types.Coreconfig, error) {
	var config types.Coreconfig
	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}
func PipeInit(config_path string) (types.Pipeconfig, error) {
	var config types.Pipeconfig
	content, err := ioutil.ReadFile(config_path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}
