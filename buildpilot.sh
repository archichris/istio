#!/bin/bash
rm ./out/* -rf
./isclr.sh
make docker.pilot
