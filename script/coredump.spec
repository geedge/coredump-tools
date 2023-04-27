%{!?_topdir: %define _topdir %(pwd)}

Name: coredump-tools
Version: %{_version}
Release: %{_release}
Summary: tsg's coredump handler and tools
Group: Applications/Internet
License: GPL
URL: https://git.mesalab.cn/linxin/coredump-handler


Source0: coredump-tools-%{_version}.tar.gz
%define debug_package %{nil}
%description
This package contains the binary executable for coredump-handler and coredump-tool, as well as its configuration files.

%prep
%setup -q

%build
cd ./coredump-handler
go build -o coredump-handler .
cd ..
cd ./coredump-ctl
go build -o coredump-tool .
cd ..
%install
mkdir -p %{buildroot}/etc/coredump-handler
mkdir -p %{buildroot}/opt/tsg/coredump
cp ./coredump-handler/coredump-handler %{buildroot}/opt/tsg/coredump
cp ./config/config.json %{buildroot}/etc/coredump-handler
cp ./coredump-ctl/coredump-tool %{buildroot}/opt/tsg/coredump
%post
echo "export PATH=/opt/tsg/coredump:$PATH" >>~/.bashrc
%files
%defattr (-,root,root)
/etc/coredump-handler/config.json
/opt/tsg/coredump/coredump-handler
/opt/tsg/coredump/coredump-tool
