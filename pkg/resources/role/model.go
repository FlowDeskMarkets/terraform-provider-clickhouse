package resourcerole

import (
	"fmt"

	"github.com/Triple-Whale/terraform-provider-clickhouse/v4/pkg/common"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type CHGrant struct {
	RoleName   string `ch:"role_name"`
	AccessType string `ch:"access_type"`
	Database   string `ch:"database"`
}

type CHRole struct {
	Name       string `ch:"name"`
	Privileges []CHGrant
}

type RoleResource struct {
	Name       string
	Database   string
	Privileges *schema.Set
}

func (r *CHRole) ToRoleResource() (*RoleResource, error) {
	var database string
	var privileges []string
	for i := 0; i < len(r.Privileges); i++ {
		if database != "" && r.Privileges[i].Database != "" && r.Privileges[i].Database != database {
			return nil, fmt.Errorf("role %s has privileges on different databases", r.Name)
		}
		database = r.Privileges[i].Database
		privileges = append(privileges, r.Privileges[i].AccessType)
	}

	return &RoleResource{Name: r.Name, Database: database, Privileges: common.StringListToSet(privileges)}, nil
}

func (r *CHRole) GetPrivilegesList() []string {
	var privileges []string
	for _, privilege := range r.Privileges {
		privileges = append(privileges, privilege.AccessType)
	}
	return privileges
}
