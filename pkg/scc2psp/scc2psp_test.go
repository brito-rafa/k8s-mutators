package scc2psp

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	security "github.com/openshift/api/security/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
)

const clientName = "testClient"

func TestBuildPsp(t *testing.T) {
	m := newMutatorFromFileData(t, "full.json")
	scc := m.input
	psp := m.buildPsp()

	assert.Equal(t, "PodSecurityPolicy", psp.Kind)
	assert.Equal(t, "policy/v1beta1", psp.APIVersion)
	assert.Equal(t, scc.Name, psp.Name)
	assert.Equal(t, scc.Namespace, psp.Namespace)
	assert.Equal(t, len(psp.Annotations), 6)
	assert.Equal(t, 1, len(psp.Labels))
	assert.Contains(t, psp.Labels, "testkey")
	assert.Equal(t, "testval", psp.Labels["testkey"])
	assert.Equal(t, scc.AllowPrivilegedContainer, psp.Spec.Privileged)
	assert.Equal(t, 1, len(psp.Spec.DefaultAddCapabilities))
	assert.Contains(t, psp.Spec.DefaultAddCapabilities[0], "SETUID")
	assert.Equal(t, 1, len(psp.Spec.RequiredDropCapabilities))
	assert.Contains(t, psp.Spec.RequiredDropCapabilities[0], "MKNOD")
	assert.Equal(t, 1, len(psp.Spec.AllowedCapabilities))
	assert.Contains(t, psp.Spec.AllowedCapabilities[0], "KILL")
	assert.Equal(t, 6, len(psp.Spec.Volumes))

	expected := []string{"configMap", "downwardAPI", "emptyDir", "persistentVolumeClaim", "projected", "secret"}

	var volumes []string
	for _, volume := range psp.Spec.Volumes {
		volumes = append(volumes, string(volume))
	}

	for _, volume := range volumes {
		assert.Contains(t, expected, volume)
	}

	assert.Equal(t, 1, len(psp.Spec.AllowedFlexVolumes))
	assert.Contains(t, psp.Spec.AllowedFlexVolumes[0].Driver, "testDriver")

	assert.Equal(t, scc.AllowHostNetwork, psp.Spec.HostNetwork)
	assert.Equal(t, scc.AllowHostPID, psp.Spec.HostPID)
	assert.Equal(t, scc.AllowHostIPC, psp.Spec.HostIPC)
	assert.Equal(t, scc.DefaultAllowPrivilegeEscalation, psp.Spec.DefaultAllowPrivilegeEscalation)
	assert.Equal(t, scc.AllowPrivilegeEscalation, psp.Spec.AllowPrivilegeEscalation)
	assert.Equal(t, v1beta1.SELinuxStrategy(scc.SELinuxContext.Type), psp.Spec.SELinux.Rule)

	assert.Equal(t, scc.SELinuxContext.SELinuxOptions.User, psp.Spec.SELinux.SELinuxOptions.User)
	assert.Equal(t, scc.SELinuxContext.SELinuxOptions.Role, psp.Spec.SELinux.SELinuxOptions.Role)
	assert.Equal(t, scc.SELinuxContext.SELinuxOptions.Type, psp.Spec.SELinux.SELinuxOptions.Type)
	assert.Equal(t, scc.SELinuxContext.SELinuxOptions.Level, psp.Spec.SELinux.SELinuxOptions.Level)

	assert.Equal(t, v1beta1.RunAsUserStrategy("RunAsAny"), psp.Spec.RunAsUser.Rule)

	assert.Equal(t, 1, len(psp.Spec.RunAsUser.Ranges))
	assert.Equal(t, int64(1000), psp.Spec.RunAsUser.Ranges[0].Min)
	assert.Equal(t, int64(2000), psp.Spec.RunAsUser.Ranges[0].Max)

	assert.Equal(t, v1beta1.SupplementalGroupsStrategyType(scc.SupplementalGroups.Type), psp.Spec.SupplementalGroups.Rule)

	assert.Equal(t, 1, len(psp.Spec.SupplementalGroups.Ranges))
	assert.Equal(t, int64(3000), psp.Spec.SupplementalGroups.Ranges[0].Min)
	assert.Equal(t, int64(4000), psp.Spec.SupplementalGroups.Ranges[0].Max)

	assert.Equal(t, v1beta1.FSGroupStrategyType(scc.FSGroup.Type), psp.Spec.FSGroup.Rule)

	assert.Equal(t, 1, len(psp.Spec.FSGroup.Ranges))
	assert.Equal(t, int64(5000), psp.Spec.FSGroup.Ranges[0].Min)
	assert.Equal(t, int64(6000), psp.Spec.FSGroup.Ranges[0].Max)

	assert.Equal(t, scc.ReadOnlyRootFilesystem, psp.Spec.ReadOnlyRootFilesystem)
	assert.Equal(t, scc.AllowedUnsafeSysctls, psp.Spec.AllowedUnsafeSysctls)
	assert.Equal(t, scc.ForbiddenSysctls, psp.Spec.ForbiddenSysctls)
}

func TestBuildPspDefaultEmptyElements(t *testing.T) {
	scc := security.SecurityContextConstraints{}
	m := NewMutator(clientName, logrus.New(), scc)
	psp := m.buildPsp()

	assert.Equal(t, 0, len(psp.Annotations))
	assert.Equal(t, 0, len(psp.Spec.Volumes))
	assert.Equal(t, 0, len(psp.Spec.AllowedFlexVolumes))
	assert.Equal(t, ((*v1.SELinuxOptions)(nil)), psp.Spec.SELinux.SELinuxOptions)
}

func TestBuildClusterRole(t *testing.T) {
	scc := security.SecurityContextConstraints{}
	scc.Name = "crTest"

	m := NewMutator(clientName, logrus.New(), scc)
	cr := m.buildClusterRole()

	assert.Equal(t, "ClusterRole", cr.Kind)
	assert.Equal(t, "rbac.authorization.k8s.io/v1", cr.APIVersion)
	assert.Equal(t, "vmware-psp:"+scc.Name, cr.Name)
	assert.Equal(t, 1, len(cr.Rules))
	assert.Equal(t, 1, len(cr.Rules[0].Verbs))
	assert.Equal(t, "use", cr.Rules[0].Verbs[0])
	assert.Equal(t, 1, len(cr.Rules[0].APIGroups))
	assert.Equal(t, "rbac.authorization.k8s.io", cr.Rules[0].APIGroups[0])
	assert.Equal(t, 1, len(cr.Rules[0].Resources))
	assert.Equal(t, "podsecuritypolicies", cr.Rules[0].Resources[0])
	assert.Equal(t, scc.Name, cr.Rules[0].ResourceNames[0])
}

func TestBuildClusterRoleBindingNoUsersOrGroups(t *testing.T) {
	scc := security.SecurityContextConstraints{}
	m := NewMutator(clientName, logrus.New(), scc)
	crb := m.buildClusterRoleBinding()

	assert.ObjectsAreEqual(rbac.ClusterRoleBinding{}, crb)
}

func TestBuildClusterRoleBindingWithServiceAccountUser(t *testing.T) {
	scc := security.SecurityContextConstraints{}

	scc.Users = make([]string, 1)
	scc.Users[0] = "system:serviceaccount:default:default"

	m := NewMutator(clientName, logrus.New(), scc)
	crb := m.buildClusterRoleBinding()

	apiGroup := "rbac.authorization.k8s.io"
	mutatedName := "vmware-psp:" + scc.Name

	assert.Equal(t, 1, len(scc.Users))
	assert.Equal(t, 0, len(scc.Groups))
	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)
	assert.Equal(t, apiGroup+"/v1", crb.APIVersion)
	assert.Equal(t, mutatedName, crb.Name)
	assert.Equal(t, "ServiceAccount", crb.Subjects[0].Kind)
	assert.Equal(t, "default", crb.Subjects[0].Name)
	assert.Equal(t, "default", crb.Subjects[0].Namespace)
	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, apiGroup, crb.RoleRef.APIGroup)
	assert.Equal(t, mutatedName, crb.RoleRef.Name)
}

func TestBuildClusterRoleBindingWithNonServiceAccountUser(t *testing.T) {
	scc := security.SecurityContextConstraints{}

	scc.Users = make([]string, 1)
	scc.Users[0] = "testservice:testaccount:default:default"

	m := NewMutator(clientName, logrus.New(), scc)
	crb := m.buildClusterRoleBinding()

	apiGroup := "rbac.authorization.k8s.io"
	mutatedName := "vmware-psp:" + scc.Name

	assert.Equal(t, 1, len(scc.Users))
	assert.Equal(t, 0, len(scc.Groups))
	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)
	assert.Equal(t, apiGroup+"/v1", crb.APIVersion)
	assert.Equal(t, mutatedName, crb.Name)
	assert.Equal(t, "User", crb.Subjects[0].Kind)
	assert.Equal(t, "testservice:testaccount:default:default", crb.Subjects[0].Name)
	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, apiGroup, crb.RoleRef.APIGroup)
	assert.Equal(t, mutatedName, crb.RoleRef.Name)
}

func TestBuildClusterRoleBindingWithDisallowedUsers(t *testing.T) {
	scc := security.SecurityContextConstraints{}

	disallowedUsers := []string{"system:serviceaccount:velero:admin",
		"system:serviceaccount:openshift:admin",
		"system:serviceaccount:management-infra:admin"}

	for _, disallowedUser := range disallowedUsers {
		scc.Users = make([]string, 1)
		scc.Users[0] = disallowedUser

		m := NewMutator(clientName, logrus.New(), scc)
		crb := m.buildClusterRoleBinding()

		assert.Equal(t, 0, len(crb.Subjects))
		assert.True(t, reflect.DeepEqual(crb, rbac.ClusterRoleBinding{}))
	}
}

func TestBuildClusterRoleBindingWithGroup(t *testing.T) {
	scc := security.SecurityContextConstraints{}

	scc.Groups = make([]string, 1)
	scc.Groups[0] = "system:authenticated"

	m := NewMutator(clientName, logrus.New(), scc)
	crb := m.buildClusterRoleBinding()

	apiGroup := "rbac.authorization.k8s.io"
	mutatedName := "vmware-psp:" + scc.Name

	assert.Equal(t, 0, len(scc.Users))
	assert.Equal(t, 1, len(scc.Groups))
	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)
	assert.Equal(t, apiGroup+"/v1", crb.APIVersion)
	assert.Equal(t, mutatedName, crb.Name)
	assert.Equal(t, "Group", crb.Subjects[0].Kind)
	assert.Equal(t, apiGroup, crb.Subjects[0].APIGroup)
	assert.Equal(t, "system:authenticated", crb.Subjects[0].Name)
	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, apiGroup, crb.RoleRef.APIGroup)
	assert.Equal(t, mutatedName, crb.RoleRef.Name)
}

func TestBuildRoleBindingWithDisallowedGroups(t *testing.T) {
	scc := security.SecurityContextConstraints{}

	disallowedGroups := []string{"velero:authenticated",
		"openshift:authenticated",
		"management-infra:authenticated"}

	for _, disdisallowedGroup := range disallowedGroups {
		scc.Groups = make([]string, 1)
		scc.Groups[0] = disdisallowedGroup

		m := NewMutator(clientName, logrus.New(), scc)
		crb := m.buildClusterRoleBinding()

		assert.Equal(t, 0, len(crb.Subjects))
		assert.True(t, reflect.DeepEqual(crb, rbac.ClusterRoleBinding{}))
	}
}

func newMutatorFromFileData(t *testing.T, fileName string) Mutator {
	sccFilePath := filepath.Join("testdata", fileName)
	sccFile, err := ioutil.ReadFile(sccFilePath)

	if err != nil {
		t.Fatal(err)
	}

	scc := security.SecurityContextConstraints{}
	err = json.Unmarshal(sccFile, &scc)

	if err != nil {
		t.Errorf("Failed to unmarshall SecurityContextConstraints JSON = %v", err)
	}

	return NewMutator(clientName, logrus.New(), scc)
}
