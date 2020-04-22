// // Copyright 2017 Istio Authors
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

package comb

import (
	"testing"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/config/labels"
)

func TestNewController(t *testing.T) {
	ctrl, err := NewController(gTestCenterAddr, "testclustid")
	if err != nil {
		t.Errorf("NewController failed, %v", err)
	}
	if ctrl.clusterID != "testclustid" || ctrl.monitor == nil {
		t.Errorf("NewController failed, %+v", ctrl)
	}
}

func TestInstancesByPortr(t *testing.T) {
	err := testMonitorPre(t)
	if err != nil {
		t.Errorf("Prepare testing failed, %v", err)
		return
	}
	defer testMonitorEnd(t)

	controller := Controller{
		monitor:   gTestMonitor,
		clusterID: "testclusterid",
	}

	err = gTestMonitor.initialize()
	if err != nil || !gTestMonitor.initDone {
		t.Errorf("initialize monitor failed, %v, monitor: %+v", err, gTestMonitor)
		return
	}

	hostnames, ok := gTestMonitor.combSvcs[gTestSvcId]

	if !ok {
		t.Errorf("register service failed, monitor: %+v", gTestMonitor)
		return
	}

	svc := gTestMonitor.services[hostnames[0]]
	port := svc.Ports[0].Port
	l := testCvsInsts[0].Properties
	l["serviceid"] = gTestSvcId
	l["instanceid"] = gTestInsIds[0]

	insts, err := controller.InstancesByPort(svc, port, labels.Collection{l})
	if err != nil || len(insts) == 0 {
		t.Errorf("InstancesByPort failed, %v, instances: %v", err, insts)
		return
	}
}

func TestGetProxyServiceInstances(t *testing.T) {
	err := testMonitorPre(t)
	if err != nil {
		t.Errorf("Prepare testing failed, %v", err)
		return
	}
	defer testMonitorEnd(t)
	err = gTestMonitor.initialize()
	if err != nil || !gTestMonitor.initDone {
		t.Errorf("initialize monitor failed, %v, monitor: %+v", err, gTestMonitor)
		return
	}

	controller := Controller{
		monitor:   gTestMonitor,
		clusterID: "testclusterid",
	}

	node := model.Proxy{IPAddresses: []string{"127.0.0.1", "192.168.0.1"}}

	insts, err := controller.GetProxyServiceInstances(&node)
	if err != nil {
		t.Errorf("GetProxyServiceInstances failed, %v", err)
	}

	if len(insts) != 2 {
		t.Errorf("GetProxyServiceInstances failed, %v, insts:%v", err, insts)
	}
}

func TestGetProxyWorkloadLabels(t *testing.T) {
	err := testMonitorPre(t)
	if err != nil {
		t.Errorf("Prepare testing failed, %v", err)
		return
	}
	defer testMonitorEnd(t)
	err = gTestMonitor.initialize()
	if err != nil || !gTestMonitor.initDone {
		t.Errorf("initialize monitor failed, %v, monitor: %+v", err, gTestMonitor)
		return
	}

	controller := Controller{
		monitor:   gTestMonitor,
		clusterID: "testclusterid",
	}

	node := model.Proxy{IPAddresses: []string{"127.0.0.1", "192.168.0.1"}}

	l, err := controller.GetProxyWorkloadLabels(&node)
	if err != nil {
		t.Errorf("GetProxyServiceInstances failed, %v", err)
	}

	if len(l) != 2 {
		t.Errorf("GetProxyServiceInstances failed, %v, insts:%v", err, l)
	}
}
