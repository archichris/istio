// Copyright 2018 Istio Authors
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
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// main serves as the program entry point
func main() {
	// obtain the port and prefix via program arguments
	info := strings.Split(os.Args[1], ":")
	if len(os.Args) < 4 {
		return
	}
	svcEth := info[0]
	svcPort := info[1]

	inf, err := net.InterfaceByName(svcEth)
	if err != nil {
		fmt.Printf("InterfaceByName(%v) failed, %v\n", svcEth, err)
		return
	}
	ips, err := inf.Addrs()
	if err != nil {
		fmt.Printf("Addrs(%v) failed, %v\n", svcEth, err)
		return
	}

	ipnet, ok := ips[0].(*net.IPNet)
	if !ok {
		fmt.Printf("Addrs(%v) not right\n", ips)
		return
	}

	svcIP := ipnet.IP.String()

	addr := fmt.Sprintf("%s:%s", svcIP, svcPort)
	go serve(addr)

	info = strings.Split(os.Args[2], ":")
	cliEth := info[0]
	cliPort := info[1]

	inf, err = net.InterfaceByName(cliEth)
	if err != nil {
		fmt.Printf("InterfaceByName(%v) failed, %v\n", cliEth, err)
		return
	}
	ips, err = inf.Addrs()
	if err != nil {
		fmt.Printf("Addrs(%v) failed, %v\n", cliEth, err)
		return
	}

	ipnet, ok = ips[0].(*net.IPNet)

	if !ok {
		fmt.Printf("Addrs(%v) not right\n", ips)
		return
	}

	port, err := strconv.Atoi(cliPort)

	if err != nil {
		fmt.Printf("Atoi(%v) failed\n", cliPort)
		return
	}

	cliAddr := &net.TCPAddr{IP: ipnet.IP, Port: port}
	d := net.Dialer{LocalAddr: cliAddr}

	// info = strings.Split(os.Args[2], ":")
	// peerAddr := info[0]
	// cliPort := info[1]

	peerAddr := strings.TrimSpace(os.Args[3])

	request := fmt.Sprintf("Ping: %v -> %v", cliAddr, peerAddr)

	for {
		conn, err := d.Dial("tcp", peerAddr)
		if err != nil {
			fmt.Printf("Dial(%v) failed, %v\n", peerAddr, err)
			time.Sleep(time.Second * 1)
			continue
		}
		defer conn.Close()
		for {
			if _, err := conn.Write([]byte(request + "\n")); err != nil {
				fmt.Printf("couldn't send request: %v\n", err)
				return
			} else {
				reader := bufio.NewReader(conn)
				if response, err := reader.ReadBytes(byte('\n')); err != nil {
					fmt.Printf("couldn't read server response: %v\n", err)
					return
				} else {
					fmt.Print(string(response))
				}

			}
			time.Sleep(time.Second * 1)
		}
	}
}

// serve starts serving on a given address
func serve(addr string) {
	// create a tcp listener on the given port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("failed to create listener, err:", err)
		os.Exit(1)
	}
	fmt.Printf("listening on %s\n", listener.Addr())

	// listen for new connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("failed to accept connection, err:", err)
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
				fmt.Println("failed to read data, err:", err)
			}
			return
		}
		fmt.Printf("%s", bytes)

		// prepend prefix and send as response
		line := fmt.Sprintf("Pong: %s -> %s\n", conn.LocalAddr(), conn.RemoteAddr())
		_, err = conn.Write([]byte(line))
		if err != nil {
			fmt.Printf("conn.Write failed, %v", err)
		}
	}
}
