#!/bin/bash
origin=$(dirname $(pwd))
docker image rm -f $(docker images | grep pilot | awk '{print$3}')
make docker.pilot
cd ${origin}
tag.sh pilot $1
