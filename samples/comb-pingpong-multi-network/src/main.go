// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	// "github.com/go-chassis/go-chassis/core/registry"
	client "github.com/go-chassis/go-chassis/pkg/scclient"
	"github.com/go-chassis/go-chassis/pkg/scclient/proto"
)

var (
	svcTmp = proto.MicroService{
		AppId:       "default",
		ServiceName: "server",
		Version:     "0.0.1",
	}
	instTmp = proto.MicroServiceInstance{
		InstanceId:     "replace",
		App:            "default",
		ServiceName:    "replace",
		HostName:       "replace",
		ServiceId:      "replace",
		Status:         "UP",
		Endpoints:      []string{"udp://127.0.0.1:8080"},
		Properties:     map[string]string{"a": "b", "extPlane_fabric": "tcp://192.168.0.1:80"},
		DataCenterInfo: &proto.DataCenterInfo{Name: "dc", Region: "bj", AvailableZone: "az1"},
	}

	opt = client.Options{
		EnableSSL:    false,
		ConfigTenant: "default",
		Timeout:      time.Duration(15),
		Version:      "v3",
	}
)

func register(r *client.RegistryClient, svcName, baseAddr, fabricAddr string) (string, string) {
	svcTmp.ServiceName = svcName
	svcTmp.ServiceId = ""
	svcId, err := r.RegisterService(&svcTmp)
	if err != nil {
		log.Printf("[Error] Register service %s failed, %v\n", svcName, err)
		return "", ""
	}

	instTmp.ServiceName = svcTmp.ServiceName
	instTmp.ServiceId = svcId
	instTmp.InstanceId = ""
	instTmp.HostName = svcTmp.ServiceName
	instTmp.Endpoints = []string{baseAddr}
	instTmp.Properties["extPlane_fabric"] = fabricAddr

	insId, err := r.RegisterMicroServiceInstance(&instTmp)
	if err != nil {
		log.Printf("[Error] Register instance %v failed, %v\n", instTmp, err)
		return "", ""
	}
	return svcId, insId
}

// NewCombMonitor watches for changes in Consul services and CatalogServices
func main() {
	log.Printf("[Info] Start working, %v", os.Args)
	namePrefix := os.Args[1]
	r := &client.RegistryClient{}
	address := os.Getenv("COMB_ADDR")
	opt.Addrs = []string{address}
	err := r.Initialize(opt)
	if err != nil {
		log.Printf("[Error] Client failed, %v\n", err)
		os.Exit(-1)
	}

	info := strings.Split(os.Args[2], ":")
	eth := info[0]
	port := info[1]

	inf, err := net.InterfaceByName(eth)
	if err != nil {
		log.Printf("[Error] InterfaceByName(%v) failed, %v\n", eth, err)
		return
	}
	ips, err := inf.Addrs()
	if err != nil {
		log.Printf("[Error]Addrs(%v) failed, %v\n", eth, err)
		return
	}

	ipnet, ok := ips[0].(*net.IPNet)
	if !ok {
		log.Printf("[Error] Addrs(%v) not right\n", ips)
		return
	}

	baseAddr := fmt.Sprintf("%s:%s", ipnet.IP.String(), port)
	basePort, _ := strconv.Atoi(port)
	basePort = basePort + 100
	cliBaseAddr := &net.TCPAddr{IP: ipnet.IP, Port: basePort}

	info = strings.Split(os.Args[3], ":")
	eth = info[0]
	port = info[1]

	inf, err = net.InterfaceByName(eth)
	if err != nil {
		log.Printf("[Error] InterfaceByName(%v) failed, %v\n", eth, err)
		return
	}
	ips, err = inf.Addrs()
	if err != nil {
		log.Printf("[Error] Addrs(%v) failed, %v\n", eth, err)
		return
	}

	ipnet, ok = ips[0].(*net.IPNet)
	if !ok {
		log.Printf("[Error] Addrs(%v) not right\n", ips)
		return
	}

	fabricAddr := fmt.Sprintf("%s:%s", ipnet.IP.String(), port)
	fabricPort, _ := strconv.Atoi(port)
	fabricPort = fabricPort + 100
	cliFabricAddr := &net.TCPAddr{IP: ipnet.IP, Port: fabricPort}

	svcID, insID := register(r, namePrefix+"_Server", "tcp://"+baseAddr, "tcp://"+fabricAddr)

	if len(svcID) == 0 || len(insID) == 0 {
		return
	}

	go serve(baseAddr)
	go serve(fabricAddr)

	peerSvc := os.Args[4] + "_Server"
	var instances []*proto.MicroServiceInstance
	for {
		instances, err = r.FindMicroServiceInstances(svcID, "default", peerSvc, "latest")
		if err != nil {
			log.Printf("[Error] FindMicroServiceInstances(%s, default, %s, latest) failed %v", svcID, peerSvc, err)
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	instance := instances[0]

	targetBaseAddr := strings.Split(strings.Split(instance.Endpoints[0], "://")[1], "?")[0]
	targetFabricAddr := strings.Split(strings.Split(instance.Properties["extPlane_fabric"], "://")[1], "?")[0]

	go cliProc(cliBaseAddr, targetBaseAddr)
	go cliProc(cliFabricAddr, targetFabricAddr)

	ch := make(chan struct{})
	<-ch
}

// serve starts serving on a given address
func serve(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println("[Error] Failed to create listener, err:", err)
		os.Exit(1)
	}
	log.Printf("[Info] Listening on %s\n", listener.Addr())

	// listen for new connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("[Error] Failed to accept connection, err:", err)
			continue
		}

		// pass an accepted connection to a handler goroutine
		go handleConnection(conn)
	}
}

// handleConnection handles the lifetime of a connection
func handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		// read client request data
		bytes, err := reader.ReadBytes(byte('\n'))
		if err != nil {
			if err != io.EOF {
				log.Println("[Error] Failed to read data, err:", err)
			}
			return
		}
		fmt.Printf("%s", bytes)

		// prepend prefix and send as response
		line := fmt.Sprintf("Pong: %s -> %s\n", conn.LocalAddr(), conn.RemoteAddr())
		_, err = conn.Write([]byte(line))
		if err != nil {
			log.Printf("[Error] Conn.Write failed, %v", err)
		} else {
			log.Printf("[Info] Server Reply: %s", line)
		}
	}
}

func cliProc(localAddr *net.TCPAddr, tagetAddr string) {
	d := net.Dialer{LocalAddr: localAddr}
	request := fmt.Sprintf("Ping: %s -> %s", localAddr.String(), tagetAddr)
	for {
		conn, err := d.Dial("tcp", tagetAddr)
		if err != nil {
			log.Printf("[Error] Dial(%v) failed, %v\n", tagetAddr, err)
			time.Sleep(time.Second * 1)
			continue
		}
		defer conn.Close()
		for {
			if _, err := conn.Write([]byte(request + "\n")); err != nil {
				log.Printf("[Error] Couldn't send request: %v\n", err)
				return
			} else {
				reader := bufio.NewReader(conn)
				if response, err := reader.ReadBytes(byte('\n')); err != nil {
					log.Printf("[Error] Couldn't read server response: %v\n", err)
					return
				} else {
					log.Print(string(response))
				}

			}
			time.Sleep(time.Second * 1)
		}
	}

}
