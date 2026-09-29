package main

import (
	"errors"
	"testing"
)

func TestSentryOptionsUseEnvironment(t *testing.T) {
	t.Setenv("SENTRY_DSN", "https://public@example.ingest.sentry.io/1")
	t.Setenv("SENTRY_ENVIRONMENT", "test")
	t.Setenv("SENTRY_RELEASE", "abc123")

	metadata := newRunMetadata()
	opts := sentryOptions(metadata)
	if got, want := opts.Dsn, "https://public@example.ingest.sentry.io/1"; got != want {
		t.Errorf("DSN = %q, want %q", got, want)
	}
	if got, want := opts.Environment, "test"; got != want {
		t.Errorf("environment = %q, want %q", got, want)
	}
	if got, want := opts.Release, "abc123"; got != want {
		t.Errorf("release = %q, want %q", got, want)
	}
	if opts.SendDefaultPII || opts.MaxBreadcrumbs != 0 {
		t.Error("Sentry options must not collect PII or breadcrumbs")
	}
}

func TestRunMetadataDefaultsAndStage(t *testing.T) {
	t.Setenv("SENTRY_ENVIRONMENT", "")
	t.Setenv("SENTRY_RELEASE", "")
	oldBuildRelease := buildRelease
	buildRelease = "build-sha"
	t.Cleanup(func() { buildRelease = oldBuildRelease })

	metadata := newRunMetadata()
	if metadata.environment != "local" || metadata.release != "build-sha" || len(metadata.loadID) != 32 {
		t.Errorf("unexpected metadata: %#v", metadata)
	}
	if got := stageFromError(withStage("download", errors.New("unavailable"))); got != "download" {
		t.Errorf("stage = %q, want download", got)
	}
	if got := stageFromError(errors.New("unavailable")); got != "startup" {
		t.Errorf("default stage = %q, want startup", got)
	}
}
