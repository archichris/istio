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
	"os"
	"testing"
	"time"
)

// TestTcpEchoServer tests the behavior of the TCP Echo Server.
func TestTcpEchoServer(t *testing.T) {
	// start the TCP Echo Server
	os.Args = []string{"main", "eth0:9000", "eth1:20000", "172.16.56.10:9001"}
	go main()

	// wait for the TCP Echo Server to start
	time.Sleep(2 * time.Second)

	os.Args = []string{"main", "eth1:9001", "eth0:20001", " 192.168.56.10:9000"}
	go main()

	time.Sleep(10 * time.Second)
}
