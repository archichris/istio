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

// var (
// 	registryAddr = "192.168.56.31:30543"
// 	testSvc      = proto.MicroService{
// 		// ServiceId:   "serviceid",
// 		AppId:       "appid",
// 		ServiceName: "test",
// 		Version:     "0.0.1",
// 		Properties:  map[string]string{"t1": "t1", "t2": "t2"},
// 	}
// )

// func TestServices(t *testing.T) {
// 	ctrl, err := NewController(registryAddr, "")
// 	if ctrl == nil || err != nil {
// 		t.Errorf("Create NetController failed, ctrl:%v, err:%v", ctrl, err)
// 	}
// 	serviceid, err := ctrl.client.RegisterService(&testSvc)
// 	ctrl.monitor.initCache()
// 	if len(serviceid) == 0 || err != nil {
// 		t.Errorf("RegisterService %v failed, serviceid:%v, err:%v", testSvc, serviceid, err)
// 	}

// 	ss, err := ctrl.Services()
// 	if len(ss) != 1 || err != nil {
// 		t.Errorf("Services failed, svcs:%v, err:%v", ss, err)
// 	}

// }
