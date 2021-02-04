package scc2psp

import (
	"regexp"

	security "github.com/openshift/api/security/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	policy "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
)

const rbacAPIGroup = "rbac.authorization.k8s.io"

// MutatorOutput contains the mutated output structures
type MutatorOutput struct {
	PodSecurityPolicy  policy.PodSecurityPolicy
	ClusterRole        rbac.ClusterRole
	ClusterRoleBinding rbac.ClusterRoleBinding
}

// Mutator contains common atttributes and the mutation input source structure
type Mutator struct {
	name  string
	log   logrus.FieldLogger
	input security.SecurityContextConstraints
}

// NewMutator creates a new Mutator. Clients of this API should set a meaningful name that can be used
// to easily identify the calling client.
func NewMutator(name string, log logrus.FieldLogger, scc security.SecurityContextConstraints) Mutator {
	return Mutator{
		name:  name,
		log:   log,
		input: scc,
	}
}

// Mutate converts a SecurityContextsConstraints into PodSecurityPolicy, ClusterRole and ClusterRoleBinding
func (m *Mutator) Mutate() *MutatorOutput {
	m.log.Debugf("[%s] input to mutate = %#v", m.name, m.input)

	return &MutatorOutput{
		PodSecurityPolicy:  m.buildPsp(),
		ClusterRole:        m.buildClusterRole(),
		ClusterRoleBinding: m.buildClusterRoleBinding(),
	}
}

func (m *Mutator) buildPsp() policy.PodSecurityPolicy {
	scc := m.input

	psp := policy.PodSecurityPolicy{}
	psp.Kind = "PodSecurityPolicy"
	psp.APIVersion = "policy/v1beta1"
	psp.Name = scc.Name
	psp.Namespace = scc.Namespace
	psp.Annotations = m.annotateUnsupportedFields(scc)
	psp.Labels = scc.Labels
	psp.Spec.Privileged = scc.AllowPrivilegedContainer
	psp.Spec.DefaultAddCapabilities = scc.DefaultAddCapabilities
	psp.Spec.RequiredDropCapabilities = scc.RequiredDropCapabilities
	psp.Spec.AllowedCapabilities = scc.AllowedCapabilities

	if scc.Volumes != nil {
		psp.Spec.Volumes = make([]policy.FSType, len(scc.Volumes))

		for i := range scc.Volumes {
			psp.Spec.Volumes[i] = (v1beta1.FSType)(scc.Volumes[i])
		}
	}

	if scc.AllowedFlexVolumes != nil {
		psp.Spec.AllowedFlexVolumes = make([]policy.AllowedFlexVolume, len(scc.AllowedFlexVolumes))

		for i := range scc.AllowedFlexVolumes {
			psp.Spec.AllowedFlexVolumes[i].Driver = scc.AllowedFlexVolumes[i].Driver
		}
	}

	psp.Spec.HostNetwork = scc.AllowHostNetwork
	psp.Spec.HostPID = scc.AllowHostPID
	psp.Spec.HostIPC = scc.AllowHostIPC
	psp.Spec.DefaultAllowPrivilegeEscalation = scc.DefaultAllowPrivilegeEscalation
	psp.Spec.AllowPrivilegeEscalation = scc.AllowPrivilegeEscalation
	psp.Spec.SELinux.Rule = v1beta1.SELinuxStrategy(scc.SELinuxContext.Type)

	if scc.SELinuxContext.SELinuxOptions != nil {
		psp.Spec.SELinux.SELinuxOptions = &v1.SELinuxOptions{}
		psp.Spec.SELinux.SELinuxOptions.User = scc.SELinuxContext.SELinuxOptions.User
		psp.Spec.SELinux.SELinuxOptions.Role = scc.SELinuxContext.SELinuxOptions.Role
		psp.Spec.SELinux.SELinuxOptions.Type = scc.SELinuxContext.SELinuxOptions.Type
		psp.Spec.SELinux.SELinuxOptions.Level = scc.SELinuxContext.SELinuxOptions.Level
	}

	psp.Spec.RunAsUser.Rule = "RunAsAny"

	if scc.RunAsUser.Type != "MustRunAsRange" {
		psp.Spec.RunAsUser.Rule = v1beta1.RunAsUserStrategy(scc.RunAsUser.Type)
	}

	if scc.RunAsUser.UIDRangeMin != nil && scc.RunAsUser.UIDRangeMax != nil {
		psp.Spec.RunAsUser.Ranges = make([]policy.IDRange, 1)
		psp.Spec.RunAsUser.Ranges[0].Min = *scc.RunAsUser.UIDRangeMin
		psp.Spec.RunAsUser.Ranges[0].Max = *scc.RunAsUser.UIDRangeMax
	}

	psp.Spec.SupplementalGroups.Rule = (v1beta1.SupplementalGroupsStrategyType)(scc.SupplementalGroups.Type)

	if scc.SupplementalGroups.Ranges != nil {
		psp.Spec.SupplementalGroups.Ranges = make([]policy.IDRange, len(scc.SupplementalGroups.Ranges))

		for i := range scc.SupplementalGroups.Ranges {
			psp.Spec.SupplementalGroups.Ranges[i].Min = scc.SupplementalGroups.Ranges[i].Min
			psp.Spec.SupplementalGroups.Ranges[i].Max = scc.SupplementalGroups.Ranges[i].Max
		}
	}

	psp.Spec.FSGroup.Rule = v1beta1.FSGroupStrategyType(scc.FSGroup.Type)

	if scc.FSGroup.Ranges != nil {
		psp.Spec.FSGroup.Ranges = make([]policy.IDRange, len(scc.FSGroup.Ranges))

		for i := range scc.FSGroup.Ranges {
			psp.Spec.FSGroup.Ranges[i].Min = scc.FSGroup.Ranges[i].Min
			psp.Spec.FSGroup.Ranges[i].Max = scc.FSGroup.Ranges[i].Max
		}
	}

	psp.Spec.ReadOnlyRootFilesystem = scc.ReadOnlyRootFilesystem
	psp.Spec.AllowedUnsafeSysctls = scc.AllowedUnsafeSysctls
	psp.Spec.ForbiddenSysctls = scc.ForbiddenSysctls

	m.log.Debugf("[%s] mutated PSP = %#v", m.name, psp)

	return psp
}

func (m *Mutator) annotateUnsupportedFields(scc security.SecurityContextConstraints) map[string]string {
	annotations := scc.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotateUnsupportedField := func(fieldName string) {
		field := "SecurityContextConstraints." + fieldName
		annotations[m.name+"/"+field] = "unsupported"
		m.log.Warnf("[%s] %s is unsupported", m.name, field)
	}

	if scc.Priority != nil {
		annotateUnsupportedField("Priority")
	}

	if scc.RunAsUser.UID != nil {
		annotateUnsupportedField("RunAsUser.UID")
	}

	if scc.SeccompProfiles != nil {
		annotateUnsupportedField("SeccompProfiles")
	}

	if scc.AllowHostDirVolumePlugin {
		annotateUnsupportedField("AllowHostDirVolumePlugin")
	}

	if scc.AllowHostPorts {
		annotateUnsupportedField("AllowHostPorts")
	}

	return annotations
}

func (m *Mutator) buildClusterRole() rbac.ClusterRole {
	scc := m.input

	clusterrole := rbac.ClusterRole{}
	clusterrole.Rules = make([]rbac.PolicyRule, 1)
	clusterrole.Kind = "ClusterRole"
	clusterrole.APIVersion = rbacAPIGroup + "/v1"
	clusterrole.Name = "vmware-psp:" + scc.Name

	clusterrole.Rules[0].Verbs = make([]string, 1)
	clusterrole.Rules[0].Verbs = []string{"use"}

	clusterrole.Rules[0].APIGroups = make([]string, 1)
	clusterrole.Rules[0].APIGroups = []string{rbacAPIGroup}

	clusterrole.Rules[0].Resources = make([]string, 1)
	clusterrole.Rules[0].Resources = []string{"podsecuritypolicies"}

	clusterrole.Rules[0].ResourceNames = make([]string, 1)
	clusterrole.Rules[0].ResourceNames = []string{scc.Name}

	m.log.Debugf("[%s] mutated clusterrole.Rules = %#v", m.name, clusterrole.Rules)

	return clusterrole
}

var (
	userIgnores    = regexp.MustCompile(`openshift|velero|management-infra`)
	colon          = regexp.MustCompile(":")
	serviceAccount = regexp.MustCompile("system:serviceaccount")
)

func (m *Mutator) buildClusterRoleBinding() rbac.ClusterRoleBinding {
	scc := m.input

	if (len(scc.Users) + len(scc.Groups)) > 0 {
		crb := rbac.ClusterRoleBinding{}
		crb.Kind = "ClusterRoleBinding"
		crb.APIVersion = rbacAPIGroup + "/v1"
		crb.Name = "vmware-psp:" + scc.Name

		userSubjects := m.buildClusterRoleBindingUserSubjects(scc, crb)

		if len(userSubjects) > 0 {
			for _, subject := range userSubjects {
				crb.Subjects = append(crb.Subjects, subject)
			}
		}

		groupSubjects := m.buildClusterRoleBindingGroupSubjects(scc)

		if len(groupSubjects) > 0 {
			for _, subject := range groupSubjects {
				crb.Subjects = append(crb.Subjects, subject)
			}
		}

		crb.RoleRef.Kind = "ClusterRole"
		crb.RoleRef.APIGroup = rbacAPIGroup
		crb.RoleRef.Name = "vmware-psp:" + scc.Name

		if len(crb.Subjects) > 0 {
			m.log.Debugf("[%s] mutated clusterrolebinding = %#v", m.name, crb)
			return crb
		}
	}

	m.log.Debugf("[%s] clusterrolebinding not created as preconditions were not met for input = %#v", m.name, m.input)

	return rbac.ClusterRoleBinding{}
}

func (m Mutator) buildClusterRoleBindingUserSubjects(scc security.SecurityContextConstraints, crb rbac.ClusterRoleBinding) []rbac.Subject {
	subjects := []rbac.Subject{}

	for i := 0; i < len(scc.Users); i++ {
		if !(userIgnores.MatchString(scc.Users[i])) {
			subject := rbac.Subject{}

			if serviceAccount.MatchString(scc.Users[i]) {
				subject.Kind = "ServiceAccount"

				// parse the Users into system:serviceaccount:<namespace>:<name>
				userSplit := colon.Split(scc.Users[i], -1)
				if userSplit != nil {
					subject.Name = userSplit[len(userSplit)-1]
					subject.Namespace = userSplit[len(userSplit)-2]
				}
			} else {
				subject.Kind = "User"
				subject.Name = scc.Users[i]
				subject.APIGroup = rbacAPIGroup
			}

			subjects = append(subjects, subject)

			m.log.Debugf("[%s] User subjects = %#v", m.name, subject)
		}
	}

	return subjects
}

func (m Mutator) buildClusterRoleBindingGroupSubjects(scc security.SecurityContextConstraints) []rbac.Subject {
	subjects := []rbac.Subject{}

	for i := 0; i < len(scc.Groups); i++ {
		if !(userIgnores.MatchString(scc.Groups[i])) {
			subject := rbac.Subject{}
			subject.Kind = "Group"
			subject.APIGroup = rbacAPIGroup
			subject.Name = scc.Groups[i]

			subjects = append(subjects, subject)

			m.log.Debugf("[%s] Group subject = %#v", m.name, subject)
		}
	}

	return subjects
}
