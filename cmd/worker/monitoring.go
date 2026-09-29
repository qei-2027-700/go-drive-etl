package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/getsentry/sentry-go"
)

// buildRelease can be set at build time with:
// -ldflags "-X main.buildRelease=<git-commit-sha>"
var buildRelease string

type runMetadata struct {
	environment string
	release     string
	loadID      string
}

func newRunMetadata() runMetadata {
	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "local"
	}

	release := os.Getenv("SENTRY_RELEASE")
	if release == "" {
		release = buildRelease
	}
	if release == "" {
		release = "unknown"
	}

	return runMetadata{
		environment: environment,
		release:     release,
		loadID:      newLoadID(),
	}
}

func newLoadID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failures are exceptionally rare. A deterministic fallback
		// still keeps the value non-sensitive and makes the failed run traceable.
		return "unavailable"
	}
	return hex.EncodeToString(bytes)
}

func sentryOptions(metadata runMetadata) sentry.ClientOptions {
	return sentry.ClientOptions{
		Dsn:              os.Getenv("SENTRY_DSN"),
		Environment:      metadata.environment,
		Release:          metadata.release,
		SendDefaultPII:   false,
		MaxBreadcrumbs:   0,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			// This worker must never deliberately send document contents, credentials,
			// or identifiers. Clear SDK-populated fields as a second line of defence.
			event.User = sentry.User{}
			event.Request = nil
			event.Breadcrumbs = nil
			return event
		},
	}
}

func captureFailure(err error, metadata runMetadata) {
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("stage", stageFromError(err))
		scope.SetTag("environment", metadata.environment)
		scope.SetTag("release", metadata.release)
		scope.SetTag("load_id", metadata.loadID)
		sentry.CaptureException(err)
	})
}

type stageError struct {
	stage string
	err   error
}

func (e *stageError) Error() string { return e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

func withStage(stage string, err error) error {
	return &stageError{stage: stage, err: err}
}

func stageFromError(err error) string {
	var stageErr *stageError
	if errors.As(err, &stageErr) && stageErr.stage != "" {
		return stageErr.stage
	}
	return "startup"
}

func recoveredPanicError(recovered any) error {
	return withStage("startup", fmt.Errorf("予期しない panic: %v", recovered))
}
