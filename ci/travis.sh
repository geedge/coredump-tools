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
export VERSION_ID=$(./autorevision.sh -t sh -o version | grep VCS_TAG | sed 's/^.*v//g' | sed 's/"//g')
export VERSION_ID="1.0.0"
yum install -y golang
go version
mkdir -p ./rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
tar -czvf ./rpmbuild/SOURCES/coredump-tools-${VERSION_ID}.tar.gz ../coredump-tools
cp ./script/coredump.spec ./rpmbuild/SPECS
rpmbuild --define "_topdir `pwd`/rpmbuild" --define "_version ${VERSION_ID}" --define "_release release" -ba ./rpmbuild/SPECS/coredump.spec

ls -halt ./*.rpm

# if [ -n "${UPLOAD}" ]; then	
# 	cp ~/rpm_upload_tools.py ./
# 	python3 rpm_upload_tools.py ${PULP3_RPM_REPO_NAME} ${PULP3_RPM_DIST_NAME} *.rpm
# fi


