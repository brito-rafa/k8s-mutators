package route2httpproxy

import (
	"encoding/base64"
	"errors"

	routev1API "github.com/openshift/api/route/v1"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"

	"strings"

	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// declare a list of OCP routes Spec fields that are not supported in httpproxy
var (
	ocpRouteWildCardPolicy                = "Route.Spec.WildCardPolicy"
	ocpRouteWeight                        = "Route.Spec.Weight"
	ocpRouteAlternateBackends             = "Route.Spec.AlternateBackends"
	ocpRouteInsecureEdgeTerminationPolicy = "Route.Spec.InsecureEdgeTerminationPolicy"
	ocpRouteDestinationCACertificate      = "Route.Spec.DestinationCACertificate"
	ocpRouteCACertificate                 = "Route.Spec.CACertificate"
)

// if there's an error, print it out
func checkError(pluginName string, log logrus.FieldLogger, msgString string, err error) {
	if err != nil {
		log.Errorf("[%s] %s %v", pluginName, msgString, err.Error())
	}
}

func unsupportedField(pluginName string, log logrus.FieldLogger, msgString string) {
	// maybe only print out if log level is debug or ...
	log.Warnf("[%s] %s %s", pluginName, msgString, "unsupported")
}

// append the list of unsupported fields
func annotateUnsupported(pluginName string, src routev1API.Route) map[string]string {

	annotations := src.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[pluginName+"/"+ocpRouteWildCardPolicy] = "unsupported"
	annotations[pluginName+"/"+ocpRouteWeight] = "unsupported"
	annotations[pluginName+"/"+ocpRouteAlternateBackends] = "unsupported"
	annotations[pluginName+"/"+ocpRouteInsecureEdgeTerminationPolicy] = "unsupported"
	annotations[pluginName+"/"+ocpRouteDestinationCACertificate] = "unsupported"
	annotations[pluginName+"/"+ocpRouteCACertificate] = "unsupported"

	return annotations
}

// translateRoute will return a route structure element for the HTTPProxy.Spec.Routes based on OCP RouteTargetRef
// The main RouteTargetRef is Route.Spec.To, the alternate routes are under Route.Spec.AlternateBackends
// One service object is required per RouteTargetRef
// Today, this function is written only to return a single element, which is the translation of Route.Spec.To
// TO DO : Make this function to return an array of httpproxy routes, including to parse Route.Spec.AlternateBackends
// TO DO : Handle weight to be extracted from OCP Route into HTTPProxy Route
// TO DO : Handle OCP InsecureEdgeTerminationPolicy Allow as permitInsecure
func translateRoute(pluginName string, log logrus.FieldLogger, ocpRoute routev1API.Route, service core.Service) (*contourv1.Route, error) {

	log.Debugf("[%s] Details of the Service are: %v\n", pluginName, service)

	if service.Name != ocpRoute.Spec.To.Name {
		log.Errorf("[%s] The service name is %s and the OCP route reference Spec.To.Name is %s. They need to match.", pluginName, service.Name, ocpRoute.Spec.To.Name)
		myErr := errors.New("the service name and the target reference do not match")
		return nil, myErr
	}

	// OCP Route only support "Service" as of this writing
	// this condition is only for future sanity check
	if ocpRoute.Spec.To.Kind != "Service" {
		log.Errorf("[%s] The OCP target reference kind is %s. This plugin only supports Service.", pluginName, ocpRoute.Spec.To.Kind)
		myErr := errors.New("the OCP target reference kind is not Service")
		return nil, myErr
	}

	// TODO: handle weights
	// https://projectcontour.io/docs/main/config/request-routing/#upstream-weighting
	// it must go along with AlternateBackends
	if ocpRoute.Spec.To.Weight != nil {
		log.Debugf("[%s] OCP Route %s has weights defined. This mutation does not support weights and they will be ignored.", pluginName, ocpRoute.Name)

	}

	// Route.Spec.Port: If specified, it is the port used by the OCP router.
	// This is the value we want on httpproxy port
	// but we have to look it up on the service object
	// Most routers will use all endpoints exposed by the service by default

	// The service can support multiple ports, it is an array.
	serviceNumberPorts := len(service.Spec.Ports)

	var matchedPort int32

	if ocpRoute.Spec.Port != nil {
		log.Debugf("[%s] OCP Route Spec.Port defined and TargetPort is %v", pluginName, ocpRoute.Spec.Port.TargetPort)

		// Spec.Port.TargetPort can be a string or an integer
		// if string, we need to lookup the service.Ports[].Name (string)
		// if int, we need to lookup the service.Ports[].TargetPort (int)
		for i := 0; i < serviceNumberPorts; i++ {
			if ocpRoute.Spec.Port.TargetPort == service.Spec.Ports[i].TargetPort {
				log.Debugf("[%s] OCP Route Spec.Port.TargetPort %v is an integer and found on service %v", pluginName, ocpRoute.Spec.Port.TargetPort, service.Spec.Ports[i].TargetPort)
				matchedPort = service.Spec.Ports[i].Port
				break
			}
			if ocpRoute.Spec.Port.TargetPort.StrVal == service.Spec.Ports[i].Name {
				log.Debugf("[%s] OCP Route Spec.Port.TargetPort is a string with value %s and it matched on service name %s", pluginName, ocpRoute.Spec.Port.TargetPort.StrVal, service.Spec.Ports[i].Name)
				matchedPort = service.Spec.Ports[i].Port
				break
			}
		}
		if matchedPort == 0 {
			log.Errorf("[%s] translateRoute cannot match Route.Spec.Port %#v with Service object %#v", pluginName, ocpRoute.Spec.Port, service.Spec.Ports)
			myErr := errors.New("translateRoute cannot match Route.Spec.Port with Service object")
			return nil, myErr
		}
	} else {
		// Route.Spec.Port is not defined, picking the first Port from Service object
		matchedPort = service.Spec.Ports[0].Port
	}
	log.Debugf("[%s] This is the matched port to be used: %v\n", pluginName, matchedPort)

	// The httpproxy service name
	httpproxySvc := contourv1.Service{
		Name: service.Name,
	}

	// the port of httpproxy service must be in integer
	httpproxySvc.Port = int(matchedPort)

	httpproxyRoute := contourv1.Route{}
	if ocpRoute.Spec.Path != "" {
		httpproxyRoute.Conditions[0].Prefix = ocpRoute.Spec.Path
	}

	httpproxyRoute.Services = append(httpproxyRoute.Services, httpproxySvc)
	log.Debugf("[%s] httpproxy Services array %#v", pluginName, httpproxyRoute.Services)

	return &httpproxyRoute, nil
}

func createSecret(pluginName string, log logrus.FieldLogger, ocpRoute routev1API.Route) (*core.Secret, error) {
	hpSecretNamePrefix := "hpsecret-" //defining a secret-prefix
	hpSecretName := ocpRoute.ObjectMeta.Name
	hpSecretName = hpSecretNamePrefix + hpSecretName

	ocpRouteKey := ocpRoute.Spec.TLS.Key
	ocpRouteCrt := ocpRoute.Spec.TLS.Certificate
	hpSecretKey := base64.StdEncoding.EncodeToString([]byte(ocpRouteKey))
	hpSecretCrt := base64.StdEncoding.EncodeToString([]byte(ocpRouteCrt))
	hpSecret := core.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      hpSecretName,
			Namespace: ocpRoute.ObjectMeta.Namespace,
		},
		Data: map[string][]byte{
			"tls.key": []byte(hpSecretKey),
			"tls.crt": []byte(hpSecretCrt),
		},
	}
	log.Debugf("[%s] Created a Secret to be used for the HTTPProxy %#v", pluginName, hpSecret)

	// To DO: create error conditions, for now, passing nil
	return &hpSecret, nil

}

// Mutate converts an OpenShift (OCP) Route to Contour HTTProxy
// If OCP route has a certificate, returns it as a secret
// TO DO: Support OCP Route.Spec.AlternateBackends
// TO DO : Handle OCP InsecureEdgeTerminationPolicy Allow as permitInsecure
func Mutate(pluginName string, log logrus.FieldLogger, ocpRoute routev1API.Route, service core.Service, domain string) (*contourv1.HTTPProxy, *core.Secret, error) {

	log.Debugf("[%s] ocpRoute %#v", pluginName, ocpRoute)

	if service.Namespace != ocpRoute.Namespace {
		log.Errorf("[%s] The namespace of service is %s and the namespace of OCP Route is %s. They need to match.", pluginName, service.Namespace, ocpRoute.Namespace)
		return nil, nil, errors.New("namespace of service and namespace of OCP Route do not match")
	}

	if ocpRoute.Spec.AlternateBackends != nil {
		log.Warnf("[%s] OCP Route %s has AlternateBackends defined. This mutation does not support them and they will be ignored.", pluginName, ocpRoute.Name)
	}

	// Check if the OCP Route allows HTTP
	// TODO: Decide the mutation.
	if ocpRoute.Spec.TLS != nil {
		if (ocpRoute.Spec.TLS.InsecureEdgeTerminationPolicy != "") && (ocpRoute.Spec.TLS.InsecureEdgeTerminationPolicy == "Allow") {
			log.Warnf("[%s] OCP Route %s InsecureEdgeTerminationPolicy is set as Allow. This mutation does not support it and it will default to HTTPS redirect.", pluginName, ocpRoute.Name)
		}

		if ocpRoute.Spec.TLS.DestinationCACertificate != "" {
			log.Warnf("[%s] OCP Route %s has DestinationCACertificate set. This mutation does not support it and it will ignore it.", pluginName, ocpRoute.Name)
		}
	}

	hp := contourv1.HTTPProxy{}

	// populate metadata with properties from ocpRoute object
	hp.Kind = "HTTPProxy"
	hp.APIVersion = "projectcontour.io/v1"
	hp.Name = ocpRoute.Name
	hp.Namespace = ocpRoute.Namespace
	hp.Annotations = annotateUnsupported(pluginName, ocpRoute)
	hp.Labels = ocpRoute.Labels

	// Start building the httpproxy Spec

	// We need to convert the RouteTargetRef from OCP Route in the format of httpproxy route
	hpTranslatedRoute, err := translateRoute(pluginName, log, ocpRoute, service)
	if err != nil {
		log.Errorf("[%s] Error in parsing the OCP Route and Service.", pluginName)
		return nil, nil, err
	}

	hp.Spec.Routes = append(hp.Spec.Routes, *hpTranslatedRoute)
	log.Debugf("[%s] httpproxy translated routes: %#v", pluginName, hp.Spec.Routes)

	// Handling the wildcard DNS domain
	var httpproxyFqdn string
	if domain != "" {
		normalizedDomain := domain
		// let's accept the domain starting with "*." or "."
		if domain[0:2] == "*." {
			normalizedDomain = domain[2:]
		} else if domain[0:1] == "." {
			normalizedDomain = domain[1:]
		}
		ocpRouteSplit := strings.SplitN(ocpRoute.Spec.Host, ".", 2)
		httpproxyFqdn = ocpRouteSplit[0] + "." + normalizedDomain
	} else {
		// user did not specify the new wild card DNS
		httpproxyFqdn = ocpRoute.Spec.Host
		log.Warnf("[%s] No new wildcard DNS domain specified. This mutation will use original domain from OCP route %s.", pluginName, httpproxyFqdn)

	}

	//extract the Prefix of OCP route
	log.Debugf("[%s] FQDN of the httpproxy will be set to:] %s", pluginName, httpproxyFqdn)

	hp.Spec.VirtualHost = &contourv1.VirtualHost{
		Fqdn: httpproxyFqdn,
	}

	// Handling TLS
	if ocpRoute.Spec.TLS != nil {

		log.Debugf("[%s] OCP route TLS is set and termination is %s", pluginName, ocpRoute.Spec.TLS.Termination)

		if ocpRoute.Spec.TLS.Termination == "passthrough" {
			hp.Spec.VirtualHost.TLS = &contourv1.TLS{
				Passthrough: true,
			}
		}

		if ocpRoute.Spec.TLS.Termination == "edge" || ocpRoute.Spec.TLS.Termination == "reencrypt" {
			if ocpRoute.Spec.TLS.Certificate != "" && ocpRoute.Spec.TLS.Key != "" {
				log.Debugf("[%s] OCP route has certs and keys, generating secret.", pluginName)
				hpSecret, err := createSecret(pluginName, log, ocpRoute)
				if err != nil {
					log.Errorf("[%s] Error in creating the secret.", pluginName)
					return nil, nil, err
				}
				hp.Spec.VirtualHost.TLS = &contourv1.TLS{
					SecretName: hpSecret.Name,
				}

				return &hp, hpSecret, nil
			}
		}
	}
	return &hp, nil, nil
}
