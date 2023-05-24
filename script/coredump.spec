%{!?_topdir: %define _topdir %(pwd)}

Name: coredump-tools
Version: %{_version}
Release: %{_release}
Summary: tsg's coredump handler and tools
Group: Applications/Internet
License: GPL
URL: https://git.mesalab.cn/linxin/coredump-handler


Source0: coredump-tools-%{_version}-%{_commit}.tar.gz
%define debug_package %{nil}
%description
This package contains the binary executable for coredump-handler and coredump-tool, as well as its configuration files.

%prep
%setup -q

%build
cd ./coredump-handler
go build -o coredump-handler .
cd ..
cd ./coredump-tool
go build -o coredump-tool .
cd ..
%install
mkdir -p %{buildroot}/opt/tsg/coredump
mkdir -p %{buildroot}/opt/tsg/coredump/bin
mkdir -p %{buildroot}/opt/tsg/coredump/etc
mkdir -p %{buildroot}/etc/profile.d
mkdir -p %{buildroot}/usr/lib/tmpfiles.d
mkdir -p %{buildroot}/usr/lib/sysctl.d
cp ./coredump-handler/coredump_handler_wrapper.so %{buildroot}/opt/tsg/coredump/bin
cp ./coredump-handler/coredump-handler %{buildroot}/opt/tsg/coredump/bin
cp ./config/config.json %{buildroot}/opt/tsg/coredump/etc
cp ./coredump-tool/coredump-tool %{buildroot}/opt/tsg/coredump/bin
cp ./coredump-tool/minidump-2-core %{buildroot}/opt/tsg/coredump/bin
cp ./script/coredump-tools.sh %{buildroot}/etc/profile.d
cp ./coredump.conf %{buildroot}/usr/lib/tmpfiles.d
cp ./60-coredump.conf %{buildroot}/usr/lib/sysctl.d
%files
%defattr (-,root,root)
/opt/tsg/coredump/etc/config.json
/opt/tsg/coredump/bin/coredump-handler
/opt/tsg/coredump/bin/coredump-tool
/opt/tsg/coredump/bin/minidump-2-core
/opt/tsg/coredump/bin/coredump_handler_wrapper.so
/etc/profile.d/coredump-tools.sh
/usr/lib/tmpfiles.d/coredump.conf
/usr/lib/sysctl.d/60-coredump.conf
