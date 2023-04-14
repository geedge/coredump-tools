package types

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
type Coredump_config struct {
	Initial_ns_pid       string
	Process_ns_pid       string
	Process_exe_path     string
	Timestap             int64
	Corepipe_config_path string
	Image_id             string
}
