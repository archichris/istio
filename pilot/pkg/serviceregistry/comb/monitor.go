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

	depConsumer = client.DependencyMicroService{
		AppID:       monitorSvc.AppId,
		ServiceName: monitorSvc.ServiceName,
		Version:     monitorSvc.Version,
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
	doUpdateWatch    bool
	combSvcs         map[string][]string //comb service ID to service hostnames
	depSvcs          map[string]*client.DependencyMicroService
	// combSvcTstps     map[string]string
	// combInsTstps     map[string]string
	opt          *client.Options
	svcHandlers  []func(*model.Service, model.Event)
	instHandlers []func(*model.ServiceInstance, model.Event)
}

var (
	periodicCheckTime time.Duration = 2 * time.Second
)

// NewCombMonitor watches for changes in Consul services and CatalogServices
func NewCombMonitor(opt *client.Options) (*combMonitor, error) {
	r := &client.RegistryClient{}
	// err := r.Initialize(*opt)
	// if err != nil {
	// 	return nil, err
	// }
	return &combMonitor{
		client:   r,
		services: make(map[string]*model.Service),
		// servicesList:     []*model.Service{},
		serviceInstances: make(map[string][]*model.ServiceInstance),
		initDone:         false,
		doUpdateWatch:    true,
		combSvcs:         make(map[string][]string),
		depSvcs:          make(map[string]*client.DependencyMicroService),
		opt:              opt,
		svcHandlers:      make([]func(*model.Service, model.Event), 0),
		instHandlers:     make([]func(*model.ServiceInstance, model.Event), 0),
	}, nil
}

func (m *combMonitor) regSelf() error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	svcId, err := m.client.RegisterService(&monitorSvc)
	if err != nil {
		log.Errorf("[Comb] Register self service failed, %v", err)
		return err
	}
	m.consumerId = svcId
	log.Infof("[Comb] ServiceComb's consumer ID is %s", m.consumerId)
	return nil
}

func (m *combMonitor) updateWatch() error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if !m.doUpdateWatch {
		return nil
	}

	if len(m.depSvcs) == 0 {
		return nil
	}

	providers := []*client.DependencyMicroService{}

	for _, v := range m.depSvcs {
		providers = append(providers, v)

	}

	request := client.MircroServiceDependencyRequest{
		Dependencies: []*client.MicroServiceDependency{
			&client.MicroServiceDependency{
				Consumer:  &depConsumer,
				Providers: providers,
			},
		},
	}

	err := m.client.AddDependencies(&request)
	if err != nil {
		log.Errorf("[Comb] AddDependencies %+v failed, %v", request, err)
		return err
	}

	log.Infof("[dbg][Comb] watch dependencies: %v", m.depSvcs)

	m.doUpdateWatch = false
	return nil
}

func (m *combMonitor) serviceAddHandler(service *proto.MicroService) error {

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if _, ok := excludeSvcNames[service.ServiceName]; ok {
		return nil
	}

	provider := client.DependencyMicroService{
		AppID:       service.AppId,
		ServiceName: service.ServiceName,
		Version:     service.Version,
	}

	// watchKey := strings.Join([]string{service.AppId, service.ServiceName, service.Version}, "_")

	if _, ok := m.depSvcs[service.ServiceId]; !ok {
		m.depSvcs[service.ServiceId] = &provider
		m.doUpdateWatch = true
	}

	endpoints, err := m.client.GetMicroServiceInstances(m.consumerId, service.ServiceId)
	if err != nil {
		log.Errorf("[Comb] GetMicroServiceInstances failed, err: %v", err)
		return err
	} else if endpoints == nil {
		// log.Warnf("[Comb] %s have no instance", service.ServiceName)
		return nil
	}

	log.Infof("[dbg][Comb] New comb service: %v-%v", service.ServiceName, service.ServiceId)
	svcs := convertService(service, endpoints)
	m.combSvcs[service.ServiceId] = []string{}
	for _, svc := range svcs {
		log.Infof("[dbg][Comb] Add new istio service: %v-%v-%v", svc.Hostname, svc.Address, svc.Ports)
		m.services[string(svc.Hostname)] = svc
		m.combSvcs[service.ServiceId] = append(m.combSvcs[service.ServiceId], string(svc.Hostname))
		// m.combSvcTstps[service.ServiceId] = service.ModTimestamp
		instances := []*model.ServiceInstance{}
		for _, endpoint := range endpoints {
			if endpoint.Status == "UP" {
				instances = append(instances, convertInstance(svc, endpoint)...)
			}
			// m.combInsTstps[endpoint.InstanceId] = endpoint.ModTimestamp
		}
		m.serviceInstances[string(svc.Hostname)] = instances
		for _, ins := range instances {
			log.Infof("[dbg][Comb] Add new istio instance: %v-%v-%v", ins.Endpoint.Address, ins.Endpoint.EndpointPort, ins.Endpoint.Labels)
		}
	}
	return nil
}

func (m *combMonitor) serviceDelHandler(id string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	log.Infof("[dbg][Comb] delete comb service: %v", id)
	hostnames := m.combSvcs[id]
	for _, hostname := range hostnames {
		delete(m.serviceInstances, hostname)
		delete(m.services, hostname)
		log.Infof("[dbg][Comb] delete istio service: %v", hostname)
	}
	delete(m.combSvcs, id)
	delete(m.depSvcs, id)
}

func (m *combMonitor) initialize() error {

	if m.initDone {
		return nil
	}

	err := m.client.Initialize(*m.opt)
	if err != nil {
		return err
	}

	err = m.regSelf()
	if err != nil {
		m.client.Close()
		return err
	}

	m.services = make(map[string]*model.Service)
	m.serviceInstances = make(map[string][]*model.ServiceInstance)
	m.combSvcs = make(map[string][]string)

	// get all services from servicecomb
	services, err := m.client.GetAllMicroServices()
	if err != nil {
		m.client.Close()
		return err
	}

	for _, service := range services {
		err := m.serviceAddHandler(service)
		if err != nil {
			m.client.Close()
			return err
		}
	}

	err = m.client.WatchMicroService(m.consumerId, m.watchIns)
	if err != nil {
		log.Errorf("[Comb] WatchMicroService failed, %v", err)
		m.client.Close()
		return err
	}
	m.initDone = true
	log.Infof("[dbg][Comb] monitor is initialized successfully")
	return nil
}

func (m *combMonitor) Start(stop <-chan struct{}) {
	_ = m.initialize()
	change := make(chan struct{})
	go m.watchComb(change, stop)
}

func (m *combMonitor) SyncSvc() ([]*proto.MicroService, []string, error) {
	services, err := m.client.GetAllMicroServices()
	if err != nil {
		log.Errorf("[Comb] GetAllMicroServices failed, %v", err)
		return nil, nil, err
	}
	svcCur := map[string]bool{}
	addServices := []*proto.MicroService{}
	delServiceIds := []string{}
	for _, service := range services {
		if _, ok := excludeSvcNames[service.ServiceName]; ok {
			continue
		}
		if _, ok := m.combSvcs[service.ServiceId]; !ok {
			addServices = append(addServices, service)
		}
		svcCur[service.ServiceId] = true
	}
	for id := range m.combSvcs {
		if _, ok := svcCur[id]; !ok {
			delServiceIds = append(delServiceIds, id)
		}
	}
	return addServices, delServiceIds, nil
}

func (m *combMonitor) loopProc() {
	err := m.initialize()
	if err == nil {
		addSvcs, delSvcIds, err := m.SyncSvc()
		if err == nil {
			for _, svc := range addSvcs {
				err = m.serviceAddHandler(svc)
				if err != nil {
					log.Errorf("[Comb] serviceAddHandler failed, %v", err)
					continue
				}
			}
			for _, id := range delSvcIds {
				m.serviceDelHandler(id)
			}
			_ = m.updateWatch()
		}

	}
}

func (m *combMonitor) watchComb(change chan struct{}, stop <-chan struct{}) {
	log.Infof("[dbg][Comb] Start to watch ServiceCenter")
	for {
		select {
		case <-stop:
			if len(m.consumerId) != 0 {
				_, _ = m.client.UnregisterMicroService(m.consumerId)
			}
			m.client.Close()
			return
		default:
			m.loopProc()
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
	case client.EventCreate, client.EventUpdate:
		m.insAddUpdateAction(response)
	case client.EventDelete:
		m.insDeleteAction(response)
	default:
		log.Warnf("[Comb] Do not support this Action = %s", response.Action)
	}
}

// createAction added micro-service instance to the cache
func (m *combMonitor) insAddUpdateAction(response *client.MicroServiceInstanceChangedEvent) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	id := response.Instance.ServiceId
	hostnames := m.combSvcs[id]
	addInstances := []*model.ServiceInstance{}
	updateInstances := []*model.ServiceInstance{}

	for _, hostname := range hostnames {
		instances := convertInstance(m.services[hostname], response.Instance)
		tmpAddInst := []*model.ServiceInstance{}
		for _, newIns := range instances {
			update := false
			for i, curIns := range m.serviceInstances[hostname] {
				if newIns.Endpoint.Labels["instanceid"] == curIns.Endpoint.Labels["instanceid"] &&
					newIns.ServicePort.Name == curIns.ServicePort.Name {
					updateInstances = append(updateInstances, newIns)
					m.serviceInstances[hostname][i] = newIns
					update = true
					log.Infof("[dbg][Comb] Update istio instance: %v-%v-%v", newIns.Endpoint.Address, newIns.Endpoint.EndpointPort, newIns.Endpoint.Labels)
					break
				}
			}
			if !update {
				tmpAddInst = append(tmpAddInst, newIns)
				log.Infof("[dbg][Comb] Add istio instance: %v-%v-%v", newIns.Endpoint.Address, newIns.Endpoint.EndpointPort, newIns.Endpoint.Labels)
			}
		}
		if len(tmpAddInst) != 0 {
			m.serviceInstances[hostname] = append(m.serviceInstances[hostname], tmpAddInst...)
			addInstances = append(addInstances, tmpAddInst...)
		}
	}

	for _, ins := range addInstances {
		for _, f := range m.instHandlers {
			f(ins, model.EventAdd)
		}
	}

	for _, ins := range updateInstances {
		for _, f := range m.instHandlers {
			f(ins, model.EventUpdate)
		}
	}

}

// deleteAction delete micro-service instance
func (m *combMonitor) insDeleteAction(response *client.MicroServiceInstanceChangedEvent) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	id := response.Instance.ServiceId
	hostnames := m.combSvcs[id]

	for _, hostname := range hostnames {
		instances := m.serviceInstances[hostname]
		for i, instance := range instances {
			if instance.Endpoint.Labels["instanceid"] == response.Instance.InstanceId {
				if i != len(instances)-1 {
					m.serviceInstances[hostname] = append(instances[:i], instances[i+1:]...)
				} else {
					m.serviceInstances[hostname] = instances[:i]
				}
				log.Infof("[dbg][Comb] Delete istio instance: %v-%v-%v", instance.Endpoint.Address, instance.Endpoint.EndpointPort, instance.Endpoint.Labels)
				for _, f := range m.instHandlers {
					f(instance, model.EventDelete)
				}
				break
			}
		}
	}
}

func (m *combMonitor) Services() ([]*model.Service, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	err := m.initialize()
	if err != nil {
		return nil, err
	}

	servicesList := make([]*model.Service, 0, len(m.services))
	for _, value := range m.services {
		servicesList = append(servicesList, value)
	}

	return servicesList, nil
}
