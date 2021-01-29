package dc2deployment

import (
	dcAPI "github.com/openshift/api/apps/v1"
	"github.com/sirupsen/logrus"
	deployAPI "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Test ensures that this deployment config will have zero replicas except
	// while a deployment is running. This allows the deployment config to be used
	// as a continuous deployment test - triggering on images, running the
	// deployment, and then succeeding or failing. Post strategy hooks and After
	// actions can be used to integrate successful deployment with an action.
	dcTest = "DeploymentConfig.Spec.test"
	// Triggers determine how updates to a DeploymentConfig result in new
	// deployments. If no triggers are defined, a new deployment can only occur as
	// a result of an explicit client update to the DeploymentConfig with a new
	// LatestVersion. If null, defaults to having a config change trigger.
	dcTriggers = "DeploymentConfig.Spec.triggers"
	// activeDeadlineSeconds is the duration in seconds that the deployer pods for
	// this deployment config may be active on a node before the system actively
	// tries to terminate them
	dcActiveDeadlineSeconds = "DeploymentConfig.Spec.Strategy.activeDeadlineSeconds"
)

func unsupportedField(pluginName string, log logrus.FieldLogger, msgString string) {
	// maybe only print out if log level is debug or ...
	log.Warnf("[%s] %s %s", pluginName, msgString, "unsupported")
}

func annotateUnsupported(pluginName string, src deployAPI.Deployment) map[string]string {

	annotations := src.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[pluginName+"/"+dcTest] = "unsupported"
	annotations[pluginName+"/"+dcTriggers] = "unsupported"
	annotations[pluginName+"/"+dcActiveDeadlineSeconds] = "unsupported"

	return annotations
}

// Mutate converts a deploymentconfig to deployment
func Mutate(pluginName string, log logrus.FieldLogger, dc dcAPI.DeploymentConfig) (deployAPI.Deployment, error) {

	deploy := deployAPI.Deployment{}

	//MetaData Section Start
	deploy.Kind = "Deployment"
	deploy.APIVersion = "apps/v1"
	deploy.Name = dc.Name
	deploy.Namespace = dc.Namespace
	deploy.Labels = dc.Labels
	deploy.Annotations = annotateUnsupported(pluginName, deploy)
	//End of MetaData Section

	//Spec section start
	deploy.Spec.Replicas = &dc.Spec.Replicas
	deploy.Spec.RevisionHistoryLimit = dc.Spec.RevisionHistoryLimit
	deploy.Spec.Paused = dc.Spec.Paused
	deploy.Spec.MinReadySeconds = dc.Spec.MinReadySeconds
	if dc.Spec.Selector != nil {
		deploy.Spec.Selector = new(v1.LabelSelector)

		deploy.Spec.Selector.MatchLabels = make(map[string]string)

		for index, element := range dc.Spec.Selector {
			deploy.Spec.Selector.MatchLabels[index] = element
		}
	}
	log.Debugf("[%s] Strategy: %#v", pluginName, dc.Spec.Strategy)

	// openshift supoorts Strategy.Type={"Recreate","Custom","Rolling"}
	// K8s native supports k8s.Strategy.Type={"Recreate","RollingUpdate"}
	// Custom strategy.type is not supported in K8s native, hence defaulting it to RollingUpdate
	// In case the Strategy.type from Openshift is Recreate then a default value gets assigned.
	if dc.Spec.Strategy.Type != "" {
		if dc.Spec.Strategy.Type == "Rolling" || dc.Spec.Strategy.Type == "Custom" {
			deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType("RollingUpdate")
		} else {
			deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType(dc.Spec.Strategy.Type)
		}

	}
	//Rollingupdate.MaxSurge and Rollingupdate.MaxUnavailable supported only when Strategy type is "RollingUpdate"
	//If openshift Strategy.type is Custom Rollingupdate.MaxSurge and Rollingupdate.MaxUnavailable is not applicable.
	if deploy.Spec.Strategy.Type == "RollingUpdate" && dc.Spec.Strategy.Type != "Custom" {
		deploy.Spec.Strategy.RollingUpdate = new(deployAPI.RollingUpdateDeployment)
		deploy.Spec.Strategy.RollingUpdate.MaxSurge = dc.Spec.Strategy.RollingParams.MaxSurge
		deploy.Spec.Strategy.RollingUpdate.MaxUnavailable = dc.Spec.Strategy.RollingParams.MaxUnavailable
	}
	deploy.Spec.RevisionHistoryLimit = dc.Spec.RevisionHistoryLimit

	if dc.Spec.Template != nil {
		dc.Spec.Template.DeepCopyInto(&deploy.Spec.Template)
	}
	// End of Spec Section

	//Logging the unsupported fields start
	// dc.Triggers
	if dc.Spec.Triggers != nil {
		unsupportedField(pluginName, log, dcTriggers)
	}
	//dc.Test
	if dc.Spec.Test == true {
		unsupportedField(pluginName, log, dcTest)
	}
	//dc.Spec.Strategy.ActiveDeadlineSeconds
	if dc.Spec.Strategy.ActiveDeadlineSeconds != nil {
		unsupportedField(pluginName, log, dcActiveDeadlineSeconds)
	}
	//End of unsupported fileds

	//Return
	return deploy, nil

}
