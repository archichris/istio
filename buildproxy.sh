#!/bin/bash
origin=$(dirname $(pwd))
docker image rm -f $(docker images | grep proxyv2 | awk '{print$3}')
make docker.proxyv2
cd ${origin}
tag.sh proxyv2 $1
