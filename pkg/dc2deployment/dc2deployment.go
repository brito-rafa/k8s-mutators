package dc2deployment

import (
	dcAPI "github.com/openshift/api/apps/v1"
	"github.com/sirupsen/logrus"
	deployAPI "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// declare a list of dc fields that are not supported in deployment.apps
var (
	dcTriggers = "DeploymentConfig.Spec.Triggers"
	dcTest     = "DeploymentConfig.Spec.Test"
)

// if there's an error, print it out
func checkError(pluginName string, log *logrus.Logger, msgString string, err error) {
	if err != nil {
		log.Errorf("[%s] %s %v", pluginName, msgString, err.Error())
	}
}

func unsupportedField(pluginName string, log *logrus.Logger, msgString string) {
	// maybe only print out if log level is debug or ...
	log.Warnf("[%s] %s %s", pluginName, msgString, "unsupported")
}

// append the list of unsupported fields
func annotateUnsupported(pluginName string, src dcAPI.DeploymentConfig) map[string]string {

	annotations := src.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[pluginName+"/"+dcTriggers] = "unsupported"
	annotations[pluginName+"/"+dcTest] = "unsupported"

	return annotations
}

// Mutate converts a deploymentconfig to deployment
func Mutate(pluginName string, dc dcAPI.DeploymentConfig, log *logrus.Logger) (deployAPI.Deployment, error) {

	log.Tracef("[%s] dc %#v", pluginName, dc)

	deploy := deployAPI.Deployment{}

	// follow this guide for converting OpenShift DeploymentConfig to Kubernetes Deployment
	// https://gist.github.com/bmaupin/d5be3ca882345ff92e8336698230dae0

	// populate deploy with properties from dc object
	// metadata first
	deploy.Kind = "Deployment"
	deploy.APIVersion = "apps/v1"
	deploy.Name = dc.Name
	deploy.Namespace = dc.Namespace
	deploy.Annotations = annotateUnsupported(pluginName, dc)
	deploy.Labels = dc.Labels

	log.Debugf("[%s] Strategy: %#v", pluginName, dc.Spec.Strategy)
	// openshift Strategy.Type={"Recreate","Custom","Rolling"}
	// k8s.Strategy.Type={"Recreate","RollingUpdate"} - no custom, default to RollingUpdate
	if dc.Spec.Strategy.Type != "" {
		deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType(dc.Spec.Strategy.Type)
		if dc.Spec.Strategy.Type != "Recreate" {
			deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType("RollingUpdate")
		}
	}

	deploy.Spec.MinReadySeconds = dc.Spec.MinReadySeconds

	// dc.Triggers
	if dc.Spec.Triggers != nil {
		unsupportedField(pluginName, log, dcTriggers)
	}

	deploy.Spec.Replicas = &dc.Spec.Replicas

	deploy.Spec.RevisionHistoryLimit = dc.Spec.RevisionHistoryLimit

	if dc.Spec.Test == true {
		unsupportedField(pluginName, log, dcTest)
	}

	deploy.Spec.Paused = dc.Spec.Paused

	log.Debugf("[%s] dc.Selector: %#v", pluginName, dc.Spec.Selector)
	if dc.Spec.Selector != nil {
		deploy.Spec.Selector = new(v1.LabelSelector)

		deploy.Spec.Selector.MatchLabels = make(map[string]string)

		for index, element := range dc.Spec.Selector {
			deploy.Spec.Selector.MatchLabels[index] = element
		}
	}
	log.Debugf("[%s] d.Selector: %#v", pluginName, deploy.Spec.Selector)

	if dc.Spec.Template != nil {
		dc.Spec.Template.DeepCopyInto(&deploy.Spec.Template)
	}

	log.Tracef("[%s] deployment %#v", pluginName, deploy)

	return deploy, nil
}
