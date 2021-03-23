package dc2deployment

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	apps "github.com/openshift/api/apps/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDeployment(t *testing.T) {
	type output struct {
		kind         string
		apiVersion   string
		strategyType string
	}

	tests := []struct {
		name  string
		input string
		want  output
	}{
		{
			"Test config map with Rolling Strategy",
			"example_with_Rolling.json",
			output{
				kind:         "Deployment",
				apiVersion:   "apps/v1",
				strategyType: "RollingUpdate",
			},
		},
		{
			"Test config map with Recreate Strategy",
			"example_with_Recreate.json",
			output{
				kind:         "Deployment",
				apiVersion:   "apps/v1",
				strategyType: "Recreate",
			},
		},
		{
			"Test config map with Custom Strategy",
			"example_with_Rolling.json",
			output{
				kind:         "Deployment",
				apiVersion:   "apps/v1",
				strategyType: "RollingUpdate",
			},
		},
	}

	for _, tc := range tests {
		dcInput := newMutatorFromFileData(t, tc.input, tc.name)
		m, _ := Mutate(tc.name, logrus.New(), dcInput)

		assert.Equal(t, tc.want.kind, m.Kind)
		assert.Equal(t, tc.want.apiVersion, m.APIVersion)
		assert.Equal(t, dcInput.Name, m.Name)
		assert.Equal(t, dcInput.Spec.Template.Spec, m.Spec.Template.Spec)

		if dcInput.Spec.Strategy.Type == "Custom" || dcInput.Spec.Strategy.Type == "Rolling" {
			assert.Contains(t, tc.want.strategyType, m.Spec.Strategy.Type)
		} else {
			assert.Contains(t, dcInput.Spec.Strategy.Type, m.Spec.Strategy.Type)
		}

		if dcInput.Spec.Strategy.Type == "Rolling" {
			assert.EqualValues(t, dcInput.Spec.Strategy.RollingParams.MaxSurge, m.Spec.Strategy.RollingUpdate.MaxSurge)
			assert.EqualValues(t,
				dcInput.Spec.Strategy.RollingParams.MaxUnavailable,
				m.Spec.Strategy.RollingUpdate.MaxUnavailable)
		}

	}
}

func newMutatorFromFileData(t *testing.T, fileName, testName string) apps.DeploymentConfig {
	dcConfigFilePath := filepath.Join("testdata", fileName)
	dc2File, err := ioutil.ReadFile(dcConfigFilePath)
	if err != nil {
		t.Fatalf("%s: %v", testName, err)
	}

	dc := apps.DeploymentConfig{}
	err = json.Unmarshal(dc2File, &dc)
	if err != nil {
		t.Errorf("%s: unmarshall DeploymentConfig  JSON = %v", testName, err)
	}

	return dc
}
