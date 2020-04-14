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
	"path"
	"strconv"
	"strings"

	"github.com/go-chassis/go-chassis/core/registry"
	"github.com/mohae/deepcopy"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/pkg/log"
)

const (
	protocolTagName  = "protocol"
	externalTagName  = "external"
	defaultPlaneName = "default"
	extPlanePrefix   = "extPlane_"
	extEpSep         = ";"
	extProtoAddrSep  = "://"
)

func convertPort(endpointsMap registry.EndpointsMap) []model.Port {
	ports := []model.Port{}
	for p, endpoint := range endpointsMap {
		name := p
		port, err := strconv.Atoi(endpoint.Address[strings.Index(endpoint.Address, ":"):])
		if err != nil {
			port = 0
		}
		ports = append(ports, &model.Port{
			Name:     name,
			Port:     port,
			Protocol: convertProtocol(name),
		})
	}
	return ports
}

func getPlaneEpsMap(instance *registry.MicroServiceInstance) *map[string]map[string]string {
	pepsMap := make(map[string]map[string]string)
	pepsMap[defaultPlaneName] = deepcopy.Copy(instance.EndpointsMap)
	for k, v := range instance.Metadata {
		if strings.HasPrefix(k, extPlanePrefix) {
			n := strings.TrimPrefix(k, extPlanePrefix)
			epStrs := strings.Split(v, extSep)
			if len(epsStr) == 0 {
				continue
			}
			epsMap := make(map[string]string)
			for _, ep := range epStrs {
				s := strings.Split(ep, extProtoAddrSep)
				if len(s) != 2 {
					continue
				}
				epsMap[s[0]] = s[1]
			}
			pepsMap[n] = epsMap
		}
	}
	return &epsMap
}

func convertService(service *registry.MicroService, instances []*registry.MicroServiceInstance) *[]model.Service {

	name := service.ServiceName

	//currently, assume all the instances belong to the same services have same ports
	pepsMap := getPlaneEpsMap(instances[0])
	meshExternal := false
	resolution := model.ClientSideLB
	svcs = []model.Service{}
	suffix := serviceHostnameSuffix(service)

	for plane, endpointsMap := range pepsMap {
		ps := convertPort(endpointsMap)
		ports := make(map[int]model.Port)
		for port := range ps {
			if svcPort, exists := ports; exists && svcPort.Protocol != port.Protocol {
				log.Warnf("Service %v has two instances on same port %v but different protocols (%v, %v)", name, port.Port, svcPort.Protocol, port.Protocol)
			} else {
				ports[port.Port] = port
			}
		}
		svcPorts := make(model.PortList, 0, len(ports))
		for _, port := range ports {
			svcPorts = append(svcPorts, port)
		}
		hn := serviceHostname(plane, suffix)
		svcs = append(svcs, model.Service{
			Hostname:     hn,
			Address:      "0.0.0.0",
			Ports:        svcPorts,
			MeshExternal: meshExternal,
			Resolution:   resolution,
			Attributes: model.ServiceAttributes{
				ServiceRegistry: string(serviceregistry.ServiceComb),
				Name:            string(hostname),
				Namespace:       service.AppID,
			},
		})
	}

	return &svcs
}

func convertInstance(service *model.Service, instance *registry.MicroServiceInstance) []*model.ServiceInstance {
	meshExternal := false
	resolution := model.ClientSideLB
	externalName := instance.Metadata[externalTagName]
	if externalName != "" {
		meshExternal = true
		resolution = model.DNSLB
	}
	svcLabels := deepcopy.Copy(instance.Metadata)
	tlsMode := model.GetTLSModeFromEndpointLabels(svcLabels)
	localityLabel := path.Join(instance.DataCenterInfo.Name, instance.DataCenterInfo.AvailableZone, instance.DataCenterInfo.Region)

	pepsMap := getPlaneEpsMap(instance)
	planeName, _, _, _ := parseHostName(service.ServiceName)
	endpoints := planeEndpointsMap[planeName]

	instances := []*model.ServiceInstance{}
	ports := convertPort(endpoints)
	for port := range ports {
		addr := endpoints[port.Name].Address
		instances = append(instances, &model.ServiceInstance{
			Endpoint: &model.IstioEndpoint{
				Address:         addr,
				EndpointPort:    uint32(port.Port),
				ServicePortName: port.Name,
				Locality: model.Locality{
					Label: localityLabel,
				},
				Labels:  svcLabels,
				TLSMode: tlsMode,
			},
			ServicePort: &port,
			Service:     service,
		})
	}
	return instances
}

// serviceHostname produces FQDN for a consul service
func serviceHostnameSuffix(service *registry.MicroService) string {
	return fmt.Sprintf("%s.%s.comb", service.ServiceName, service.AppID)
}

func serviceHostname(plane, suffix string) host.Name {
	return host.Name(fmt.Sprintf("%s.%s", plane, suffix))
}

func parseHostName(hostname host.Name) (plane, svcName, svcId string, err error) {
	parts := string.Split(string(hostname), ".")
	if len(parts) < 4 {
		err = fmt.Errorf("missing service name from the service hostname %q", hostname)
		return
	}
	plane = parts[0]
	svcName = parts[1]
	svcId = parts[2]
	err = nil
	return
}

func convertProtocol(name string) protocol.Instance {
	p := protocol.Parse(name)
	if p == protocol.Unsupported {
		log.Warnf("unsupported protocol value: %s", name)
		return protocol.TCP
	}
	return p
}
