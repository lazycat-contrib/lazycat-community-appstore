package clientcmd

import (
	"context"
	"errors"
	"testing"
)

func TestRunValidatesBeforeOpeningResources(t *testing.T) {
	t.Setenv("CLIENT_ADDR", "0.0.0.0:8090")
	t.Setenv("CLIENT_SESSION_SECRET", "")
	err := Run(t.Context())
	if err == nil || err.Error() != "validate client config: CLIENT_SESSION_SECRET must be changed for non-loopback deployments" {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestExecuteReturnsFailureWhenRunFails(t *testing.T) {
	errSentinel := errors.New("sentinel")
	previous := run
	run = func(context.Context) error { return errSentinel }
	t.Cleanup(func() { run = previous })
	if code := Execute(); code != 1 {
		t.Fatalf("Execute() = %d, want 1", code)
	}
}
