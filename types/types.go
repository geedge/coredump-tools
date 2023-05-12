package types

type Coreconfig struct {
	Core_pattern     string
	Core_limited     string
	Core__pipe_limit string
	Socket_path      string
}
type Pipeconfig struct {
	Compress              bool   //是否压缩
	Storage_type          int    //是否存储为minidump格式
	Storage               string //存储位置
	Total_file_mem_limit  string //磁盘空间限制
	Containered_sock_path string //ctr sock地址
}
type Coredump_config struct {
	Initial_ns_pid       string
	Process_ns_pid       string
	Process_exe_path     string
	Timestamp            int64
	Corepipe_config_path string `json:"-"`
	Container_id         string
	Image_name           string
	UID                  string
	GID                  string
	Signal               int
	Hostname             string
	Storage              string
}
type Coredump_cli_table struct {
	PID       string
	UID       string
	GID       string
	SIG       int
	EXE       string
	CONTAINER string
	IMAGE     string
	HOSTNAME  string
	STORAGE   string
	TIMESTAMP string
}
