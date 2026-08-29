package service

import (
	"context"
	"testing"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

type fakeSource struct{}

func (fakeSource) Snapshot() parser.Stats { return parser.Stats{} }
func (fakeSource) LastError() error       { return nil }
func (fakeSource) LastRefresh() time.Time { return time.Time{} }

func TestControllerStartStopLifecycle(t *testing.T) {
	controller := NewController("127.0.0.1:0", fakeSource{})
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	status := controller.Status()
	if status.State != StateRunning || status.Address == "" {
		t.Fatalf("unexpected running status: %+v", status)
	}
	if err := controller.Start(); err != nil {
		t.Fatalf("idempotent start failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if status := controller.Status(); status.State != StateStopped {
		t.Fatalf("unexpected stopped status: %+v", status)
	}
}

func TestControllerReportsBindFailure(t *testing.T) {
	first := NewController("127.0.0.1:0", fakeSource{})
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	defer first.StopWithTimeout()

	second := NewController(first.Status().Address, fakeSource{})
	if err := second.Start(); err == nil {
		t.Fatal("expected address collision to fail")
	}
	if status := second.Status(); status.State != StateStopped || status.LastError == nil {
		t.Fatalf("bind failure was not retained: %+v", status)
	}
}
