package main

import (
	"testing"
)

func TestSentryOptionsUseEnvironment(t *testing.T) {
	t.Setenv("SENTRY_DSN", "https://public@example.ingest.sentry.io/1")
	t.Setenv("SENTRY_ENVIRONMENT", "test")

	opts := sentryOptions()
	if got, want := opts.Dsn, "https://public@example.ingest.sentry.io/1"; got != want {
		t.Errorf("DSN = %q, want %q", got, want)
	}
	if got, want := opts.Environment, "test"; got != want {
		t.Errorf("environment = %q, want %q", got, want)
	}
}
