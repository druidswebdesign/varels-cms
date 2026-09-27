package auth

import "context"

// User is the authenticated identity carried in the request context.
type User struct {
	ID          int64
	Email       string
	DisplayName string
	Role        Role
}

type userContextKey struct{}

// WithUser stores the authenticated user on the context.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

// UserFromContext returns the authenticated user, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey{}).(User)
	return u, ok
}
