package app

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	originalPrefix := EnvPrefix
	t.Cleanup(func() { EnvPrefix = originalPrefix })

	for _, prefix := range []string{"MYCLI__", "CUSTOM__", ""} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			EnvPrefix = prefix
			key := prefix + "GETENV_TEST_VALUE"
			t.Setenv(key, "configured")
			if got := GetEnv("GETENV_TEST_VALUE", "fallback"); got != "configured" {
				t.Fatalf("GetEnv() = %q, want configured", got)
			}

			t.Setenv(key, "")
			if got := GetEnv("GETENV_TEST_VALUE", "fallback"); got != "" {
				t.Fatalf("GetEnv() = %q, want an empty value", got)
			}

			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
			if got := GetEnv("GETENV_TEST_VALUE", "fallback"); got != "fallback" {
				t.Fatalf("GetEnv() = %q, want fallback", got)
			}
		})
	}
}

func TestMyCliHomeCustomPrefix(t *testing.T) {
	originalPrefix := EnvPrefix
	t.Cleanup(func() { EnvPrefix = originalPrefix })
	EnvPrefix = "CUSTOM__"
	t.Setenv("MYCLI__HOME", "/ignored")
	t.Setenv("CUSTOM__HOME", "/custom")

	if got := MyCliHome(); got != "/custom" {
		t.Fatalf("MyCliHome() = %q, want /custom", got)
	}
}
