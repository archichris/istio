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
	"os"
	"time"

	// "github.com/go-chassis/go-chassis/core/config"
	// "github.com/go-chassis/go-chassis/core/registry/servicecenter"
	client "github.com/go-chassis/go-chassis/pkg/scclient"
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
	client    *client.RegistryClient
	monitor   combMonitor
	clusterID string
}

// func getTLSConfig(scheme, t string) (*tls.Config, error) {
// 	var tlsConfig *tls.Config
// 	secure := scheme == common.HTTPS
// 	if secure {
// 		sslTag := t + "." + common.Consumer
// 		tmpTLSConfig, sslConfig, err := chassisTLS.GetTLSConfigByService(t, "", common.Consumer)
// 		if err != nil {
// 			if chassisTLS.IsSSLConfigNotExist(err) {
// 				tmpErr := fmt.Errorf("%s tls mode, but no ssl config", sslTag)
// 				log.Error(tmpErr.Error() + ", err: " + err.Error())
// 				return nil, tmpErr
// 			}
// 			log.Errorf("Load %s TLS config failed: %s", scheme, err)
// 			return nil, err
// 		}
// 		log.Warnf("%s TLS mode, verify peer: %t, cipher plugin: %s.",
// 			sslTag, sslConfig.VerifyPeer, sslConfig.CipherPlugin)
// 		tlsConfig = tmpTLSConfig
// 	}
// 	return tlsConfig, nil
// }

// func getDiscoverOptions() (oSD registry.Options, err error) {
// 	hostsSD, schemeSD, err := registry.URIs2Hosts(strings.Split(config.GetServiceDiscoveryAddress(), ","))
// 	if err != nil {
// 		return
// 	}
// 	oSD.Addrs = hostsSD
// 	oSD.Tenant = config.GetServiceDiscoveryTenant()
// 	oSD.Version = config.GetServiceDiscoveryAPIVersion()
// 	oSD.ConfigPath = config.GetServiceDiscoveryConfigPath()
// 	oSD.TLSConfig, err = getTLSConfig(schemeSD, "serviceDiscovery")
// 	if err != nil {
// 		return
// 	}
// 	if oSD.TLSConfig != nil {
// 		oSD.EnableSSL = true
// 	}
// 	return
// }

var (
	opt = client.Options{
		Addrs:        []string{"127.0.0.1:30100"},
		EnableSSL:    false,
		ConfigTenant: "default",
		Timeout:      time.Duration(15),
	}
)

// type Options struct {
// 	Addrs        []string
// 	EnableSSL    bool
// 	ConfigTenant string
// 	Timeout      time.Duration
// 	TLSConfig    *tls.Config
// 	// Other options can be stored in a context
// 	Context    context.Context
// 	Compressed bool
// 	Verbose    bool
// 	Version    string
// }

// NewController creates a new servicecomb controller
func NewController(addr string, clusterID string) (*Controller, error) {
	// if err := config.Init(); err != nil {
	// 	log.Error("failed to initialize conf: " + err.Error())
	// 	return nil, err
	// }
	// // var oR, oSD, oCD registry.Options
	// oSD, err := getDiscoverOptions()
	// if err != nil {
	// 	return nil, err
	// }

	// sco := servicecenter.ToSCOptions(oSD)

	address := os.Getenv("COMB_ADDR")
	r := &client.RegistryClient{}

	if len(address) != 0 {
		opt.Addrs[0] = address
	} else if len(addr) != 0 {
		opt.Addrs[0] = addr
	}

	if err := r.Initialize(opt); err != nil {
		log.Errorf("RegistryClient initialization failed. %s", err)
		return nil, err
	}

	controller := Controller{
		client: r,
	}

	return &controller, nil
}

// Provider indicate registry type used
func (c *Controller) Provider() serviceregistry.ProviderID {
	return serviceregistry.Comb
}

// Cluster return cluster id
func (c *Controller) Cluster() string {
	return c.clusterID
}

// Services list declarations of all services in the system
func (c *Controller) Services() ([]*model.Service, error) {
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()

	servicesList := make([]*model.Service, 0, len(c.monitor.services))
	for _, value := range c.monitor.services {
		servicesList = append(servicesList, value)
	}

	return servicesList, nil
}

// GetService retrieves a service by host name if it exists
func (c *Controller) GetService(hostname host.Name) (*model.Service, error) {
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()

	// err := c.initCache()
	// if err != nil {
	// 	return nil, err
	// }
	if service, ok := c.monitor.services[string(hostname)]; ok {
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
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()

	// err := c.initCache()
	// if err != nil {
	// 	return nil, err
	// }

	if serviceInstances, ok := c.monitor.serviceInstances[string(svc.Hostname)]; ok {
		var instances []*model.ServiceInstance
		for _, instance := range serviceInstances {
			if labels.HasSubsetOf(instance.Endpoint.Labels) && portMatch(instance, port) {
				instances = append(instances, instance)
			}
		}

		return instances, nil
	}
	return nil, fmt.Errorf("could not find instance of service: %s", string(svc.Hostname))
}

// returns true if an instance's port matches with any in the provided list
func portMatch(instance *model.ServiceInstance, port int) bool {
	return port == 0 || port == instance.ServicePort.Port
}

// GetProxyServiceInstances lists service instances co-located with a given proxy
func (c *Controller) GetProxyServiceInstances(node *model.Proxy) ([]*model.ServiceInstance, error) {
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()

	// err := c.initCache()
	// if err != nil {
	// 	return nil, err
	// }

	out := make([]*model.ServiceInstance, 0)
	for _, instances := range c.monitor.serviceInstances {
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

// GetProxyWorkloadLabels lists service labels co-located with a given proxy
func (c *Controller) GetProxyWorkloadLabels(proxy *model.Proxy) (labels.Collection, error) {
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()

	// err := c.initCache()
	// if err != nil {
	// 	return nil, err
	// }

	out := make(labels.Collection, 0)
	for _, instances := range c.monitor.serviceInstances {
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
	c.monitor.svcHandlers = append(c.monitor.svcHandlers, f)
	return nil
}

// func (c *Controller) serviceHandler(serviceID string, *instance registry.MicroServiceInstance, event string) error {
// 	// convertEven
// 	return nil
// }

// AppendInstanceHandler implements a service catalog operation
func (c *Controller) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
	c.monitor.instHandlers = append(c.monitor.instHandlers, f)
	return nil
}

// func (c *Controller) instanceHandler(serviceID string, *instance registry.MicroServiceInstance, event string) error {
// 	// convertEven
// 	return nil
// }

// GetIstioServiceAccounts implements model.ServiceAccounts operation TODO
func (c *Controller) GetIstioServiceAccounts(svc *model.Service, ports []int) []string {
	return []string{
		spiffe.MustGenSpiffeURI("default", "default"),
	}
}

// func (c *Controller) initCache() error {
// 	if c.initDone {
// 		return nil
// 	}
// 	c.services = make(map[string]*model.Service)
// 	c.serviceInstances = make(map[string][]*model.ServiceInstance)
// 	c.combSvc = make(map[string][]string)
// 	c.combInst = make(map[string][]string)

// 	// get all services from servicecomb
// 	services, err := ((*servicecenter.ServiceDiscovery)(c.discover)).GetAllMicroServices()
// 	if err != nil {
// 		return err
// 	}

// 	for _, service := range services {
// 		// get endpoints of a service from consul

// 		endpoints, err := c.discover.GetMicroServiceInstances("0", service.ServiceID)
// 		if err != nil {
// 			return err
// 		}

// 		svcs := convertService(service, endpoints)
// 		for _, svc := range svcs {
// 			c.services[string(svc.Hostname)] = svc
// 			c.combSvc[service.ServiceID] = append(c.combSvc[service.ServiceID], string(svc.Hostname))
// 			instances := []*model.ServiceInstance{}
// 			// instances := make([]*model.ServiceInstance, len(endpoints))
// 			for _, endpoint := range endpoints {
// 				// for _, instance := range convertInstance(svc, endpoint) {
// 				instances = append(instances, convertInstance(svc, endpoint)...)
// 				c.combInst[service.ServiceID] = append(c.combInst[service.ServiceID], endpoint.InstanceID)
// 			}
// 			c.serviceInstances[string(svc.Hostname)] = instances
// 		}
// 	}

// 	c.initDone = true
// 	return nil
// }

// func (c *Controller) refreshCache() {
// 	c.cacheMutex.Lock()
// 	defer c.cacheMutex.Unlock()
// 	c.initDone = false
// }

// // InstanceChanged instances event callback
// func (c *Controller) InstanceChanged(instance []*registry.MicroServiceInstance, event model.Event) error {
// 	c.refreshCache()
// 	return nil
// }

// // ServiceChanged services event callback
// func (c *Controller) ServiceChanged(service []*registry.MicroService, event model.Event) error {
// 	c.refreshCache()
// 	return nil
// }
