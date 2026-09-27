package auth

// Role identifies a user's permission level. Only admin and staff can log in;
// clients are customers and providers are suppliers (ADR-0018).
type Role string

const (
	RoleAdmin Role = "admin"
	RoleStaff Role = "staff"
)

// rank orders roles for the RequireRole minimum check.
func (r Role) rank() int {
	switch r {
	case RoleAdmin:
		return 2
	case RoleStaff:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether r is at or above min.
func (r Role) AtLeast(min Role) bool {
	return r.rank() >= min.rank()
}

// CanViewFinancials reports whether the role may see buy prices, margins,
// net profit, OpEx, and financial exports.
func (r Role) CanViewFinancials() bool {
	return r == RoleAdmin
}
