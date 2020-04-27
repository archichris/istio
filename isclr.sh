#!/bin/bash
for a in proxyv2 pilot
do
   docker image rm -f $(docker images | grep $a | awk '{print$3}')
done
