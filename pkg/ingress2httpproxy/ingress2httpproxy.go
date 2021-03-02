package ingress2httpproxy

import (
	"strings"

	contour "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	networking "k8s.io/api/networking/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	unsupportedHosts = "unsupported-hosts"
)

// MutatorOutput contains the mutated output structures
type MutatorOutput struct {
	HTTPProxy contour.HTTPProxy
}

// Mutator contains common atttributes and the mutation input source structure
type Mutator struct {
	name   string
	log    logrus.FieldLogger
	input  networking.Ingress
	domain string
}

// NewMutator creates a new Mutator. Clients of this API should set a meaningful name that can be used
// to easily identify the calling client.
func NewMutator(name string, log logrus.FieldLogger, ingress networking.Ingress, domain string) Mutator {
	return Mutator{
		name:   name,
		log:    log,
		input:  ingress,
		domain: domain,
	}
}

// Mutate converts a Ingress into HTTPProxy
func (m *Mutator) Mutate() *MutatorOutput {
	return &MutatorOutput{
		HTTPProxy: m.buildHTTPProxy(),
	}
}

// buildHTTPProxy takes ingress object as an input and returns  Contour HTTPProxy
func (m *Mutator) buildHTTPProxy() contour.HTTPProxy {
	var httpAnnotations = make(map[string]string)
	hpTranslatedRoute, httpAnnotations := m.createRoute(httpAnnotations)
	hp := contour.HTTPProxy{
		TypeMeta: meta.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:        m.input.ObjectMeta.Name,
			Annotations: httpAnnotations,
			Namespace:   m.input.ObjectMeta.Namespace,
		},
		Spec: contour.HTTPProxySpec{
			Routes: hpTranslatedRoute,
		},
	}
	log.Warnf("[%s] No new wildcard DNS domain specified. This mutation will use original domain from Ingress Host %s.", m.name, m.input.Spec.Rules[0].Host)
	httpProxyFqdn := m.input.Spec.Rules[0].Host
	if m.domain != "" {
		normalizedDomain := m.domain
		// let's accept the domain starting with "*." or "."
		if m.domain[0:2] == "*." {
			normalizedDomain = m.domain[2:]
		}
		if m.domain[0:1] == "." {
			normalizedDomain = m.domain[1:]
		}
		prefix := strings.SplitN(m.input.Spec.Rules[0].Host, ".", 2)
		httpProxyFqdn = prefix[0] + "." + normalizedDomain

	}

	hp.Spec.VirtualHost = &contour.VirtualHost{
		Fqdn: httpProxyFqdn,
	}

	hp.Spec.VirtualHost.TLS = &contour.TLS{}
	hp.Spec.VirtualHost.TLS.SecretName = m.input.Spec.TLS[0].SecretName

	return hp
}

// createRoute creates the route object which includes conditions and service details
func (m *Mutator) createRoute(httpAnnotations map[string]string) ([]contour.Route, map[string]string) {
	inrules := m.input.Spec.Rules
	routes := make([]contour.Route, len(inrules[0].HTTP.Paths))
	for i, path := range inrules[0].HTTP.Paths {
		routes[i].Conditions = []contour.MatchCondition{
			{Prefix: path.Path},
		}
		routes[i].Services = []contour.Service{
			{
				Name: path.Backend.ServiceName,
				Port: path.Backend.ServicePort.IntValue(),
			},
		}
	}

	// Check if multiple rules present in Ingress object
	if len(inrules) > 1 {
		hosts := make([]string, 0, len(inrules)-1)
		for _, inrule := range inrules[1:] {
			hosts = append(hosts, inrule.Host)
		}
		log.Infof("[%s] unsupported hosts: %s", m.name, strings.Join(hosts, ", "))
		httpAnnotations[m.name+"/"+unsupportedHosts] = strings.Join(hosts, ", ")
	}

	return routes, httpAnnotations
}
