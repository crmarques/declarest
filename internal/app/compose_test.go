package app

import "testing"

func TestNewContainer(t *testing.T) {
	t.Parallel()

	container := NewContainer()
	if container.Contexts == nil {
		t.Fatal("expected non-nil context manager")
	}
	if container.Reconciler == nil {
		t.Fatal("expected non-nil reconciler")
	}

	wiring := container.CommandWiring()
	if wiring.Contexts == nil {
		t.Fatal("expected non-nil contexts wiring")
	}
	if wiring.Reconciler == nil {
		t.Fatal("expected non-nil reconciler wiring")
	}
}
