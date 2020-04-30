#!/bin/bash
set -e 

ORIGIN_PATH=$(cd $(dirname $0);pwd)

echo "Build client"
$ORIGIN_PATH/client/build/build.sh  client

echo "Build server"
$ORIGIN_PATH/server/build/build.sh  server

mkdir -p $ORIGIN_PATH/image

rm -rf $ORIGIN_PATH/image/*

mv -f $ORIGIN_PATH/client/release/client.tar.gz  $ORIGIN_PATH/image
mv -f $ORIGIN_PATH/server/release/server.tar.gz  $ORIGIN_PATH/image

echo "Build image"
cp Dockerfile -rf $ORIGIN_PATH/image/

cp build_image.sh -rf $ORIGIN_PATH/image/
cp start.sh -rf $ORIGIN_PATH/image/

$ORIGIN_PATH/image/build_image.sh