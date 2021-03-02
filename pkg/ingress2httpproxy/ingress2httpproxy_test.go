package ingress2httpproxy

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	networking "k8s.io/api/networking/v1beta1"
)

const clientName = "testClient"

func TestBuildHTTPProxy(t *testing.T) {
	type output struct {
		httpProxy  string
		apiVersion string
	}

	tests := []struct {
		name   string
		input  string
		want   output
		domain string
	}{
		{
			"Test config map with *.domain",
			"examplewithwildcard.json",
			output{
				httpProxy:  "HTTPProxy",
				apiVersion: "projectcontour.io/v1",
			},
			"*.migrator.servicemesh.biz",
		},
		{
			"Test config map with .domain",
			"examplewithwildcard.json",
			output{
				httpProxy:  "HTTPProxy",
				apiVersion: "projectcontour.io/v1",
			},
			".migrator.servicemesh.biz",
		},
		{
			"Test config map with .domain",
			"examplewithoutwildcard.json",
			output{
				httpProxy:  "HTTPProxy",
				apiVersion: "projectcontour.io/v1",
			},
			"cafe.migrator.servicemesh.biz",
		},
	}

	for _, tc := range tests {
		m := newMutatorFromFileData(t, tc.input, tc.domain)
		hp := m.buildHTTPProxy()

		assert.Equal(t, tc.want.httpProxy, hp.Kind)
		assert.Equal(t, tc.want.apiVersion, hp.APIVersion)
		assert.Equal(t, m.input.Name, hp.Name)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[0].Path, hp.Spec.Routes[0].Conditions[0].Prefix)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName, hp.Spec.Routes[0].Services[0].Name)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue(), hp.Spec.Routes[0].Services[0].Port)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[1].Path, hp.Spec.Routes[1].Conditions[0].Prefix)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[1].Backend.ServiceName, hp.Spec.Routes[1].Services[0].Name)
		assert.Equal(t, m.input.Spec.Rules[0].HTTP.Paths[1].Backend.ServicePort.IntValue(), hp.Spec.Routes[1].Services[0].Port)
		assert.Equal(t, m.input.Spec.Rules[0].Host, hp.Spec.VirtualHost.Fqdn)

	}
}

func newMutatorFromFileData(t *testing.T, fileName string, domain string) Mutator {
	ingressFilePath := filepath.Join("testdata", fileName)
	ingressFile, err := ioutil.ReadFile(ingressFilePath)

	if err != nil {
		t.Fatal(err)
	}

	ingress := networking.Ingress{}
	err = json.Unmarshal(ingressFile, &ingress)

	if err != nil {
		t.Errorf("Failed to unmarshall Ingress JSON = %v", err)
	}

	return NewMutator(clientName, logrus.New(), ingress, domain)
}
