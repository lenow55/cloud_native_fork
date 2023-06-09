/*
Copyright The CloudNativePG Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package roles

import (
	"context"
	"fmt"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/cloudnative-pg/cloudnative-pg/pkg/management/postgres"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type funcCall struct{ verb, roleName string }

type mockRoleManager struct {
	roles       map[string]DatabaseRole
	callHistory []funcCall
}

var roleSynchronizer = RoleSynchronizer{
	instance: &postgres.Instance{
		Namespace: "myPod",
	},
}

func (m *mockRoleManager) List(_ context.Context) ([]DatabaseRole, error) {
	m.callHistory = append(m.callHistory, funcCall{"list", ""})
	re := make([]DatabaseRole, len(m.roles))
	i := 0
	for _, r := range m.roles {
		re[i] = r
		i++
	}
	return re, nil
}

func (m *mockRoleManager) Update(
	_ context.Context, role DatabaseRole,
) error {
	m.callHistory = append(m.callHistory, funcCall{"update", role.Name})
	_, found := m.roles[role.Name]
	if !found {
		return fmt.Errorf("tring to update unknown role: %s", role.Name)
	}
	m.roles[role.Name] = role
	return nil
}

func (m *mockRoleManager) UpdateComment(
	_ context.Context, role DatabaseRole,
) error {
	m.callHistory = append(m.callHistory, funcCall{"updateComment", role.Name})
	_, found := m.roles[role.Name]
	if !found {
		return fmt.Errorf("tring to update comment of unknown role: %s", role.Name)
	}
	m.roles[role.Name] = role
	return nil
}

func (m *mockRoleManager) Create(
	_ context.Context, role DatabaseRole,
) error {
	m.callHistory = append(m.callHistory, funcCall{"create", role.Name})
	_, found := m.roles[role.Name]
	if found {
		return fmt.Errorf("tring to create existing role: %s", role.Name)
	}
	m.roles[role.Name] = role
	return nil
}

func (m *mockRoleManager) Delete(
	_ context.Context, role DatabaseRole,
) error {
	m.callHistory = append(m.callHistory, funcCall{"delete", role.Name})
	_, found := m.roles[role.Name]
	if !found {
		return fmt.Errorf("tring to delete unknown role: %s", role.Name)
	}
	delete(m.roles, role.Name)
	return nil
}

func (m *mockRoleManager) GetLastTransactionID(_ context.Context, _ DatabaseRole) (int64, error) {
	return 0, nil
}

var _ = Describe("Role synchronizer tests", func() {
	It("it will Create ensure:present roles in spec missing from DB", func(ctx context.Context) {
		managedConf := apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:   "edb_test",
					Ensure: apiv1.EnsurePresent,
				},
				{
					Name:   "foo_bar",
					Ensure: apiv1.EnsurePresent,
				},
			},
		}
		rm := mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
			},
		}
		_, err := roleSynchronizer.synchronizeRoles(ctx, &rm, &managedConf, map[string]apiv1.PasswordState{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rm.callHistory).To(ConsistOf(
			[]funcCall{
				{"list", ""},
				{"create", "edb_test"},
				{"create", "foo_bar"},
			},
		))
		Expect(rm.callHistory).To(ConsistOf(
			funcCall{"list", ""},
			funcCall{"create", "edb_test"},
			funcCall{"create", "foo_bar"},
		))
	})

	It("it will ignore ensure:absent roles in spec missing from DB", func(ctx context.Context) {
		managedConf := apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:   "edb_test",
					Ensure: apiv1.EnsureAbsent,
				},
			},
		}
		rm := mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
			},
		}

		_, err := roleSynchronizer.synchronizeRoles(ctx, &rm, &managedConf, map[string]apiv1.PasswordState{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rm.callHistory).To(ConsistOf(funcCall{"list", ""}))
	})

	It("it will ignore DB roles that are not in spec", func(ctx context.Context) {
		managedConf := apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:   "edb_test",
					Ensure: apiv1.EnsureAbsent,
				},
			},
		}
		rm := mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"ignorezMoi": {
					Name:      "ignorezMoi",
					Superuser: true,
				},
			},
		}
		_, err := roleSynchronizer.synchronizeRoles(ctx, &rm, &managedConf, map[string]apiv1.PasswordState{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rm.callHistory).To(ConsistOf(funcCall{"list", ""}))
	})

	It("it will Delete ensure:absent roles that are in the DB", func(ctx context.Context) {
		managedConf := apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:   "edb_test",
					Ensure: apiv1.EnsureAbsent,
				},
			},
		}
		rm := mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"edb_test": {
					Name:      "edb_test",
					Superuser: true,
				},
			},
		}
		_, err := roleSynchronizer.synchronizeRoles(ctx, &rm, &managedConf, map[string]apiv1.PasswordState{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rm.callHistory).To(ConsistOf(
			funcCall{"list", ""},
			funcCall{"delete", "edb_test"},
		))
	})

	It("it will Update ensure:present roles that are in the DB but have different fields", func(ctx context.Context) {
		managedConf := apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:      "edb_test",
					Ensure:    apiv1.EnsurePresent,
					CreateDB:  true,
					BypassRLS: true,
				},
			},
		}
		rm := mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"edb_test": {
					Name:      "edb_test",
					Superuser: true,
				},
			},
		}
		_, err := roleSynchronizer.synchronizeRoles(ctx, &rm, &managedConf, map[string]apiv1.PasswordState{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(rm.callHistory).To(ConsistOf(
			funcCall{"list", ""},
			funcCall{"update", "edb_test"},
		))
	})
})

var _ = DescribeTable("Role status getter tests",
	func(spec *apiv1.ManagedConfiguration, db mockRoleManager, expected map[string]apiv1.RoleStatus) {
		ctx := context.TODO()

		roles, err := db.List(ctx)
		Expect(err).ToNot(HaveOccurred())

		statusMap := evaluateNextRoleActions(ctx, spec, roles, map[string]apiv1.PasswordState{}, nil).
			convertToRolesByStatus()

		// pivot the result to have a map: roleName -> Status, which is easier to compare for Ginkgo
		statusByRole := make(map[string]apiv1.RoleStatus)
		for action, roles := range statusMap {
			for _, role := range roles {
				statusByRole[role.Name] = action
			}
		}
		Expect(statusByRole).To(BeEquivalentTo(expected))
	},
	Entry("detects roles that are fully reconciled",
		&apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:      "ensurePresent",
					Superuser: true,
					Ensure:    apiv1.EnsurePresent,
				},
				{
					Name:      "ensureAbsent",
					Superuser: true,
					Ensure:    apiv1.EnsureAbsent,
				},
			},
		},
		mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"ensurePresent": {
					Name:      "ensurePresent",
					Superuser: true,
				},
			},
		},
		map[string]apiv1.RoleStatus{
			// TODO: at the moment, any role in DB and Spec will get reconciled
			// Once we can read SCRAM-SHA-256 and reconcile only on drift, this will change
			"ensurePresent": apiv1.RoleStatusPendingReconciliation,
			"ensureAbsent":  apiv1.RoleStatusReconciled,
			"postgres":      apiv1.RoleStatusReserved,
		},
	),
	Entry("detects roles that are not reconciled",
		&apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:      "unwantedInDB",
					Superuser: true,
					Ensure:    apiv1.EnsureAbsent,
				},
				{
					Name:      "missingFromDB",
					Superuser: true,
					Ensure:    apiv1.EnsurePresent,
				},
				{
					Name:      "drifted",
					Superuser: true,
					Ensure:    apiv1.EnsurePresent,
				},
			},
		},
		mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"unwantedInDB": {
					Name:      "unwantedInDB",
					Superuser: true,
				},
				"drifted": {
					Name:      "drifted",
					Superuser: false,
				},
			},
		},
		map[string]apiv1.RoleStatus{
			"postgres":      apiv1.RoleStatusReserved,
			"unwantedInDB":  apiv1.RoleStatusPendingReconciliation,
			"missingFromDB": apiv1.RoleStatusPendingReconciliation,
			"drifted":       apiv1.RoleStatusPendingReconciliation,
		},
	),
	Entry("detects roles that are not in the spec and ignores them",
		&apiv1.ManagedConfiguration{
			Roles: []apiv1.RoleConfiguration{
				{
					Name:      "edb_admin",
					Superuser: true,
					Ensure:    apiv1.EnsurePresent,
				},
			},
		},
		mockRoleManager{
			roles: map[string]DatabaseRole{
				"postgres": {
					Name:      "postgres",
					Superuser: true,
				},
				"edb_admin": {
					Name:      "edb_admin",
					Superuser: true,
				},
				"missingFromSpec": {
					Name:      "missingFromSpec",
					Superuser: false,
				},
			},
		},
		map[string]apiv1.RoleStatus{
			"postgres": apiv1.RoleStatusReserved,
			// TODO: at the moment, any role in DB and Spec will get reconciled
			// Once we can read SCRAM-SHA-256 and reconcile only on drift, this will change
			"edb_admin":       apiv1.RoleStatusPendingReconciliation,
			"missingFromSpec": apiv1.RoleStatusNotManaged,
		},
	),
)
