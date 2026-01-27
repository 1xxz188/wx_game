package fsm

import (
	"errors"
	"testing"
)

const (
	stateIdle int32 = iota
	stateRunning
	stateStopped
	statePaused
)

func TestNewFSM(t *testing.T) {
	f := NewFSM(stateIdle)
	if f.CurrentState() != stateIdle {
		t.Errorf("expected initial state %d, got %d", stateIdle, f.CurrentState())
	}
}

func TestRegisterTransition(t *testing.T) {
	f := NewFSM(stateIdle)
	called := false
	if err := f.RegisterTransition(stateIdle, stateRunning, func() error {
		called = true
		return nil
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !f.CanTransition(stateRunning) {
		t.Error("expected transition to be registered")
	}

	if err := f.Transition(stateRunning); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("callback was not called")
	}
}

func TestRegisterTransition_Wildcard(t *testing.T) {
	f := NewFSM(stateIdle)
	called := false
	if err := f.RegisterTransition(StateAny, stateStopped, func() error {
		called = true
		return nil
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !f.CanTransition(stateStopped) {
		t.Error("expected wildcard transition to be registered")
	}

	if err := f.Transition(stateStopped); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("wildcard callback was not called")
	}
}

func TestRegisterTransition_Duplicate(t *testing.T) {
	f := NewFSM(stateIdle)

	if err := f.RegisterTransition(stateIdle, stateRunning, nil); err != nil {
		t.Errorf("first registration should succeed: %v", err)
	}

	err := f.RegisterTransition(stateIdle, stateRunning, nil)
	if !errors.Is(err, ErrTransitionExists) {
		t.Errorf("expected ErrTransitionExists, got %v", err)
	}
}

func TestTransition_Success(t *testing.T) {
	f := NewFSM(stateIdle)
	callOrder := []int32{}

	_ = f.RegisterTransition(stateIdle, stateRunning, func() error {
		callOrder = append(callOrder, stateRunning)
		return nil
	})
	_ = f.RegisterTransition(stateRunning, stateStopped, func() error {
		callOrder = append(callOrder, stateStopped)
		return nil
	})

	if err := f.Transition(stateRunning); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if f.CurrentState() != stateRunning {
		t.Errorf("expected state %d, got %d", stateRunning, f.CurrentState())
	}

	if err := f.Transition(stateStopped); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if f.CurrentState() != stateStopped {
		t.Errorf("expected state %d, got %d", stateStopped, f.CurrentState())
	}

	if len(callOrder) != 2 || callOrder[0] != stateRunning || callOrder[1] != stateStopped {
		t.Errorf("unexpected call order: %v", callOrder)
	}
}

func TestTransition_InvalidTransition(t *testing.T) {
	f := NewFSM(stateIdle)
	_ = f.RegisterTransition(stateIdle, stateRunning, nil)

	err := f.Transition(stateStopped)
	if err == nil {
		t.Error("expected error for invalid transition")
	}

	if f.CurrentState() != stateIdle {
		t.Errorf("state should not change on invalid transition, got %d", f.CurrentState())
	}
}

func TestTransition_CallbackError(t *testing.T) {
	f := NewFSM(stateIdle)
	expectedErr := errors.New("callback failed")

	_ = f.RegisterTransition(stateIdle, stateRunning, func() error {
		return expectedErr
	})

	err := f.Transition(stateRunning)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if f.CurrentState() != stateIdle {
		t.Errorf("state should not change on callback error, got %d", f.CurrentState())
	}
}

func TestTransition_SameState(t *testing.T) {
	f := NewFSM(stateIdle)
	callCount := 0

	_ = f.RegisterTransition(stateIdle, stateIdle, func() error {
		callCount++
		return nil
	})

	if err := f.Transition(stateIdle); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected callback to be called once, got %d", callCount)
	}

	if f.CurrentState() != stateIdle {
		t.Errorf("expected state %d, got %d", stateIdle, f.CurrentState())
	}
}

func TestTransition_Wildcard(t *testing.T) {
	f := NewFSM(stateRunning)
	called := false

	_ = f.RegisterTransition(StateAny, stateStopped, func() error {
		called = true
		return nil
	})

	if err := f.Transition(stateStopped); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !called {
		t.Error("wildcard callback was not called")
	}

	if f.CurrentState() != stateStopped {
		t.Errorf("expected state %d, got %d", stateStopped, f.CurrentState())
	}
}

func TestTransition_ExactPriority(t *testing.T) {
	f := NewFSM(stateIdle)
	exactCalled := false
	wildcardCalled := false

	_ = f.RegisterTransition(stateIdle, stateRunning, func() error {
		exactCalled = true
		return nil
	})
	_ = f.RegisterTransition(StateAny, stateRunning, func() error {
		wildcardCalled = true
		return nil
	})

	if err := f.Transition(stateRunning); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !exactCalled {
		t.Error("exact callback should be called")
	}
	if wildcardCalled {
		t.Error("wildcard callback should NOT be called when exact match exists")
	}
}

func TestTransition_NilCallback(t *testing.T) {
	f := NewFSM(stateIdle)
	_ = f.RegisterTransition(stateIdle, stateRunning, nil)

	if err := f.Transition(stateRunning); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if f.CurrentState() != stateRunning {
		t.Errorf("expected state %d, got %d", stateRunning, f.CurrentState())
	}
}

func TestCurrentState(t *testing.T) {
	f := NewFSM(stateIdle)
	if f.CurrentState() != stateIdle {
		t.Errorf("expected state %d, got %d", stateIdle, f.CurrentState())
	}

	_ = f.RegisterTransition(stateIdle, stateRunning, nil)
	_ = f.Transition(stateRunning)

	if f.CurrentState() != stateRunning {
		t.Errorf("expected state %d, got %d", stateRunning, f.CurrentState())
	}
}

func TestCanTransition(t *testing.T) {
	f := NewFSM(stateIdle)
	_ = f.RegisterTransition(stateIdle, stateRunning, nil)

	if !f.CanTransition(stateRunning) {
		t.Error("expected CanTransition to return true for registered transition")
	}

	if f.CanTransition(stateStopped) {
		t.Error("expected CanTransition to return false for unregistered transition")
	}
}

func TestCanTransition_Wildcard(t *testing.T) {
	f := NewFSM(stateIdle)
	_ = f.RegisterTransition(StateAny, stateStopped, nil)

	if !f.CanTransition(stateStopped) {
		t.Error("expected CanTransition to return true for wildcard transition")
	}

	_ = f.Transition(stateStopped)
	_ = f.RegisterTransition(stateStopped, stateIdle, nil)
	_ = f.Transition(stateIdle)

	if !f.CanTransition(stateStopped) {
		t.Error("expected CanTransition to return true for wildcard from different state")
	}
}

func TestTransition_WildcardFallback(t *testing.T) {
	f := NewFSM(stateIdle)

	_ = f.RegisterTransition(stateRunning, stateStopped, func() error {
		t.Error("exact callback should not be called")
		return nil
	})
	_ = f.RegisterTransition(StateAny, stateStopped, nil)

	if err := f.Transition(stateStopped); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if f.CurrentState() != stateStopped {
		t.Errorf("expected state %d, got %d", stateStopped, f.CurrentState())
	}
}
