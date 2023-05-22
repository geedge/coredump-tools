CC=g++
CFLAGS=-I/builds/linxin/coredump-tools/breakpad-coredump-tools/src -std=c++11 -fPIC
LDFLAGS=-shared -fPIC
SRC=/builds/linxin/coredump-tools/breakpad-coredump-tools/src/tools/linux/coredump_handler_wrapper/coredump_handler_wrapper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/linux_core_dumper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/minidump_writer.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/path_helper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/linux_libc_support.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/elf_core_dump.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/minidump_file_writer.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/linux_dumper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/memory_mapped_file.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/safe_readlink.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/linux_ptrace_dumper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/dump_writer_common/ucontext_reader.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/string_conversion.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/dump_writer_common/thread_info.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/file_id.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/linux_core_dumper.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/minidump_writer.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/client/linux/minidump_writer/pe_file.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/convert_UTF.cc /builds/linxin/coredump-tools/breakpad-coredump-tools/src/common/linux/elfutils.cc

OBJ=$(SRC:.cc=.o)

all: coredump_handler_wrapper.so

%.o: %.cc
	$(CC) $(CFLAGS) -c $< -o $@

coredump_handler_wrapper.so: $(OBJ)
	$(CC) $(LDFLAGS) $^ -o $@

clean:
	rm -f *.o coredump_handler_wrapper.so
