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

//servcie comb
package comb

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-chassis/go-chassis/core/config"
	"github.com/go-chassis/go-chassis/core/registry"
	"github.com/go-chassis/go-chassis/core/registry/servicecenter"
	"github.com/hashicorp/consul/api"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/spiffe"
	"istio.io/pkg/log"
)

var _ serviceregistry.Instance = &Controller{}

// Controller communicates with Consul and monitors for changes
type Controller struct {
	discover         *registry.ServiceDiscovery
	monitor          Monitor
	services         map[string]*model.Service //key hostname value service
	servicesList     []*model.Service
	serviceInstances map[string][]*model.ServiceInstance //key hostname value serviceInstance array
	cacheMutex       sync.Mutex
	initDone         bool
	clusterID        string
}

// type ServiceDiscovery struct {
// 	Name           string
// 	registryClient *client.RegistryClient
// 	opts           client.Options
// }

func GetDiscoverOptions() (oSD registry.Options, err error) {
	hostsSD, schemeSD, err := registry.URIs2Hosts(strings.Split(config.GetServiceDiscoveryAddress(), ","))
	if err != nil {
		return
	}
	oSD.Addrs = hostsSD
	oSD.Tenant = config.GetServiceDiscoveryTenant()
	oSD.Version = config.GetServiceDiscoveryAPIVersion()
	oSD.ConfigPath = config.GetServiceDiscoveryConfigPath()
	oSD.TLSConfig, err = registry.getTLSConfig(schemeSD, "serviceDiscovery")
	if err != nil {
		return
	}
	if oSD.TLSConfig != nil {
		oSD.EnableSSL = true
	}
	return
}

// NewController creates a new Consul controller
func NewController(addr string, clusterID string) (*Controller, error) {
	if err := config.Init(); err != nil {
		log.Error("failed to initialize conf: " + err.Error())
		return nil, err
	}
	// var oR, oSD, oCD registry.Options
	oSD, err := GetDiscoverOptions()
	if err != nil {
		return nil, err
	}

	// servicecenter.NewServiceDiscovery(oSD)

	// sco := servicecenter.ToSCOptions(oSD)
	// r := &client.RegistryClient{}
	// if err := r.Initialize(sco); err != nil {
	// 	log.Errorf("RegistryClient initialization failed. %s", err)
	// 	return nil, err
	// }

	//  servicecenter.NewServiceDiscovery(oSD) registry.ServiceDiscovery {

	// registry.enableRegistryCache()
	// sco := servicecenter.ToSCOptions(oSD)
	// r := &client.RegistryClient{}
	// if err := r.Initialize(sco); err != nil {
	// 	log.Errorf("RegistryClient initialization failed. %s", err)
	// 	return nil, err
	// }

	controller := Controller{
		discover: servicecenter.NewServiceDiscovery(oSD),
	}

	return &controller, nil
}

func (c *Controller) Provider() serviceregistry.ProviderID {
	return serviceregistry.ServiceComb
}

// func (c *Controller) Cluster() string {
// 	return c.clusterID
// }

// Services list declarations of all services in the system
func (c *Controller) Services() ([]*model.Service, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	return c.servicesList, nil
}

// GetService retrieves a service by host name if it exists
func (c *Controller) GetService(hostname host.Name) (*model.Service, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	// Get actual service by name
	name, err := parseHostname(hostname)
	if err != nil {
		log.Infof("parseHostname(%s) => error %v", hostname, err)
		return nil, err
	}

	if service, ok := c.services[name]; ok {
		return service, nil
	}
	return nil, nil
}

// ManagementPorts retrieves set of health check ports by instance IP.
// This does not apply to Consul service registry, as Consul does not
// manage the service instances. In future, when we integrate Nomad, we
// might revisit this function.
func (c *Controller) ManagementPorts(addr string) model.PortList {
	return nil
}

// WorkloadHealthCheckInfo retrieves set of health check info by instance IP.
// This does not apply to Consul service registry, as Consul does not
// manage the service instances. In future, when we integrate Nomad, we
// might revisit this function.
func (c *Controller) WorkloadHealthCheckInfo(addr string) model.ProbeList {
	return nil
}

// InstancesByPort retrieves instances for a service that match
// any of the supplied labels. All instances match an empty tag list.
func (c *Controller) InstancesByPort(svc *model.Service, port int,
	labels labels.Collection) ([]*model.ServiceInstance, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	// Get actual service by name
	name, err := parseHostname(svc.Hostname)
	if err != nil {
		log.Infof("parseHostname(%s) => error %v", svc.Hostname, err)
		return nil, err
	}

	if serviceInstances, ok := c.serviceInstances[name]; ok {
		var instances []*model.ServiceInstance
		for _, instance := range serviceInstances {
			if labels.HasSubsetOf(instance.Endpoint.Labels) && portMatch(instance, port) {
				instances = append(instances, instance)
			}
		}

		return instances, nil
	}
	return nil, fmt.Errorf("could not find instance of service: %s", name)
}

// returns true if an instance's port matches with any in the provided list
func portMatch(instance *model.ServiceInstance, port int) bool {
	return port == 0 || port == instance.ServicePort.Port
}

// GetProxyServiceInstances lists service instances co-located with a given proxy
func (c *Controller) GetProxyServiceInstances(node *model.Proxy) ([]*model.ServiceInstance, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	out := make([]*model.ServiceInstance, 0)
	for _, instances := range c.serviceInstances {
		for _, instance := range instances {
			addr := instance.Endpoint.Address
			if len(node.IPAddresses) > 0 {
				for _, ipAddress := range node.IPAddresses {
					if ipAddress == addr {
						out = append(out, instance)
						break
					}
				}
			}
		}
	}

	return out, nil
}

func (c *Controller) GetProxyWorkloadLabels(proxy *model.Proxy) (labels.Collection, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	err := c.initCache()
	if err != nil {
		return nil, err
	}

	out := make(labels.Collection, 0)
	for _, instances := range c.serviceInstances {
		for _, instance := range instances {
			addr := instance.Endpoint.Address
			if len(proxy.IPAddresses) > 0 {
				for _, ipAddress := range proxy.IPAddresses {
					if ipAddress == addr {
						out = append(out, instance.Endpoint.Labels)
						break
					}
				}
			}
		}
	}

	return out, nil
}

// Run all controllers until a signal is received
func (c *Controller) Run(stop <-chan struct{}) {
	c.monitor.Start(stop)
}

// AppendServiceHandler implements a service catalog operation
func (c *Controller) AppendServiceHandler(f func(*model.Service, model.Event)) error {
	c.monitor.AppendServiceHandler(func(instances []*api.CatalogService, event model.Event) error {
		f(convertService(instances), event)
		return nil
	})
	return nil
}

// AppendInstanceHandler implements a service catalog operation
func (c *Controller) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
	c.monitor.AppendInstanceHandler(func(endpoint *registry.MicroServiceInstance, event model.Event) error {
		for _, instance := range convertInstance(endpoint) {
			f(instance, event)
		}
		return nil
	})
	return nil
}

// GetIstioServiceAccounts implements model.ServiceAccounts operation TODO
func (c *Controller) GetIstioServiceAccounts(svc *model.Service, ports []int) []string {
	// Need to get service account of service registered with consul
	// Currently Consul does not have service account or equivalent concept
	// As a step-1, to enabling istio security in Consul, We assume all the services run in default service account
	// This will allow all the consul services to do mTLS
	// Follow - https://goo.gl/Dt11Ct

	return []string{
		spiffe.MustGenSpiffeURI("default", "default"),
	}
}

func (c *Controller) initCache() error {
	if c.initDone {
		return nil
	}

	c.services = make(map[string]*model.Service)
	c.serviceInstances = make(map[string][]*model.ServiceInstance)

	// get all services from consul
	Services, err := c.discover.GetAllMicroServices()
	if err != nil {
		return err
	}

	for service := range Services {
		// get endpoints of a service from consul

		endpoints, err := c.discover.GetMicroServiceInstances(0, service.ServiceID)
		if err != nil {
			return err
		}
		c.services[service.serviceName] = convertService(service, endpoints)
		instances := []*model.ServiceInstance{}
		// instances := make([]*model.ServiceInstance, len(endpoints))
		for _, endpoint := range endpoints {
			for _, instance := range convertInstance(endpoint) {
				instances = append(instances, instance)
			}
		}
		c.serviceInstances[service.serviceName] = instances
	}

	c.servicesList = make([]*model.Service, 0, len(c.services))
	for _, value := range c.services {
		c.servicesList = append(c.servicesList, value)
	}

	c.initDone = true
	return nil
}

// func (c *Controller) getServices() (map[string][]string, error) {
// 	data, _, err := c.discover.GetAllMicroServices()
// 	if err != nil {
// 		log.Warnf("Could not retrieve services from consul: %v", err)
// 		return nil, err
// 	}

// 	return data, nil
// }

// nolint: unparam
// func (c *Controller) getCatalogService(name string, q *api.QueryOptions) ([]*api.CatalogService, error) {
// 	endpoints, _, err := c.client.Catalog().Service(name, "", q)
// 	if err != nil {
// 		log.Warnf("Could not retrieve service catalog from consul: %v", err)
// 		return nil, err
// 	}

// 	return endpoints, nil
// }

func (c *Controller) refreshCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.initDone = false
}

func (c *Controller) InstanceChanged(instance *registry.MicroServiceInstance, event model.Event) error {
	c.refreshCache()
	return nil
}

func (c *Controller) ServiceChanged(instances []*registry.MicroService, event model.Event) error {
	c.refreshCache()
	return nil
}
