package dc2deployment

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	dcAPI "github.com/openshift/api/apps/v1"
	"github.com/sirupsen/logrus"
	deployAPI "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	/*Test ensures that this deployment config will have zero replicas except
	     while a deployment is running. This allows the deployment config to be used
	     as a continuous deployment test - triggering on images, running the
	     deployment, and then succeeding or failing. Post strategy hooks and After
		 actions can be used to integrate successful deployment with an action.*/
	test = "DeploymentConfig.Spec.test"
	/*Triggers determine how updates to a DeploymentConfig result in new
	     deployments. If no triggers are defined, a new deployment can only occur as
	     a result of an explicit client update to the DeploymentConfig with a new
		 LatestVersion. If null, defaults to having a config change trigger.*/
	trigger = "DeploymentConfig.Spec.trigger"
	/*activeDeadlineSeconds is the duration in seconds that the deployer pods for
	  this deployment config may be active on a node before the system actively
	  tries to terminate them */
	activeDeadlineSeconds = "DeploymentConfig.Spec.Strategy.activeDeadlineSeconds"
	/*intervalSeconds is the time to wait between polling deployment status after
	  update. If the value is nil, a default will be used.*/
	intervalSeconds = "DeploymentConfig.Spec.Strategy.rollingParams.intervalSeconds"
	/*TimeoutSeconds is the time to wait for updates before giving up. If the
	  value is nil, a default will be used.*/
	timeoutSeconds = "DeploymentConfig.Spec.Strategy.rollingParams.timeoutSeconds"
	/*UpdatePeriodSeconds is the time to wait between individual pod updates. If
	  the value is nil, a default will be used.*/
	//Unable to Add since its exceeding the number of characters
	//updatePeriodSeconds = "DeploymentConfig.Spec.Strategy.rollingParams.updatePeriodSeconds"
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

	annotations[pluginName+"/"+test] = "unsupported"
	annotations[pluginName+"/"+trigger] = "unsupported"
	annotations[pluginName+"/"+activeDeadlineSeconds] = "unsupported"
	annotations[pluginName+"/"+intervalSeconds] = "unsupported"
	annotations[pluginName+"/"+timeoutSeconds] = "unsupported"
	annotations[pluginName+"/"+updatePeriodSeconds] = "unsupported"

	return annotations
}

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
	if dc.Spec.Strategy.Type != "" {
		if dc.Spec.Strategy.Type == "Rolling" || dc.Spec.Strategy.Type == "Custom" {
			deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType("RollingUpdate")
		} else {
			deploy.Spec.Strategy.Type = deployAPI.DeploymentStrategyType(dc.Spec.Strategy.Type)
		}

	}
	if deploy.Spec.Strategy.Type == "RollingUpdate" {
		deploy.Spec.Strategy.RollingUpdate = new(deployAPI.RollingUpdateDeployment)
		deploy.Spec.Strategy.RollingUpdate.MaxSurge = dc.Spec.Strategy.RollingParams.MaxSurge
		deploy.Spec.Strategy.RollingUpdate.MaxUnavailable = dc.Spec.Strategy.RollingParams.MaxUnavailable
	}
	deploy.Spec.RevisionHistoryLimit = dc.Spec.RevisionHistoryLimit

	if dc.Spec.Template != nil {
		dc.Spec.Template.DeepCopyInto(&deploy.Spec.Template)
	}
	// End of Spec Section

	//Status Section Start
	deploy.Status.AvailableReplicas = dc.Status.AvailableReplicas
	deploy.Status.ObservedGeneration = dc.Status.ObservedGeneration
	deploy.Status.Replicas = dc.Status.Replicas
	deploy.Status.UnavailableReplicas = dc.Status.UnavailableReplicas
	deploy.Status.UpdatedReplicas = dc.Status.UpdatedReplicas

	if dc.Status.Conditions != nil {
		deploy.Status.Conditions = make([](deployAPI.DeploymentCondition), len(dc.Status.Conditions))

		for i := range dc.Status.Conditions {

			deploy.Status.Conditions[i].Type = deployAPI.DeploymentConditionType(dc.Status.Conditions[i].Type)
			deploy.Status.Conditions[i].Status = dc.Status.Conditions[i].Status
			deploy.Status.Conditions[i].LastUpdateTime = dc.Status.Conditions[i].LastUpdateTime
			deploy.Status.Conditions[i].LastTransitionTime = dc.Status.Conditions[i].LastTransitionTime
			deploy.Status.Conditions[i].Reason = dc.Status.Conditions[i].Reason
			deploy.Status.Conditions[i].Message = dc.Status.Conditions[i].Message
		}
	}
	// End of Status Section
	
	//Return
	return deploy, err

}

