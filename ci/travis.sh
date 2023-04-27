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

yum install -y cligen-devel clixon-devel

if [ -n "${LIBYAML_BUILD}" ]; then
	git clone https://github.com/yaml/libyaml.git
	cd libyaml
	sed -i 's/^add_library.*$/add_library(yaml SHARED ${SRCS})/g' CMakeLists.txt
	cmake3 .
	make && make install
	cd ..
fi

mkdir build || true

cd build

cmake3 -DVERSION_DAILY_BUILD=$TESTING_VERSION_BUILD ..

if [ -n "${PACKAGE}" ]; then
	make package
fi
ls -halt ./*.rpm

if [ -n "${UPLOAD}" ]; then	
	cp ~/rpm_upload_tools.py ./
	python3 rpm_upload_tools.py ${PULP3_RPM_REPO_NAME} ${PULP3_RPM_DIST_NAME} *.rpm
fi


