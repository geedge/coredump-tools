#ifndef COREDUMP_HANDLER_H_
#define COREDUMP_HANDLER_H_

#include <sys/types.h>

bool HandleCrash(pid_t pid, const char* procfs_dir, const char* md_filename);

#endif  // COREDUMP_HANDLER_H_
