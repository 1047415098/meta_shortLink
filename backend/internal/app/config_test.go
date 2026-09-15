package app

import (
	"strings"
	"testing"
)

func TestConfiguredAdminPassword(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("APP_SECRET", strings.Repeat("s", 32))
	t.Setenv("PUBLIC_BASE_URL", "https://example.com")
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{{"explicit admin", "admin", true}, {"empty", "", false}, {"bcrypt limit", strings.Repeat("a", 73), false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ADMIN_PASSWORD", tc.password)
			c, e := LoadConfig()
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, e)
			}
			if e == nil && c.AdminPassword != tc.password {
				t.Fatal("password changed")
			}
		})
	}
}
