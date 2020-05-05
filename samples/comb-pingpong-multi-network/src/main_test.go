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
	"log"
	"os"
	"testing"
	"time"
)

// TestTcpEchoServer tests the behavior of the TCP Echo Server.
func TestTcpEchoServer(t *testing.T) {
	// start the TCP Echo Server
	log.Printf("test\n")
	os.Args = []string{"main", "a", "eth0:9000", "eth1:9001", "b"}
	os.Setenv("COMB_ADDR", "http://192.168.56.31:30543")
	go main()

	// wait for the TCP Echo Server to start
	time.Sleep(2 * time.Second)

	os.Args = []string{"main", "b", "eth0:1000", "eth1:10001", "a"}
	os.Setenv("COMB_ADDR", "http://192.168.56.31:30543")
	go main()

	time.Sleep(10 * time.Second)
}
