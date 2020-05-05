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
	"testing"
	"time"

	client "github.com/go-chassis/go-chassis/pkg/scclient"
	"github.com/go-chassis/go-chassis/pkg/scclient/proto"
)

var (
	gTestCenterAddr = "192.168.56.31:30543"
	gTestOpt        = client.Options{
		Addrs:        []string{gTestCenterAddr},
		EnableSSL:    false,
		ConfigTenant: "default",
		Timeout:      time.Duration(15),
		Version:      "v3",
	}
)

func TestNewCombMonitor(t *testing.T) {
	monitor, err := NewCombMonitor(&gTestOpt)
	if err != nil {
		t.Errorf("NewCombMonitor failed, %v", err)
		return
	}
	if !monitor.verReg.MatchString("1.0.0") {
		t.Errorf("regexp match failed, 1.0.0")
	}
	if monitor.verReg.MatchString("2019.1107.1115.51") {
		t.Errorf("regexp match failed,  2019.1107.1115.51")
	}
}

func TestRegSelf(t *testing.T) {
	m, err := NewCombMonitor(&gTestOpt)
	if err != nil {
		t.Errorf("NewCombMonitor failed, %v", err)
		return
	}
	err = m.client.Initialize(*m.opt)
	if err != nil {
		t.Errorf("Initialize client failed,%v", err)
		return
	}
	defer m.client.Close()
	m.consumerId = ""
	err = m.regSelf()
	if err != nil {
		t.Errorf("regSelf failed, %v", err)
		return
	}
	if len(m.consumerId) < 1 {
		t.Errorf("regSelf failed, recorded consumer id is %v", m.consumerId)
		return
	}
	_, _ = m.client.UnregisterMicroService(m.consumerId)
}

func TestUpdateWatch(t *testing.T) {
	var err error
	m, _ := NewCombMonitor(&gTestOpt)
	err = m.client.Initialize(*m.opt)
	if err != nil {
		t.Errorf("Initialize client failed,%v", err)
		return
	}
	defer m.client.Close()
	err = m.regSelf()
	if err != nil {
		t.Errorf("regSelf failed, %v", err)
		return
	}
	defer m.client.UnregisterMicroService(m.consumerId)

	provider := client.DependencyMicroService{
		AppID:       testCvsSvc.AppId,
		ServiceName: testCvsSvc.ServiceName,
		Version:     testCvsSvc.Version,
	}

	// watchKey := strings.Join([]string{testCvsSvc.AppId, testCvsSvc.ServiceName, testCvsSvc.Version}, "_")
	m.depSvcs["serviceid"] = &provider
	m.doUpdateWatch = false
	err = m.updateWatch()
	if err != nil {
		t.Errorf("updateWatch failed, unnecessary update, %v", err)
	}
	m.doUpdateWatch = true
	err = m.updateWatch()
	if err != nil || m.doUpdateWatch {
		t.Errorf("updateWatch failed, err:%v, updateWatch:%v", err, m.doUpdateWatch)
	}
}

func TestServiceAddDelHandler(t *testing.T) {
	var err error
	m, _ := NewCombMonitor(&gTestOpt)
	err = m.client.Initialize(*m.opt)
	if err != nil {
		t.Errorf("Initialize client failed,%v", err)
		return
	}
	defer m.client.Close()
	err = m.regSelf()
	if err != nil {
		t.Errorf("register self failed, %v", err)
		return
	}
	defer m.client.UnregisterMicroService(m.consumerId)

	testCvsSvc.ServiceId = ""

	svcId, err := m.client.RegisterService(&testCvsSvc)
	if err != nil {
		t.Errorf("RegisterService failed, %v", err)
		return
	}

	inst := testCvsInsts[0]

	inst.ServiceId = svcId

	instId, err := m.client.RegisterMicroServiceInstance(&inst)
	if err != nil {
		t.Errorf("RegisterMicroServiceInstance failed, %v", err)
		_, _ = m.client.UnregisterMicroService(svcId)
		return
	}

	defer m.client.UnregisterMicroService(svcId)

	fmt.Printf("*************** test serviceAddHandler ***************\n")

	time.Sleep(periodicCheckTime)

	s := testCvsSvc
	s.ServiceId = svcId

	err = m.serviceAddHandler(&s)
	if err != nil {
		t.Errorf("serviceAddHandler failed, %v", err)
		return
	}

	hostnames, ok := m.combSvcs[svcId]

	if !ok {
		t.Errorf("serviceAddHandler failed, comb service %v not exist", svcId)
		return
	}

	if _, ok := m.depSvcs[svcId]; !ok {
		t.Errorf("serviceAddHandler failed, depSvcs service %v not exist", svcId)
		return
	}

	for _, hostname := range hostnames {
		_, ok := m.services[hostname]
		if !ok {
			t.Errorf("serviceAddHandler failed, service of %v not exist", hostname)
			return
		}

		insts, ok := m.serviceInstances[hostname]
		if !ok {
			t.Errorf("serviceAddHandler failed, instance of %v not exist", hostname)
			return
		}

		for _, inst := range insts {
			if inst.Endpoint.Labels["serviceid"] != svcId || inst.Endpoint.Labels["instanceid"] != instId {
				t.Errorf("serviceAddHandler failed, expect:%v-%v,reality:%v-%v",
					svcId, instId, inst.Endpoint.Labels["serviceid"], inst.Endpoint.Labels["instanceid"])
				return
			}
		}
	}

	fmt.Printf("*************** test serviceDelHandler ***************\n")

	m.serviceDelHandler(svcId)

	if _, ok := m.combSvcs[svcId]; ok {
		t.Errorf("serviceDelHandler failed, comb service %v exist", svcId)
		return
	}

	if _, ok := m.depSvcs[svcId]; ok {
		t.Errorf("serviceDelHandler failed, depSvcs service %v exist", svcId)
		return
	}

	for _, hostname := range hostnames {
		_, ok := m.services[hostname]
		if ok {
			t.Errorf("serviceAddHandler failed, service of %v exist", hostname)
		}

		_, ok = m.serviceInstances[hostname]
		if ok {
			t.Errorf("serviceAddHandler failed, instance of %v exist", hostname)
		}
	}
}

func TestInitialize(t *testing.T) {
	m, _ := NewCombMonitor(&gTestOpt)
	err := m.initialize()
	if err != nil {
		t.Errorf("initialize failed, %v", err)
	}
	defer m.client.Close()
	defer m.client.UnregisterMicroService(m.consumerId)

	if !m.initDone || len(m.consumerId) == 0 {
		t.Errorf("initialize failed, %v", m)
	}
}

var (
	gTestMonitor *combMonitor
	gTestSvcId   string
	gTestInsIds  []string
)

func testMonitorPre(t *testing.T) error {
	var err error
	gTestMonitor, err = NewCombMonitor(&gTestOpt)
	if err != nil {
		t.Errorf("NewCombMonitor failed, %v", err)
		return err
	}

	err = gTestMonitor.client.Initialize(*gTestMonitor.opt)
	if err != nil {
		t.Errorf("Initialize client failed,%v", err)
		return err
	}

	defer gTestMonitor.client.Close()

	gTestInsIds = []string{}

	testCvsSvc.ServiceId = ""
	gTestSvcId, err = gTestMonitor.client.RegisterService(&testCvsSvc)
	if err != nil {
		t.Errorf("RegisterService failed, %v", err)
		return err
	}

	gTestInsIds = []string{}
	for _, ins := range testCvsInsts {
		ins.ServiceId = gTestSvcId
		ins.InstanceId = ""
		insId, err := gTestMonitor.client.RegisterMicroServiceInstance(&ins)
		if err != nil {
			t.Errorf("RegisterMicroServiceInstance failed, %v, %+v", err, ins)
			_, _ = gTestMonitor.client.UnregisterMicroService(gTestSvcId)
			gTestMonitor.client.Close()
			return err
		}
		gTestInsIds = append(gTestInsIds, insId)
	}
	return nil
}

func testMonitorEnd(t *testing.T) {
	_, _ = gTestMonitor.client.UnregisterMicroService(gTestSvcId)
	_, _ = gTestMonitor.client.UnregisterMicroService(gTestMonitor.consumerId)
	gTestMonitor.client.Close()
}

func TestMonitorStart(t *testing.T) {
	fmt.Printf("*************** TestMonitorStart ***************\n")
	if nil != testMonitorPre(t) {
		return
	}
	defer testMonitorEnd(t)

	stop := make(chan struct{})
	gTestMonitor.Start(stop)
	defer close(stop)

	time.Sleep(periodicCheckTime)

	if !gTestMonitor.initDone || len(gTestMonitor.consumerId) == 0 {
		t.Errorf("initialize failed, %v", gTestMonitor)
	}

	svcNum := len(gTestMonitor.services)

	combSvcNum := 0
	for _, v := range gTestMonitor.combSvcs {
		combSvcNum += len(v)
	}

	_, ok := gTestMonitor.combSvcs[gTestSvcId]

	if combSvcNum != svcNum || !ok {
		t.Errorf("initCache failed, services:%+v, combSvcs: %+v", gTestMonitor.services, gTestMonitor.combSvcs)
	}

	insNum := len(gTestMonitor.serviceInstances)

	if insNum < len(testCvsInsts) {
		t.Errorf("initCache failed, mointor.serviceInstances: %+v", gTestMonitor.serviceInstances)
	}
}

func TestMonitorAddDelService(t *testing.T) {
	fmt.Printf("*************** TestMonitorAddDelService ***************\n")
	time.Sleep(2 * periodicCheckTime)
	if nil != testMonitorPre(t) {
		return
	}
	defer testMonitorEnd(t)

	stop := make(chan struct{})
	gTestMonitor.Start(stop)
	defer close(stop)
	oldSvcNum := len(gTestMonitor.services)
	oldInstNum := len(gTestMonitor.serviceInstances)

	t.Logf("*************** test add services ***************")
	newService := testCvsSvc
	newService.ServiceId = ""
	newService.ServiceName = "newtest"
	serviceId, err := gTestMonitor.client.RegisterService(&newService)
	if err != nil {
		t.Errorf("RegisterService failed, %v", err)
		return
	}

	inst := testCvsInsts[0]
	inst.ServiceId = serviceId
	inst.InstanceId = ""

	_, err = gTestMonitor.client.RegisterMicroServiceInstance(&inst)
	if err != nil {
		t.Errorf("RegisterMicroServiceInstance failed, %v", err)
		_, _ = gTestMonitor.client.UnregisterMicroService(serviceId)
		return
	}

	time.Sleep(2 * periodicCheckTime)

	newSvcNum := len(gTestMonitor.services)
	newInstNum := len(gTestMonitor.serviceInstances)

	if _, ok := gTestMonitor.combSvcs[serviceId]; !ok {
		t.Errorf("add service failed, combSvcs=%v", gTestMonitor.combSvcs)
		_, _ = gTestMonitor.client.UnregisterMicroService(serviceId)
		return
	}

	if newSvcNum <= oldSvcNum || newInstNum <= oldInstNum {
		t.Errorf("add service failed, oldSvcNum=%d, newSvcNum=%d, oldInstNum=%v, newInstNum==%v",
			newSvcNum, oldSvcNum, newInstNum, oldInstNum)
		_, _ = gTestMonitor.client.UnregisterMicroService(serviceId)
		return
	}

	t.Logf("*************** test delete services ***************")

	gTestMonitor.client.UnregisterMicroService(serviceId)

	time.Sleep(2 * periodicCheckTime)

	newSvcNum = len(gTestMonitor.services)
	newInstNum = len(gTestMonitor.serviceInstances)

	if _, ok := gTestMonitor.combSvcs[serviceId]; ok {
		t.Errorf("delete service failed, combSvcs=%v", gTestMonitor.combSvcs)
		return
	}

	if newSvcNum != oldSvcNum || newInstNum != oldInstNum {
		t.Errorf("delete service failed, oldSvcNum=%d, newSvcNum=%d, oldInstNum=%v, newInstNum==%v",
			newSvcNum, oldSvcNum, newInstNum, oldInstNum)
		return
	}
}

func TestMonitorAddUpdateDeleteInst(t *testing.T) {
	fmt.Printf("*************** TestMonitorAddUpdateDeleteInst ***************\n")
	time.Sleep(2 * periodicCheckTime)
	if nil != testMonitorPre(t) {
		return
	}
	defer testMonitorEnd(t)

	stop := make(chan struct{})
	gTestMonitor.Start(stop)
	defer close(stop)

	time.Sleep(2 * periodicCheckTime)

	newInst := proto.MicroServiceInstance{
		InstanceId:     "",
		HostName:       "test4",
		ServiceId:      gTestSvcId,
		Status:         "UP",
		Endpoints:      []string{"udp://127.0.0.4:8080"},
		Properties:     map[string]string{"a": "b", "extPlane_fabric": "tcp://192.168.0.4:80", "extPlane_om": "tcp://172.16.0.4:808"},
		DataCenterInfo: &proto.DataCenterInfo{Name: "dc", Region: "bj", AvailableZone: "az4"},
	}

	t.Logf("*************** test add instance ***************")

	instId, err := gTestMonitor.client.RegisterMicroServiceInstance(&newInst)
	if err != nil {
		t.Errorf("RegisterMicroServiceInstance failed, %v", err)
		return
	}

	time.Sleep(3 * periodicCheckTime)

	hostnames := gTestMonitor.combSvcs[gTestSvcId]

	insNum := 0

	for _, hostname := range hostnames {
		for _, inst := range gTestMonitor.serviceInstances[hostname] {
			if inst.Endpoint.Labels["instanceid"] == instId {
				insNum++
			}
		}
	}

	if insNum != 3 {
		t.Errorf("fail to detect adding instance, instNum=%v", insNum)
		return
	}

	t.Logf("*************** test update instance ***************")

	newInst.Properties["updatetestlabel"] = "updatetestlabel"

	newInst.InstanceId = instId

	_, err = gTestMonitor.client.UpdateMicroServiceInstanceProperties(gTestSvcId, instId, &newInst)
	if err != nil {
		t.Errorf("UpdateMicroServiceInstanceProperties failed, %v", err)
		return
	}

	time.Sleep(3 * periodicCheckTime)

	for _, hostname := range hostnames {
		for _, inst := range gTestMonitor.serviceInstances[hostname] {
			if inst.Endpoint.Labels["instanceid"] == instId {
				label, ok := inst.Endpoint.Labels["updatetestlabel"]
				if !ok || label != "updatetestlabel" {
					t.Errorf("instance have not been updated, endpoint:%+v", *inst.Endpoint)
					return
				}
			}
		}
	}

	t.Logf("*************** test delete instance ***************")
	_, err = gTestMonitor.client.UnregisterMicroServiceInstance(gTestSvcId, instId)
	if err != nil {
		t.Errorf("UnregisterMicroServiceInstance failed, %v", err)
		return
	}

	time.Sleep(3 * periodicCheckTime)

	for _, hostname := range hostnames {
		for _, inst := range gTestMonitor.serviceInstances[hostname] {
			if inst.Endpoint.Labels["instanceid"] == instId {
				t.Errorf("UnregisterMicroServiceInstance failed, %v", *inst)
				return
			}
		}
	}
}

func TestService(t *testing.T) {
	if nil != testMonitorPre(t) {
		return
	}
	defer testMonitorEnd(t)
	stop := make(chan struct{})
	gTestMonitor.Start(stop)
	defer close(stop)
	time.Sleep(periodicCheckTime)

	svcNum := len(gTestMonitor.services)

	svcList, err := gTestMonitor.Services()

	if err != nil {
		t.Errorf("Services failed, %v", err)
	}

	if svcNum < 1 || svcNum != len(svcList) {
		t.Errorf("Services failed, svcList: %v", svcList)
	}

}
