#!/bin/bash
set -e 

workspace=$(cd $(dirname $0);pwd)
cd $workspace

IMAGE=comb-pingpong-multi-network
TAG=1.0.0

docker build -t $IMAGE:$TAG .
