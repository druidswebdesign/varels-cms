package auth

import (
	"context"
	"testing"
)

func TestUserContextRoundTrip(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("found a user on an empty context")
	}

	want := User{ID: 7, Email: "owner@example.com", DisplayName: "Owner", Role: RoleAdmin}
	ctx := WithUser(context.Background(), want)

	got, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("user not found after WithUser")
	}
	if got != want {
		t.Errorf("user = %+v, want %+v", got, want)
	}
}
