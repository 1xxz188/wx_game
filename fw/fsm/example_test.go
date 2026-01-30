package fsm_test

import (
	"errors"
	"fmt"

	"wx_game/fw/fsm"
)

const (
	StateIdle int32 = iota
	StateRunning
	StateStopped
)

func ExampleFSM_basic() {
	f := fsm.NewFSM(StateIdle)

	_ = f.RegisterTransition(StateIdle, StateRunning, func() error {
		fmt.Println("starting")
		return nil
	})
	_ = f.RegisterTransition(StateRunning, StateStopped, func() error {
		fmt.Println("stopping")
		return nil
	})

	fmt.Printf("initial: %d\n", f.CurrentState())

	_ = f.Transition(StateRunning)
	fmt.Printf("after start: %d\n", f.CurrentState())

	_ = f.Transition(StateStopped)
	fmt.Printf("after stop: %d\n", f.CurrentState())

	// Output:
	// initial: 0
	// starting
	// after start: 1
	// stopping
	// after stop: 2
}

func ExampleFSM_wildcard() {
	f := fsm.NewFSM(StateRunning)

	_ = f.RegisterTransition(fsm.StateAny, StateStopped, func() error {
		fmt.Println("emergency stop from any state")
		return nil
	})

	fmt.Printf("current: %d\n", f.CurrentState())
	_ = f.Transition(StateStopped)
	fmt.Printf("after emergency stop: %d\n", f.CurrentState())

	// Output:
	// current: 1
	// emergency stop from any state
	// after emergency stop: 2
}

func ExampleFSM_callbackError() {
	f := fsm.NewFSM(StateIdle)

	_ = f.RegisterTransition(StateIdle, StateRunning, func() error {
		return errors.New("cannot start: resource unavailable")
	})

	fmt.Printf("before: %d\n", f.CurrentState())
	err := f.Transition(StateRunning)
	fmt.Printf("error: %v\n", err)
	fmt.Printf("after: %d\n", f.CurrentState())

	// Output:
	// before: 0
	// error: cannot start: resource unavailable
	// after: 0
}
