#!/usr/bin/env sh
echo $CPLUS_INCLUDE_PATH
set -evx

chmod +x ci/get-nprocessors.sh
. ci/get-nprocessors.sh

# if possible, ask for the precise number of processors,
# otherwise take 2 processors as reasonable default; see
# https://docs.travis-ci.com/user/speeding-up-the-build/#Makefile-optimization
if [ -x /usr/bin/getconf ]; then
    NPROCESSORS=$(/usr/bin/getconf _NPROCESSORS_ONLN)
else
    NPROCESSORS=2
fi

# as of 2017-09-04 Travis CI reports 32 processors, but GCC build
# crashes if parallelized too much (maybe memory consumption problem),
# so limit to 4 processors for the time being.
if [ $NPROCESSORS -gt 4 ] ; then
	echo "$0:Note: Limiting processors to use by make from $NPROCESSORS to 4."
	NPROCESSORS=4
fi

# Tell make to use the processors. No preceding '-' required.
MAKEFLAGS="j${NPROCESSORS}"
export MAKEFLAGS

env | sort

# Set default values to OFF for these variables if not specified.
: "${NO_EXCEPTION:=OFF}"
: "${NO_RTTI:=OFF}"
: "${COMPILER_IS_GNUCXX:=OFF}"
chmod +x autorevision.sh
export VERSION_ID=$(./autorevision.sh -t sh -o version | grep VCS_TAG | sed 's/^.*v//g' | sed 's/"//g'| sed 's/VCS_TAG=//g'|tr -d "_=")
export COMMIT_ID=$(./autorevision.sh -t sh -o version | grep VCS_SHORT_HASH | sed 's/^.*v//g' | sed 's/"//g'| sed 's/VCS_SHORT_HASH=//')
# export VERSION_ID="1.0.0"
yum install -y golang
go version
export GOPROXY=https://proxy.golang.com.cn,direct
go env
pwd
ls
mkdir -p /builds/linxin/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
make -C /builds/linxin/coredump-tools/breakpad-coredump-tools/src/tools/linux/coredump_handler_wrapper
mkdir -p /opt/tsg/coredump/bin
cp /builds/linxin/coredump-tools/breakpad-coredump-tools/src/tools/linux/coredump_handler_wrapper/coredump_handler_wrapper.so /opt/tsg/coredump/bin
cp /builds/linxin/coredump-tools/breakpad-coredump-tools/src/tools/linux/coredump_handler_wrapper/coredump_handler_wrapper.so /builds/linxin/coredump-tools/coredump-handler
make -C /builds/linxin/coredump-tools/breakpad-coredump-tools
cp /builds/linxin/coredump-tools/breakpad-coredump-tools/src/tools/linux/md2core/minidump-2-core /builds/linxin/coredump-tools/coredump-tool/
mv /builds/linxin/coredump-tools /builds/linxin/coredump-tools-${VERSION_ID}
tar --warning=no-file-changed -czvf /builds/linxin/rpmbuild/SOURCES/coredump-tools-${VERSION_ID}-${COMMIT_ID}.tar.gz ../coredump-tools-${VERSION_ID}
cp /builds/linxin/coredump-tools-${VERSION_ID}/script/coredump.spec /builds/linxin/rpmbuild/SPECS
cd  /builds/linxin
rpmbuild --define "_topdir `pwd`/rpmbuild" --define "_version ${VERSION_ID}" --define "_release ${COMMIT_ID}" --define "_commit ${COMMIT_ID}" -ba /builds/linxin/rpmbuild/SPECS/coredump.spec
cd /builds/linxin/rpmbuild/RPMS/x86_64
ls -halt ./*.rpm

if [ -n "${UPLOAD}" ]; then	
	cp ~/rpm_upload_tools.py ./
	python3 rpm_upload_tools.py ${PULP3_RPM_REPO_NAME} ${PULP3_RPM_DIST_NAME} *.rpm
fi