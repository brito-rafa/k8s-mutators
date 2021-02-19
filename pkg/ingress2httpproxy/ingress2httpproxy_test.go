package ingress2httpproxy

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	core "k8s.io/api/networking/v1beta1"
)

const clientName = "testClient"

func TestBuildHP(t *testing.T) {
	m := newMutatorFromFileData(t, "example.json")
	ingress := m.input
	hp := m.buildHTTPProxy()

	assert.Equal(t, "HTTPProxy", hp.Kind)
	assert.Equal(t, ingress.Name, hp.Name)
	assert.Equal(t, "projectcontour.io/v1", hp.APIVersion)
	assert.Equal(t, ingress.Spec.TLS[0].SecretName, hp.Spec.VirtualHost.TLS.SecretName)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[0].Path, hp.Spec.Routes[0].Conditions[0].Prefix)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServiceName, hp.Spec.Routes[0].Services[0].Name)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.ServicePort.IntValue(), hp.Spec.Routes[0].Services[0].Port)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[1].Path, hp.Spec.Routes[1].Conditions[0].Prefix)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[1].Backend.ServiceName, hp.Spec.Routes[1].Services[0].Name)
	assert.Equal(t, ingress.Spec.Rules[0].HTTP.Paths[1].Backend.ServicePort.IntValue(), hp.Spec.Routes[1].Services[0].Port)
	//To-do
	//assert.Equal(t, ingress.Spec.TLS[0].Hosts[0], hp.Spec.VirtualHost.Fqdn)

}

func newMutatorFromFileData(t *testing.T, fileName string) Mutator {
	ingressFilePath := filepath.Join("testdata", fileName)
	ingressFile, err := ioutil.ReadFile(ingressFilePath)

	if err != nil {
		t.Fatal(err)
	}

	ingress := core.Ingress{}
	err = json.Unmarshal(ingressFile, &ingress)

	if err != nil {
		t.Errorf("Failed to unmarshall Ingress JSON = %v", err)
	}

	return NewMutator(clientName, logrus.New(), ingress, ingress.Spec.Rules[0].Host)
}
