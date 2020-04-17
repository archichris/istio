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

// Monitor handles service and instance changes
type Monitor interface {
	Start(<-chan struct{})
	AppendServiceHandler(func(*model.Service, model.Event))
	AppendInstanceHandler(func(*model.ServiceInstance, model.Event))
}

// // InstanceHandler processes service instance change events
// type InstanceHandler func(instance *api.CatalogService, event model.Event) error

// // ServiceHandler processes service change events
// type ServiceHandler func(instances []*api.CatalogService, event model.Event) error

type combMonitor struct {
	discovery *client.RegistryClient
	// instanceHandlers []InstanceHandler
	// serviceHandlers  []ServiceHandler
	services         map[string]*model.Service //key hostname value service
	servicesList     []*model.Service
	serviceInstances map[string][]*model.ServiceInstance //key hostname value serviceInstance array
	cacheMutex       sync.Mutex
	initDone         bool
	combSvc          map[string][]string //comb service ID to service hostnames
	svcHandlers      []func(*model.Service, model.Event)
	instHandlers     []func(*model.ServiceInstance, model.Event)
}

const (
	refreshIdleTime    time.Duration = 5 * time.Second
	periodicCheckTime  time.Duration = 2 * time.Second
	blockQueryWaitTime time.Duration = 10 * time.Minute
)

// NewCombMonitor watches for changes in Consul services and CatalogServices
func NewCombMonitor(client *client.RegistryClient) Monitor {
	return &combMonitor{
		discovery:        client,
		services:         make(map[string]*model.Service),
		servicesList:     make([]*model.Service, 0),
		serviceInstances: make(map[string][]*model.ServiceInstance),
		initDone:         false,
		combSvc:          make(map[string][]string),
		svcHandlers:      make([]func(*model.Service, model.Event), 0),
		instHandlers:     make([]func(*model.ServiceInstance, model.Event), 0),
	}
}

func (m *combMonitor) initCache() error {
	if m.initDone {
		return nil
	}
	m.services = make(map[string]*model.Service)
	m.serviceInstances = make(map[string][]*model.ServiceInstance)
	m.combSvc = make(map[string][]string)

	// get all services from servicecomb
	services, err := m.discovery.GetAllMicroServices()
	if err != nil {
		return err
	}

	for _, service := range services {
		// get endpoints of a service from consul

		endpoints, err := m.discovery.GetMicroServiceInstances("0", service.ServiceId)
		if err != nil {
			return err
		}

		svcs := convertService(service, endpoints)
		for _, svc := range svcs {
			m.services[string(svc.Hostname)] = svc
			m.combSvc[service.ServiceId] = append(m.combSvc[service.ServiceId], string(svc.Hostname))
			instances := []*model.ServiceInstance{}
			// instances := make([]*model.ServiceInstance, len(endpoints))
			for _, endpoint := range endpoints {
				// for _, instance := range convertInstance(svc, endpoint) {
				instances = append(instances, convertInstance(svc, endpoint)...)
			}
			m.serviceInstances[string(svc.Hostname)] = instances
		}
	}

	m.servicesList = make([]*model.Service, 0, len(m.services))
	for _, value := range m.services {
		m.servicesList = append(m.servicesList, value)
	}

	m.initDone = true
	return nil
}

func (m *combMonitor) Start(stop <-chan struct{}) {
	m.initCache()
	change := make(chan struct{})
	go m.watchComb(change, stop)
	// go m.updateRecord(change, stop)
}

func (m *combMonitor) watchComb(change chan struct{}, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			services, err := m.discovery.GetAllMicroServices()
			if err != nil {
				log.Errorf("GetAllMicroServices failed, %v", err)
				continue
			}
			svcCur := map[string]bool{}
			for _, service := range services {
				m.discovery.WatchMicroService(service.ServiceId, m.watch)
				if _, ok := m.combSvc[service.ServiceId]; !ok {
					m.serviceAdd(service)
				}
				svcCur[service.ServiceId] = true
			}
			for id := range m.combSvc {
				if _, ok := svcCur[id]; !ok {
					m.serviceDel(id)
				}
			}
			time.Sleep(periodicCheckTime)
		}
	}
}

func (m *combMonitor) serviceAdd(service *proto.MicroService) error {

	endpoints, err := m.discovery.GetMicroServiceInstances("0", service.ServiceId)
	if err != nil {
		return err
	}

	svcs := convertService(service, endpoints)
	for _, svc := range svcs {
		m.services[string(svc.Hostname)] = svc
		m.combSvc[service.ServiceId] = append(m.combSvc[service.ServiceId], string(svc.Hostname))
		instances := []*model.ServiceInstance{}
		// instances := make([]*model.ServiceInstance, len(endpoints))
		for _, endpoint := range endpoints {
			// for _, instance := range convertInstance(svc, endpoint) {
			instances = append(instances, convertInstance(svc, endpoint)...)
		}
		m.serviceInstances[string(svc.Hostname)] = instances
		m.servicesList = append(m.servicesList, svc)
	}
	return nil
}

func (m *combMonitor) serviceDel(id string) error {
	hostnames := m.combSvc[id]
	for _, hostname := range hostnames {
		delete(m.serviceInstances, hostname)
		delete(m.services, hostname)
	}
	delete(m.combSvc, id)
	return nil
}

func (m *combMonitor) AppendServiceHandler(h func(*model.Service, model.Event)) {
	m.svcHandlers = append(m.svcHandlers, h)
}

func (m *combMonitor) AppendInstanceHandler(h func(*model.ServiceInstance, model.Event)) {
	m.instHandlers = append(m.instHandlers, h)
}

// watch watching micro-service instance status
func (m *combMonitor) watch(response *client.MicroServiceInstanceChangedEvent) {
	if response.Instance.Status != client.MSInstanceUP {
		response.Action = common.Delete
	}
	switch response.Action {
	case client.EventCreate:
		m.createAction(response)
		break
	case client.EventDelete:
		m.deleteAction(response)
		break
	case client.EventUpdate:
		m.updateAction(response)
		break
	default:
		log.Warnf("Do not support this Action = %s", response.Action)
		return
	}
}

// createAction added micro-service instance to the cache
func (m *combMonitor) createAction(response *client.MicroServiceInstanceChangedEvent) {
	id := response.Instance.InstanceId
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
func (m *combMonitor) deleteAction(response *client.MicroServiceInstanceChangedEvent) {

	id := response.Instance.InstanceId
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
func (m *combMonitor) updateAction(response *client.MicroServiceInstanceChangedEvent) {
	id := response.Instance.InstanceId
	hostnames := m.combSvc[id]
	for _, hostname := range hostnames {
		instances := m.serviceInstances[hostname]
		instanceCurs := convertInstance(m.services[hostname], response.Instance)
		for i, instance := range instances {
			for _, instCur := range instanceCurs {
				if instance.Endpoint.Labels["instanceid"] == instCur.Endpoint.Labels["instanceid"] &&
					instance.Endpoint.ServicePortName == instCur.Endpoint.ServicePortName &&
					instance.Endpoint.EndpointPort == instCur.Endpoint.EndpointPort {
					instances[i] = instCur
					for _, f := range m.instHandlers {
						f(instCur, model.EventUpdate)
					}
					break
				}
			}
		}
	}
}
