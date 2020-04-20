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

package comb

import (
	"fmt"
	"strings"
	"testing"

	"istio.io/istio/pkg/config/protocol"

	"github.com/go-chassis/go-chassis/pkg/scclient/proto"
	"istio.io/istio/pkg/config/host"
)

var (
	// goodLabels = []string{
	// 	"key1|val1",
	// 	"version|v1",
	// }

	// badLabels = []string{
	// 	"badtag",
	// 	"goodtag|goodvalue",
	// }

	testCvsSvc = proto.MicroService{
		ServiceId:   "serviceid",
		AppId:       "default",
		ServiceName: "test",
		Version:     "0.0.1",
	}

	testCvsInsts = []proto.MicroServiceInstance{
		{
			InstanceId:     "instanceid1",
			App:            testCvsSvc.AppId,
			ServiceName:    testCvsSvc.ServiceName,
			HostName:       "test1",
			ServiceId:      testCvsSvc.ServiceId,
			Status:         "UP",
			Endpoints:      []string{"udp://127.0.0.1:8080"},
			Properties:     map[string]string{"a": "b", "extPlane_fabric": "tcp://192.168.0.1:80", "extPlane_om": "tcp://172.16.0.1:808"},
			DataCenterInfo: &proto.DataCenterInfo{Name: "dc", Region: "bj", AvailableZone: "az1"},
		},
		{
			InstanceId:     "instanceid2",
			HostName:       "test2",
			ServiceId:      testCvsSvc.ServiceId,
			Status:         "UP",
			Endpoints:      []string{"udp://127.0.0.2:8080"},
			Properties:     map[string]string{"a": "b", "extPlane_fabric": "tcp://192.168.0.2:80", "extPlane_om": "tcp://172.16.0.2:808"},
			DataCenterInfo: &proto.DataCenterInfo{Name: "dc", Region: "bj", AvailableZone: "az1"},
		},
		{
			InstanceId:     "instanceid3",
			HostName:       "test3",
			ServiceId:      testCvsSvc.ServiceId,
			Status:         "UP",
			Endpoints:      []string{"udp://127.0.0.3:8080"},
			Properties:     map[string]string{"a": "b", "extPlane_fabric": "tcp://192.168.0.3:80", "extPlane_om": "tcp://172.16.0.3:808"},
			DataCenterInfo: &proto.DataCenterInfo{Name: "dc", Region: "bj", AvailableZone: "az2"},
		},
	}
	instps = []*proto.MicroServiceInstance{&testCvsInsts[0], &testCvsInsts[1], &testCvsInsts[2]}
)

func TestParseEndpoint(t *testing.T) {
	endpoint1 := "127.0.0.1:8080?sslEnabled=true"
	endpoint2 := "192.168.0.1:80"
	endpoint3 := "172.16.0.1:808?sslEnabled=false"
	addr, port, ssl := parseEndpoint(endpoint1)
	if addr != "127.0.0.1" || port != "8080" || !ssl {
		t.Errorf("parseEndpoint failed, %s->%s,%s,%v", endpoint1, addr, port, ssl)
	}
	addr, port, ssl = parseEndpoint(endpoint2)
	if addr != "192.168.0.1" || port != "80" || ssl {
		t.Errorf("parseEndpoint failed, %s->%s,%s,%v", endpoint2, addr, port, ssl)
	}
	addr, port, ssl = parseEndpoint(endpoint3)
	if addr != "172.16.0.1" || port != "808" || ssl {
		t.Errorf("parseEndpoint failed, %s->%s,%s,%v", endpoint3, addr, port, ssl)
	}
}

func TestConvertProtocol(t *testing.T) {
	testEpsMap := map[string]string{
		"udp":  "127.0.0.1:8080",
		"tcp":  "127.0.0.1:8080",
		"grpc": "127.0.0.1:8080"}
	out := convertPort(testEpsMap)
	num := 0
	for _, port := range out {
		switch port.Name {
		case "udp":
			if port.Protocol == protocol.UDP && port.Port == 8080 {
				num++
			}
		case "tcp":
			if port.Protocol == protocol.TCP && port.Port == 8080 {
				num++
			}
		case "grpc":
			if port.Protocol == protocol.GRPC && port.Port == 8080 {
				num++
			}
		default:
			t.Errorf("convertPort error, incorrect port info %+v", port)
		}
	}

	if num != 3 {
		t.Errorf("convertPort error, number of port is incorrect %+v", out)
	}
}
func TestGetPlaneEpsMap(t *testing.T) {
	pepsMap := getPlaneEpsMap(instps[0])
	num := 0
	for p, eps := range pepsMap {
		switch p {
		case "default":
			if eps["udp"] == "127.0.0.1:8080" {
				num++
			} else {
				t.Errorf("getPlaneEpsMap failed %+v", pepsMap)
			}
		case "fabric":
			if eps["tcp"] == "192.168.0.1:80" {
				num++
			} else {
				t.Errorf("getPlaneEpsMap failed %+v", pepsMap)
			}
		case "om":
			if eps["tcp"] == "172.16.0.1:808" {
				num++
			} else {
				t.Errorf("getPlaneEpsMap failed %+v", pepsMap)
			}

		}
	}
	if num != 3 {
		t.Errorf("getPlaneEpsMap failed(%d) %+q", num, pepsMap)
	}
}

func TestConvertService(t *testing.T) {
	ss := convertService(&testCvsSvc, instps)

	if len(ss) != 3 {
		t.Errorf("convertService failed, %+v", ss)
	}
	num := 0
	for _, s := range ss {
		switch s.Hostname {
		case "default.test.default.__v0_0_1":
			if s.Address == "0.0.0.0" && s.Ports[0].Port == 8080 {
				num++
			}
		case "fabric.test.default.__v0_0_1":
			if s.Address == "0.0.0.0" && s.Ports[0].Port == 80 {
				num++
			}
		case "om.test.default.__v0_0_1":
			if s.Address == "0.0.0.0" && s.Ports[0].Port == 808 {
				num++
			}
		}
	}
	if num != 3 {
		t.Errorf("convertService failed, matchnum:%d, %+v", num, ss)
	}

}

func TestConvertInstance(t *testing.T) {
	ss := convertService(&testCvsSvc, instps)
	for _, s := range ss {
		for _, ins := range instps {
			instances := convertInstance(s, ins)
			if len(instances) != 1 {
				t.Errorf("convertInstance failed, %+v", instances)
			}
			if instances[0].Endpoint.Labels["serviceid"] != "serviceid" ||
				(instances[0].Endpoint.EndpointPort != 8080 && instances[0].Endpoint.EndpointPort != 808 && instances[0].Endpoint.EndpointPort != 80) {
				t.Errorf("convertInstance failed, %v", instances[0].Endpoint)
			}

		}
	}

}

func TestServiceHostnameSuffix(t *testing.T) {
	suffix := serviceHostnameSuffix(&testCvsSvc)
	if suffix != fmt.Sprintf("%s.%s.__v%s", testCvsSvc.ServiceName, testCvsSvc.AppId, strings.ReplaceAll(testCvsSvc.Version, ".", "_")) {
		t.Errorf("ServiceHostnameSuffix failed, %s", suffix)
	}
}

func TestServiceHostname(t *testing.T) {
	out := serviceHostname("base", "svc.default.__v1.1.1")
	if string(out) != "base.svc.default.__v1.1.1" {
		t.Errorf("TestServiceHostname failed, %s", string(out))
	}
}

func TestParseHostName(t *testing.T) {
	hostname := host.Name("base.svc.default.__v0.0.1")
	plane, svcName, appID, err := parseHostName(hostname)
	if plane != "base" || svcName != "svc" || appID != "default" || err != nil {
		t.Errorf("TestParseHostName failed, %s-%s-%s, %v", plane, svcName, appID, err)
	}
}
