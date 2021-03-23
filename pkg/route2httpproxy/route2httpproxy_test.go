package route2httpproxy

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	route "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
)

func TestDeployment(t *testing.T) {
	type output struct {
		kind             string
		apiVersion       string
		fqdn             string
		secretKind       string
		secretApiVersion string
	}

	tests := []struct {
		name         string
		routeInput   string
		serviceInput string
		want         output
		domain       string
	}{
		{
			"Test route with tls",
			"route_with_tls.json",
			"service-input.json",
			output{
				kind:             "HTTPProxy",
				apiVersion:       "projectcontour.io/v1",
				fqdn:             "nginx-example.migrator.servicemesh.biz",
				secretKind:       "Secret",
				secretApiVersion: "v1",
			},
			"*.migrator.servicemesh.biz",
		},
		{
			"Test route without tls",
			"route_without_tls.json",
			"service-input.json",
			output{
				kind:       "HTTPProxy",
				apiVersion: "projectcontour.io/v1",
				fqdn:       "nginx-example.migrator.servicemesh.biz",
			},
			"*.migrator.servicemesh.biz",
		},
	}

	for _, tc := range tests {
		routeInput, serviceInput := newMutatorFromFileData(t, tc.routeInput, tc.serviceInput, tc.name)
		hp, secret, _ := Mutate(tc.name, logrus.New(), routeInput, serviceInput, tc.domain)

		assert.Equal(t, tc.want.apiVersion, tc.want.apiVersion)
		assert.Equal(t, routeInput.Name, hp.Name)

		if routeInput.Spec.Path != "" {
			assert.Equal(t, routeInput.Spec.Path, hp.Spec.Routes[0].Conditions[0].Prefix)
		}

		assert.Equal(t, routeInput.Spec.To.Name, hp.Spec.Routes[0].Services[0].Name)
		assert.Equal(t, routeInput.Spec.Port.TargetPort.IntValue(), hp.Spec.Routes[0].Services[0].Port)
		assert.Equal(t, routeInput.Spec.To.Name, hp.Name)
		assert.Equal(t, tc.want.fqdn, hp.Spec.VirtualHost.Fqdn)

		if routeInput.Spec.TLS != nil {
			if routeInput.Spec.TLS.Termination == "passthrough" {
				assert.True(t, hp.Spec.VirtualHost.TLS.Passthrough)
			}
			if routeInput.Spec.TLS.Termination == "edge" || routeInput.Spec.TLS.Termination == "reencrypt" {
				if routeInput.Spec.TLS.Certificate != "" && routeInput.Spec.TLS.Key != "" {
					assert.Equal(t, hp.Spec.VirtualHost.TLS.SecretName, secret.Name)
					assert.Equal(t, tc.want.secretKind, secret.Kind)
					assert.Equal(t, tc.want.secretApiVersion, secret.APIVersion)
					assert.Equal(t, routeInput.ObjectMeta.Namespace, secret.Namespace)
					assert.Equal(t,
						base64.StdEncoding.EncodeToString([]byte(routeInput.Spec.TLS.Key)),
						string(secret.Data["tls.key"]))
					assert.Equal(t,
						base64.StdEncoding.EncodeToString([]byte(routeInput.Spec.TLS.Certificate)),
						string(secret.Data["tls.crt"]))
				}
			}
		}

	}
}

func newMutatorFromFileData(t *testing.T, routeFile, serviceFile, testName string) (route.Route, core.Service) {
	routeConfigFilePath := filepath.Join("testdata", routeFile)
	route2File, err := ioutil.ReadFile(routeConfigFilePath)
	if err != nil {
		t.Fatalf("%s: %v", testName, err)
	}

	route := route.Route{}
	err = json.Unmarshal(route2File, &route)
	if err != nil {
		t.Errorf("%s: unmarshall Route JSON = %v", testName, err)
	}

	serviceConfigFilePath := filepath.Join("testdata", serviceFile)
	service2File, err := ioutil.ReadFile(serviceConfigFilePath)
	if err != nil {
		t.Fatalf("%s: %v", testName, err)
	}

	service := core.Service{}
	err = json.Unmarshal(service2File, &service)
	if err != nil {
		t.Errorf("%s: unmarshall Service JSON = %v", testName, err)
	}

	return route, service
}
