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
	"github.com/go-chassis/go-chassis/pkg/scclient/proto"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/pkg/log"
)

const (
	// protocolTagName  = "protocol"
	// externalTagName  = "external"
	defaultPlaneName = "default"
	extPlanePrefix   = "extPlane_"
	extEpSep         = ";"
	// extProtoAddrSep  = "://"
)

func parseEndpoint(endpoint string) (addr, port string, ssl bool) {
	parts := strings.Split(endpoint, ":")
	addr = parts[0]
	if strings.Contains(parts[1], "sslEnabled=true") {
		ssl = true
	} else {
		ssl = false
	}
	parts = strings.Split(parts[1], "?")
	port = parts[0]
	return
}

func convertPort(endpointsMap map[string]string) []model.Port {
	ports := []model.Port{}
	for p, endpoint := range endpointsMap {
		name := p
		_, portStr, _ := parseEndpoint(endpoint)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			port = 0
		}
		ports = append(ports, model.Port{
			Name:     name,
			Port:     port,
			Protocol: convertProtocol(name),
		})
	}
	return ports
}

func getPlaneEpsMap(instance *proto.MicroServiceInstance) map[string]map[string]string {
	pepsMap := make(map[string]map[string]string)
	m, _ := registry.GetProtocolMap(instance.Endpoints)
	pepsMap[defaultPlaneName] = m
	for k, v := range instance.Properties {
		if strings.HasPrefix(k, extPlanePrefix) {
			n := strings.TrimPrefix(k, extPlanePrefix)
			eps := strings.Split(v, extEpSep)
			if len(eps) == 0 {
				continue
			}
			m, _ = registry.GetProtocolMap(eps)
			pepsMap[n] = m
		}
	}
	return pepsMap
}

func convertService(service *proto.MicroService, instances []*proto.MicroServiceInstance) []*model.Service {

	name := service.ServiceName

	//currently, assume all the instances belong to the same services have same ports
	pepsMap := getPlaneEpsMap(instances[0])
	meshExternal := false
	resolution := model.ClientSideLB
	svcs := []*model.Service{}
	suffix := serviceHostnameSuffix(service)

	for plane, endpointsMap := range pepsMap {
		ps := convertPort(endpointsMap)
		ports := make(map[int]model.Port)
		for _, port := range ps {
			if svcPort, exists := ports[port.Port]; exists && svcPort.Protocol != port.Protocol {
				log.Warnf("Service %v has two instances on same port %v but different protocols (%v, %v)", name, port.Port, svcPort.Protocol, port.Protocol)
			} else {
				ports[port.Port] = port
			}
		}
		svcPorts := make(model.PortList, 0, len(ports))
		for _, port := range ports {
			svcPorts = append(svcPorts, &port)
		}
		hn := serviceHostname(plane, suffix)
		svcs = append(svcs, &model.Service{
			Hostname:     hn,
			Address:      "0.0.0.0",
			Ports:        svcPorts,
			MeshExternal: meshExternal,
			Resolution:   resolution,
			Attributes: model.ServiceAttributes{
				ServiceRegistry: string(serviceregistry.Comb),
				Name:            string(hn),
				Namespace:       "default",
			},
		})
	}

	return svcs
}

// type dataCenterInfo struct {
// 	Name          string `json:"name"`
// 	Region        string `json:"region"`
// 	AvailableZone string `json:"az"`
// }

func convertInstance(service *model.Service, combInstance *proto.MicroServiceInstance) []*model.ServiceInstance {

	svcLabels := map[string]string{}
	for k, v := range combInstance.Properties {
		svcLabels[k] = v
	}
	svcLabels["instanceid"] = combInstance.InstanceId
	svcLabels["serviceid"] = combInstance.ServiceId
	tlsMode := model.GetTLSModeFromEndpointLabels(svcLabels)
	localityLabel := ""
	if combInstance.DataCenterInfo != nil {
		localityLabel = path.Join(combInstance.DataCenterInfo.Name, combInstance.DataCenterInfo.AvailableZone, combInstance.DataCenterInfo.Region)
	}

	pepsMap := getPlaneEpsMap(combInstance)
	planeName, _, _, _ := parseHostName(service.Hostname)
	endpoints := pepsMap[planeName]
	instances := []*model.ServiceInstance{}
	for p, endpoint := range endpoints {
		addr, portStr, _ := parseEndpoint(endpoint)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			port = 0
		}
		instances = append(instances, &model.ServiceInstance{
			Endpoint: &model.IstioEndpoint{
				Address:         addr,
				EndpointPort:    uint32(port),
				ServicePortName: p,
				Locality:        localityLabel,
				Labels:          svcLabels,
				TLSMode:         tlsMode,
			},
			ServicePort: &model.Port{
				Name:     p,
				Port:     port,
				Protocol: convertProtocol(p),
			},
			Service: service,
		})
	}
	return instances
}

// serviceHostname produces FQDN for a consul service
func serviceHostnameSuffix(service *proto.MicroService) string {
	return fmt.Sprintf("%s.%s.__v%s", service.ServiceName, service.AppId, strings.ReplaceAll(service.Version, ".", "_"))
}

func serviceHostname(plane, suffix string) host.Name {
	return host.Name(fmt.Sprintf("%s.%s", plane, suffix))
}

func parseHostName(hostname host.Name) (plane, svcName, appID string, err error) {
	parts := strings.Split(string(hostname), ".")
	if len(parts) < 4 {
		err = fmt.Errorf("missing service name from the service hostname %q", hostname)
		return
	}
	plane = parts[0]
	svcName = parts[1]
	appID = parts[2]
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

// func convertEvent(event string) model.Event {
// 	switch event {
// 	case client.EventCreate:
// 		return model.EventAdd
// 	case client.EventUpdate:
// 		return model.EventUpdate
// 	case client.EventDelete:
// 		return model.EventDelete
// 	}
// 	return model.EventUpdate
// }
