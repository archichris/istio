#!/bin/bash
set -e 


appname=$1

BUILD_PATH=$(cd $(dirname $0);pwd)
ROOT_PATH=$(cd $BUILD_PATH/..;pwd)
RELEASE_PATH=$ROOT_PATH/release

cd $ROOT_PATH

mkdir -p $RELEASE_PATH/$appname
rm -rf $RELEASE_PATH/*

CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o "$RELEASE_PATH/$appname/app"

# go build -a -o "$RELEASE_PATH/$appname/app"

cp -rf conf $RELEASE_PATH/$appname
if [ -d "lib" ]; then
	cp -rf lib $RELEASE_PATH/$appname
fi

# cp -rf build/start.sh $RELEASE_PATH/$appname

cd $RELEASE_PATH

package=$appname.tar.gz
tar -zcvf $package $appname

# cp $RELEASE_PATH/$package $BUILD_PATH
# bash $BUILD_PATH/build_image.sh

echo "build $appname success!"
