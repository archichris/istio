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
	"sync"
	"time"

	"github.com/go-chassis/go-chassis/core/common"
	// "github.com/go-chassis/go-chassis/core/registry"
	client "github.com/go-chassis/go-chassis/pkg/scclient"
	"github.com/go-chassis/go-chassis/pkg/scclient/proto"

	// "github.com/go-mesh/openlogging"
	// "github.com/hashicorp/consul/api"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/pkg/log"
)

var (
	monitorSvc = proto.MicroService{
		AppId:       "default",
		ServiceName: "IstioMonitor",
		Version:     "0.0.1",
	}
	excludeSvcNames = map[string]bool{
		monitorSvc.ServiceName: true,
		"SERVICECENTER":        true,
	}
)

type combMonitor struct {
	client     *client.RegistryClient
	consumerId string
	services   map[string]*model.Service //key hostname value service
	// servicesList     []*model.Service
	serviceInstances map[string][]*model.ServiceInstance //key hostname value serviceInstance array
	cacheMutex       sync.Mutex
	initDone         bool
	combSvc          map[string][]string //comb service ID to service hostnames
	svcHandlers      []func(*model.Service, model.Event)
	instHandlers     []func(*model.ServiceInstance, model.Event)
}

const (
	periodicCheckTime time.Duration = 2 * time.Second
)

// NewCombMonitor watches for changes in Consul services and CatalogServices
func NewCombMonitor(opt *client.Options) (*combMonitor, error) {
	r := &client.RegistryClient{}
	err := r.Initialize(*opt)
	if err != nil {
		return nil, err
	}
	return &combMonitor{
		client:   r,
		services: make(map[string]*model.Service),
		// servicesList:     []*model.Service{},
		serviceInstances: make(map[string][]*model.ServiceInstance),
		initDone:         false,
		combSvc:          make(map[string][]string),
		svcHandlers:      make([]func(*model.Service, model.Event), 0),
		instHandlers:     make([]func(*model.ServiceInstance, model.Event), 0),
	}, nil
}

func (m *combMonitor) regSelf() error {
	svcId, err := m.client.RegisterService(&monitorSvc)
	if err != nil {
		log.Errorf("Register self service failed, %v", err)
		return err
	}
	m.consumerId = svcId
	return nil
}

func (m *combMonitor) serviceAddHandler(service *proto.MicroService) error {

	if _, ok := excludeSvcNames[service.ServiceName]; ok {
		return nil
	}

	endpoints, err := m.client.GetMicroServiceInstances(m.consumerId, service.ServiceId)
	if err != nil {
		log.Errorf("GetMicroServiceInstances failed, err: %v", err)
		return err
	} else if endpoints == nil {
		log.Warnf("%s have no instance", service.ServiceName)
		return nil
	}

	svcs := convertService(service, endpoints)
	for _, svc := range svcs {
		m.services[string(svc.Hostname)] = svc
		m.combSvc[service.ServiceId] = append(m.combSvc[service.ServiceId], string(svc.Hostname))
		instances := []*model.ServiceInstance{}
		for _, endpoint := range endpoints {
			if endpoint.Status == "UP" {
				instances = append(instances, convertInstance(svc, endpoint)...)
			}
		}
		m.serviceInstances[string(svc.Hostname)] = instances
	}
	return nil
}

func (m *combMonitor) serviceDelHandler(id string) {
	hostnames := m.combSvc[id]
	for _, hostname := range hostnames {
		delete(m.serviceInstances, hostname)
		delete(m.services, hostname)
	}
	delete(m.combSvc, id)
}

func (m *combMonitor) initCache() error {
	if m.initDone {
		return nil
	}
	m.services = make(map[string]*model.Service)
	m.serviceInstances = make(map[string][]*model.ServiceInstance)
	m.combSvc = make(map[string][]string)

	// get all services from servicecomb
	services, err := m.client.GetAllMicroServices()
	if err != nil {
		return err
	}

	for _, service := range services {
		err := m.serviceAddHandler(service)
		if err == nil {
			m.initDone = true
		}
	}
	return nil
}

func (m *combMonitor) Start(stop <-chan struct{}) {
	err := m.regSelf()
	if err == nil {
		_ = m.initCache()
	}
	change := make(chan struct{})
	go m.watchComb(change, stop)
}

func (m *combMonitor) SyncSvc() ([]*proto.MicroService, []string, error) {
	services, err := m.client.GetAllMicroServices()
	if err != nil {
		log.Errorf("GetAllMicroServices failed, %v", err)
		return nil, nil, err
	}
	svcCur := map[string]bool{}
	addServices := []*proto.MicroService{}
	delServiceIds := []string{}
	for _, service := range services {
		if _, ok := m.combSvc[service.ServiceId]; !ok {
			addServices = append(addServices, service)
		}
		svcCur[service.ServiceId] = true
	}
	for id := range m.combSvc {
		if _, ok := svcCur[id]; !ok {
			delServiceIds = append(delServiceIds, id)
		}
	}
	return addServices, delServiceIds, nil
}

func (m *combMonitor) watchComb(change chan struct{}, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			err := m.regSelf()
			if err != nil {
				continue
			}

			if m.initCache() != nil {
				continue
			}

			addSvcs, delSvcIds, err := m.SyncSvc()
			if err != nil {
				continue
			}

			for _, svc := range addSvcs {
				err = m.serviceAddHandler(svc)
				if err != nil {
					log.Errorf("serviceAddHandler failed, %v", err)
					continue
				}
				err = m.client.WatchMicroService(svc.ServiceId, m.watchIns)
				if err != nil {
					log.Errorf("WatchMicroService failed, %v", err)
					continue
				}
			}

			for _, id := range delSvcIds {
				m.serviceDelHandler(id)
			}
			time.Sleep(periodicCheckTime)
		}
	}
}

func (m *combMonitor) AppendServiceHandler(h func(*model.Service, model.Event)) {
	m.svcHandlers = append(m.svcHandlers, h)
}

func (m *combMonitor) AppendInstanceHandler(h func(*model.ServiceInstance, model.Event)) {
	m.instHandlers = append(m.instHandlers, h)
}

// watch watching micro-service instance status
func (m *combMonitor) watchIns(response *client.MicroServiceInstanceChangedEvent) {
	if response.Instance.Status != client.MSInstanceUP {
		response.Action = common.Delete
	}
	switch response.Action {
	case client.EventCreate:
		m.insCreateAction(response)
	case client.EventDelete:
		m.insDeleteAction(response)
	case client.EventUpdate:
		m.insUpdateAction(response)
	default:
		log.Warnf("Do not support this Action = %s", response.Action)
	}
}

// createAction added micro-service instance to the cache
func (m *combMonitor) insCreateAction(response *client.MicroServiceInstanceChangedEvent) {
	id := response.Instance.ServiceId
	hostnames := m.combSvc[id]
	for _, hostname := range hostnames {
		instances := convertInstance(m.services[hostname], response.Instance)
		m.serviceInstances[hostname] = append(m.serviceInstances[hostname], instances...)
		for _, instance := range instances {
			for _, f := range m.instHandlers {
				f(instance, model.EventAdd)
			}
		}
	}
}

// deleteAction delete micro-service instance
func (m *combMonitor) insDeleteAction(response *client.MicroServiceInstanceChangedEvent) {

	id := response.Instance.ServiceId
	hostnames := m.combSvc[id]

	for _, hostname := range hostnames {
		instances := m.serviceInstances[hostname]
		for i, instance := range instances {
			if instance.Endpoint.Labels["instanceid"] == response.Instance.InstanceId {
				if i != len(instances)-1 {
					m.serviceInstances[hostname] = append(instances[:i], instances[i+1:]...)
				} else {
					m.serviceInstances[hostname] = instances[:i]
				}
				for _, f := range m.instHandlers {
					f(instance, model.EventDelete)
				}
				break
			}
		}
	}
}

// updateAction update micro-service instance event
func (m *combMonitor) insUpdateAction(response *client.MicroServiceInstanceChangedEvent) {
	id := response.Instance.ServiceId
	hostnames := m.combSvc[id]
	for _, hostname := range hostnames {
		instances := m.serviceInstances[hostname]
		instanceCurs := convertInstance(m.services[hostname], response.Instance)
		for i, instance := range instances {
			find := false
			for _, instCur := range instanceCurs {
				if instance.Endpoint.Labels["instanceid"] == instCur.Endpoint.Labels["instanceid"] &&
					instance.Endpoint.ServicePortName == instCur.Endpoint.ServicePortName &&
					instance.Endpoint.EndpointPort == instCur.Endpoint.EndpointPort {
					instances[i] = instCur
					find = true
					for _, f := range m.instHandlers {
						f(instCur, model.EventUpdate)
					}
					break
				}
			}
			if !find {
				m.insCreateAction(response)
			}
		}
	}
}

func (m *combMonitor) Services() ([]*model.Service, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	err := m.initCache()

	if err != nil {
		return nil, err
	}

	servicesList := make([]*model.Service, 0, len(m.services))
	for _, value := range m.services {
		servicesList = append(servicesList, value)
	}

	return servicesList, nil
}
