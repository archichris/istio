#!/bin/bash
docker tag $(docker images | grep "istio/pilot" | awk '{print$3}') 192.168.56.10:5000/pilot:$1
docker push 192.168.56.10:5000/pilot:$1
