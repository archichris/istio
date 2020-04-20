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
	"testing"
	"time"

	client "github.com/go-chassis/go-chassis/pkg/scclient"
)

var (
	testAddr = "192.168.56.31:30543"
	testOpt  = client.Options{
		Addrs:        []string{testAddr},
		EnableSSL:    false,
		ConfigTenant: "default",
		Timeout:      time.Duration(15),
	}
)

func TestNewCombMonitor(t *testing.T) {
	m, err := NewCombMonitor(&testOpt)
	if err != nil {
		t.Errorf("NewCombMonitor failed, %v", err)
		return
	}
	m.client.Close()

}

func TestStart(t *testing.T) {
	monitor, err := NewCombMonitor(&testOpt)
	if err != nil {
		t.Errorf("NewCombMonitor failed, %v", err)
		return
	}
	defer monitor.client.Close()

	testCvsSvc.ServiceId = ""
	serviceId, err := monitor.client.RegisterService(&testCvsSvc)
	if err != nil {
		t.Errorf("RegisterService failed, %v", err)
		return
	}
	defer monitor.client.UnregisterMicroService(serviceId)

	instIds := []string{}

	for _, inst := range testCvsInsts {
		inst.ServiceId = serviceId
		inst.InstanceId = ""
		instId, err := monitor.client.RegisterMicroServiceInstance(&inst)
		if err != nil {
			t.Errorf("RegisterMicroServiceInstance failed, %v", err)
		}
		instIds = append(instIds, instId)
	}

	defer func() {
		for _, instid := range instIds {
			_, _ = monitor.client.UnregisterMicroServiceInstance(serviceId, instid)
		}
	}()

	stop := make(chan struct{})
	monitor.Start(stop)
	defer close(stop)

	if len(instIds) == 0 {
		t.Errorf("RegisterMicroServiceInstance failed, no instance in registry")
	}

	svcNum := len(monitor.services)

	combSvcNum := 0
	for _, v := range monitor.combSvc {
		combSvcNum += len(v)
	}

	if combSvcNum != svcNum {
		t.Errorf("initCache failed, services:%+v, combSvc: %+v", monitor.services, monitor.combSvc)
	}

	if len(monitor.serviceInstances) < combSvcNum {
		t.Errorf("initCache failed, mointor.serviceInstances: %+v", monitor.serviceInstances)
	}

}
