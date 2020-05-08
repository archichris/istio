#!/bin/bash
origin=$(pwd)
docker image rm -f $(docker images | grep proxyv2 | awk '{print$3}')
cd ../proxy/
make build_envoy
cd ${origin}
make docker.proxyv2
cd ${origin}
tag.sh proxyv2 $1
