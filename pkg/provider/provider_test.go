package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// providerFactories are used to instantiate a provider during acceptance testing.
// The factory function will be invoked for every Terraform CLI command executed
// to create a provider server to which the CLI can reattach.
var ProviderFactories = map[string]func() (*schema.Provider, error){

	"clickhouse": func() (testAccProvider *schema.Provider, e error) {
		return New("dev")(), nil
	},
}

func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

// mockPinger is a mock implementation of the pinger interface for testing
type mockPinger struct {
	pingFunc func(context.Context) error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.pingFunc(ctx)
}

func TestPingWithRetry_ImmediateSuccess(t *testing.T) {
	mock := &mockPinger{
		pingFunc: func(ctx context.Context) error {
			return nil
		},
	}

	err := pingWithRetry(context.Background(), mock, 3, minRetryDelay)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPingWithRetry_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	mock := &mockPinger{
		pingFunc: func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("connection refused")
			}
			return nil
		},
	}

	err := pingWithRetry(context.Background(), mock, 5, minRetryDelay)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got: %d", attempts)
	}
}

func TestPingWithRetry_FailureAfterMaxRetries(t *testing.T) {
	attempts := 0
	expectedErr := errors.New("i/o timeout")
	mock := &mockPinger{
		pingFunc: func(ctx context.Context) error {
			attempts++
			return expectedErr
		},
	}

	err := pingWithRetry(context.Background(), mock, 2, minRetryDelay)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// Should attempt initial + 2 retries = 3 total
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (initial + 2 retries), got: %d", attempts)
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected wrapped timeout error, got: %v", err)
	}
}
