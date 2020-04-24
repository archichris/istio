// Copyright 2020 Istio Authors
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

package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {

	pilotArgs := PilotArgs{
		Service: ServiceArgs{
			Registries: []string{"ServiceComb"},
		},
		Config: ConfigArgs{DisableInstallCRDs: true},
		DiscoveryOptions: DiscoveryServiceOptions{
			MonitoringAddr: ":15014",
			// SecureGrpcAddr: ":15011",
		},
	}
	os.Setenv("COMB_ADDR", "192.168.56.31:30543")
	os.Setenv("JWT_POLICY", "first-party-jwt")
	os.Setenv("JWT_POPILOT_CERT_PROVIDERLICY", "istiod")
	os.Setenv("POD_NAME", "testpod")
	os.Setenv("POD_NAMESPACE", "default")
	os.Setenv("SERVICE_ACCOUNT", "istiod")
	os.Setenv("PILOT_TRACE_SAMPLING", "1")
	os.Setenv("CONFIG_NAMESPACE", "istio-config")
	os.Setenv("CONFIG_NAMESPACE", "default")
	os.Setenv("PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_OUTBOUND", "true")
	os.Setenv("PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_INBOUND", "false")
	os.Setenv("INJECTION_WEBHOOK_CONFIG_NAME", "istio-sidecar-injector")
	// os.Setenv("ISTIOD_ADDR", "istiod.istio-system.svc:15012")
	os.Setenv("SERVICE_ACCOUNT", "istiod-service-account")
	os.Setenv("ISTIOD_SERVICE_PORT_HTTPS_DNS", "15012")
	os.Setenv("ENABLE_INJECTOR", "false")

	dnsCertDir = strings.TrimPrefix(dnsCertDir, ".")
	dnsCertFile = strings.TrimPrefix(dnsCertFile, ".")
	dnsKeyFile = strings.TrimPrefix(dnsKeyFile, ".")

	os.Setenv("ROOT_CA_DIR", dnsCertDir)

	s, err := NewServer(&pilotArgs)
	if err != nil {
		t.Errorf("create new server failed, %v", err)
		return
	}

	stop := make(chan struct{})

	if err := s.Start(stop); err != nil {
		t.Errorf("failed to start discovery service: %v", err)
		return
	}

	close(stop)

}
