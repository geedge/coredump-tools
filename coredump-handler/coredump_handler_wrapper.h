#ifndef CORE_HANDLER_H_
#define CORE_HANDLER_H_

#include <pwd.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/types.h>
#include <syslog.h>
#include <unistd.h>

#ifdef __cplusplus
extern "C" {
#endif
_Bool HandleCrash(pid_t pid, const char* procfs_dir, const char* md_filename);
#ifdef __cplusplus
}
#endif

#endif  // CORE_HANDLER_H_
