package antenne

import (
	"errors"
	"testing"
)

func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, name := range []string{"ANTENNE_URL", "ANTENNE_INSTANCE", "ANTENNE_APP", "ANTENNE_SECRET", "ANTENNE_INSTANCE_ID"} {
		t.Setenv(name, values[name])
	}
}

func TestConfigFromEnv(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		instance string
		app      string
		err      error
	}{
		{"url", map[string]string{"ANTENNE_URL": "https://a.example/", "ANTENNE_SECRET": "s"}, "https://a.example", "Sonde", nil},
		{"instance synonym", map[string]string{"ANTENNE_INSTANCE": "https://b.example", "ANTENNE_SECRET": "s"}, "https://b.example", "Sonde", nil},
		{"url wins", map[string]string{"ANTENNE_URL": "https://a.example", "ANTENNE_INSTANCE": "https://b.example", "ANTENNE_SECRET": "s"}, "https://a.example", "Sonde", nil},
		{"app override", map[string]string{"ANTENNE_URL": "https://a.example", "ANTENNE_SECRET": "s", "ANTENNE_APP": "sonde-eu"}, "https://a.example", "sonde-eu", nil},
		{"unset", map[string]string{}, "", "", ErrNotConfigured},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, tc.env)
			cfg, err := ConfigFromEnv("Sonde")
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if err == nil && (cfg.Instance != tc.instance || cfg.App != tc.app || cfg.Secret != "s") {
				t.Fatalf("cfg = %+v", cfg)
			}
		})
	}
}

func TestConfigFromEnvRequiresSecret(t *testing.T) {
	setEnv(t, map[string]string{"ANTENNE_URL": "https://a.example"})
	if _, err := ConfigFromEnv("Sonde"); err == nil || errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing secret gave %v", err)
	}
}
