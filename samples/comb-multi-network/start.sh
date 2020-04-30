#!/bin/sh
set -e 


# //////////////////////////////////////////////////////////////////////////// #
#                               go sdk                                         #
# //////////////////////////////////////////////////////////////////////////// #

cd $(dirname $0)

if [ "$CSE_SERVICE_CENTER" ]; then
    export CSE_REGISTRY_ADDR=$CSE_SERVICE_CENTER
fi

if [ "$CSE_CONFIG_CENTER" ]; then
    export CSE_CONFIG_CENTER_ADDR=$CSE_CONFIG_CENTER
fi


server_if=$SERVER_IF
client_if=$CLIENT_IF

name_prefix=$NAME_PREFIX
target_prefix=$TARGET_PREFIX


# listen_addr="0.0.0.0"
server_advertise_addr=$(ifconfig $server_if | grep -E 'inet\W' | grep -o -E [0-9]+.[0-9]+.[0-9]+.[0-9]+ | head -n 1)
client_advertise_addr=$(ifconfig $client_if | grep -E 'inet\W' | grep -o -E [0-9]+.[0-9]+.[0-9]+.[0-9]+ | head -n 1)

# cd /home/$name
# replace ip addr
sed -i s/"listenAddress:\s\{1,\}[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/"listenAddress: $server_advertise_addr"/g server/conf/chassis.yaml
sed -i s/"advertiseAddress:\s\{1,\}[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/"advertiseAddress: $server_advertise_addr"/g server/conf/chassis.yaml
sed -i s/"name: Server"/"name: ${name_prefix}_Server"/g server/conf/microservice.yaml
sed -i s/"listenAddress:\s\{1,\}[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/"listenAddress: $client_advertise_addr"/g client/conf/chassis.yaml
sed -i s/"advertiseAddress:\s\{1,\}[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}.[0-9]\{1,3\}"/"advertiseAddress: $client_advertise_addr"/g client/conf/chassis.yaml
sed -i s/"name: Client"/"name: ${name_prefix}_Client"/g client/conf/microservice.yaml

./server/app &
./client/app &

while true; do
    sleep 60
done

