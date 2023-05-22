// Copyright 2020 Google LLC
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// core_handler.cc: A tool to handle coredumps on Linux

#ifdef HAVE_CONFIG_H
#include <config.h>  // Must come first
#endif

#include <pwd.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/types.h>
#include <syslog.h>
#include <unistd.h>
#include <sstream>

#include "client/linux/minidump_writer/linux_core_dumper.h"
#include "client/linux/minidump_writer/minidump_writer.h"
#include "common/path_helper.h"
#include "common/scoped_ptr.h"

#include "coredump_handler_wrapper.h"

namespace {

using google_breakpad::AppMemoryList;
using google_breakpad::LinuxCoreDumper;
using google_breakpad::MappingList;
using google_breakpad::scoped_array;

// Size of the core dump to read in order to access all the threads
// descriptions.
//
// The first section is the note0 section which contains the thread states. On
// x86-64 a typical thread description take about 1432B. Reading 1 MB allows
// several hundreds of threads.
const int core_read_size = 1024 * 1024;

void ShowUsage(const char* argv0) {
  fprintf(stderr, "Usage: %s <process id> <minidump file>\n\n",
          google_breakpad::BaseName(argv0).c_str());
  fprintf(stderr,
          "A tool which serves as a core dump handler and produces "
          "minidump files.\n");
  fprintf(stderr, "Please refer to the online documentation:\n");
  fprintf(stderr,
          "https://chromium.googlesource.com/breakpad/breakpad/+/HEAD"
          "/docs/linux_core_handler.md\n");
}

bool WriteMinidumpFromCore(const char* filename,
                           const char* core_path,
                           const char* procfs_override) {
  MappingList mappings;
  AppMemoryList memory_list;
  LinuxCoreDumper dumper(0, core_path, procfs_override);
  return google_breakpad::WriteMinidump(filename, mappings, memory_list,
                                        &dumper);
}
extern "C" bool HandleCrash(pid_t pid, const char* procfs_dir, const char* md_filename) 
{
  int r = 0;
  scoped_array<char> buf(new char[core_read_size]);
  while (r != core_read_size) {
    int ret = read(STDIN_FILENO, &buf[r], core_read_size - r);
    if (ret == 0) {
      break;
    } else if (ret == -1) {
      syslog(LOG_ERR, "Cannot read data from stdin\n");
      return false;
    }
    r += ret;
  }

  int fd = memfd_create("core_file", MFD_CLOEXEC);
  if (fd == -1) {
    syslog(LOG_ERR, "Cannot create fd\n");
    return false;
  }

  int w = write(fd, &buf[0], r);
  if (w != r) {
    syslog(LOG_ERR, "Cannot write to fd\n");
    close(fd);
    return false;
  }

  std::stringstream core_file_ss;
  core_file_ss << "/proc/self/fd/" << fd;
  std::string core_file(core_file_ss.str());

  if (!WriteMinidumpFromCore(md_filename, core_file.c_str(), procfs_dir)) {
    close(fd);
     syslog(LOG_ERR, "Write minidump from core failed\n");
    return false;
  }
  close(fd);

  return true;
}

}  // namespace
