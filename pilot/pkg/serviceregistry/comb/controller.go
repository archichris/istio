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
var (
	defaultRegisterAddr = "127.0.0.1:30100"
	opt                 = client.Options{
		EnableSSL:    false,
		ConfigTenant: "default",
		Timeout:      time.Duration(15),
		Version:      "v3",
	}
)

// Controller communicates with Consul and monitors for changes
type Controller struct {
	monitor   *combMonitor
	clusterID string
}

// NewController creates a new servicecomb controller
func NewController(addr string, clusterID string) (*Controller, error) {

	address := addr

	if len(address) == 0 {
		if len(os.Getenv("COMB_ADDR")) != 0 {
			address = os.Getenv("COMB_ADDR")
		} else {
			address = defaultRegisterAddr
		}
	}
	opt.Addrs = []string{address}

	log.Infof("[Comb] ServiceCenter address: %v", opt.Addrs)

	monitor, err := NewCombMonitor(&opt)
	if err != nil {
		return nil, err
	}

	controller := Controller{
		monitor:   monitor,
		clusterID: clusterID,
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
	return c.monitor.Services()
}

// GetService retrieves a service by host name if it exists
func (c *Controller) GetService(hostname host.Name) (*model.Service, error) {
	c.monitor.cacheMutex.Lock()
	defer c.monitor.cacheMutex.Unlock()
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

// AppendInstanceHandler implements a service catalog operation
func (c *Controller) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
	c.monitor.instHandlers = append(c.monitor.instHandlers, f)
	return nil
}

// GetIstioServiceAccounts implements model.ServiceAccounts operation TODO
func (c *Controller) GetIstioServiceAccounts(svc *model.Service, ports []int) []string {
	return []string{
		spiffe.MustGenSpiffeURI("default", "default"),
	}
}
