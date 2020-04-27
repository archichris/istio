#!/bin/bash
docker tag $(docker images | grep "istio/$1" | awk '{print$3}') 192.168.56.10:5000/$1:$2
docker push 192.168.56.10:5000/$1:$2
