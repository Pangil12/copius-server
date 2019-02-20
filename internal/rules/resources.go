package rules

import (
	"context"
	"strings"

	"github.com/titpetric/factory"

	"github.com/crusttech/crust/internal/auth"
)

type resources struct {
	ctx context.Context
	db  *factory.DB
}

func NewResources(ctx context.Context, db *factory.DB) ResourcesInterface {
	return (&resources{}).With(ctx, db)
}

func (r *resources) With(ctx context.Context, db *factory.DB) ResourcesInterface {
	return &resources{
		ctx: ctx,
		db:  db,
	}
}

func (r *resources) identity() uint64 {
	return auth.GetIdentityFromContext(r.ctx).Identity()
}

func (r *resources) IsAllowed(resource string, operation string) Access {
	if strings.Contains(resource, "*") {
		return r.checkAccessMulti(resource, operation)
	}
	return r.checkAccess(resource, operation)
}

func (r *resources) checkAccessMulti(resource string, operation string) Access {
	user := r.identity()
	result := []Access{}
	query := []string{
		// select rules
		"select r.value from sys_rules r",
		// join members
		"inner join sys_role_member m on (m.rel_role = r.rel_role and m.rel_user=?)",
		// add conditions
		"where r.resource LIKE ? and r.operation=?",
	}
	resource = strings.Replace(resource, "*", "%", -1)
	queryString := strings.Join(query, " ")
	if err := r.db.Select(&result, queryString, user, resource, operation); err != nil {
		// @todo: log error
		return Deny
	}

	// order by deny, allow
	for _, val := range result {
		if val == Deny {
			return Deny
		}
	}
	for _, val := range result {
		if val == Allow {
			return Allow
		}
	}
	return Inherit
}

func (r *resources) checkAccess(resource string, operation string) Access {
	user := r.identity()
	result := []Access{}
	query := []string{
		// select rules
		"select r.value from sys_rules r",
		// join members
		"inner join sys_role_member m on (m.rel_role = r.rel_role and m.rel_user=?)",
		// add conditions
		"where r.resource=? and r.operation=?",
	}
	queryString := strings.Join(query, " ")
	if err := r.db.Select(&result, queryString, user, resource, operation); err != nil {
		// @todo: log error
		return Deny
	}

	// order by deny, allow
	for _, val := range result {
		if val == Deny {
			return Deny
		}
	}
	for _, val := range result {
		if val == Allow {
			return Allow
		}
	}
	return Inherit
}

func (r *resources) GrantByResource(roleID uint64, resource string, operations []string, value Access) error {
	return r.db.Transaction(func() error {
		row := Rule{
			RoleID:   roleID,
			Resource: resource,
			Value:    value,
		}

		var err error
		for _, operation := range operations {
			row.Operation = operation
			switch value {
			case Inherit:
				_, err = r.db.NamedExec("delete from sys_rules where rel_role=:rel_role and resource=:resource and operation=:operation", row)
			default:
				err = r.db.Replace("sys_rules", row)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *resources) ListByResource(roleID uint64, resource string) ([]Rule, error) {
	result := []Rule{}

	query := "select * from sys_rules where rel_role = ? and resource = ?"
	if err := r.db.Select(&result, query, roleID, resource); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *resources) Grant(roleID uint64, rules []Rule) error {
	return r.db.Transaction(func() error {
		var err error
		for _, rule := range rules {
			rule.RoleID = roleID

			switch rule.Value {
			case Inherit:
				_, err = r.db.NamedExec("delete from sys_rules where rel_role=:rel_role and resource=:resource and operation=:operation", rule)
			default:
				err = r.db.Replace("sys_rules", rule)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *resources) List(roleID uint64) ([]Rule, error) {
	result := []Rule{}

	query := "select * from sys_rules where rel_role = ?"
	if err := r.db.Select(&result, query, roleID); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *resources) Delete(roleID uint64) error {
	query := "delete from sys_rules where rel_role = ?"
	if _, err := r.db.Exec(query, roleID); err != nil {
		return err
	}
	return nil
}
