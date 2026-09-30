package auth

import "testing"

func TestRoleAtLeast(t *testing.T) {
	tests := []struct {
		role Role
		min  Role
		want bool
	}{
		{RoleAdmin, RoleStaff, true},
		{RoleAdmin, RoleAdmin, true},
		{RoleStaff, RoleStaff, true},
		{RoleStaff, RoleAdmin, false},
		{Role("client"), RoleStaff, false},
		{Role(""), RoleStaff, false},
	}
	for _, tt := range tests {
		if got := tt.role.AtLeast(tt.min); got != tt.want {
			t.Errorf("%q.AtLeast(%q) = %v, want %v", tt.role, tt.min, got, tt.want)
		}
	}
}

func TestCanViewFinancials(t *testing.T) {
	if !RoleAdmin.CanViewFinancials() {
		t.Error("admin must see financials")
	}
	for _, r := range []Role{RoleStaff, Role("client"), Role("")} {
		if r.CanViewFinancials() {
			t.Errorf("%q must not see financials", r)
		}
	}
}
