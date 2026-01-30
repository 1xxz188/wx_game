package fsm

import "testing"

func BenchmarkTransition(b *testing.B) {
	f := NewFSM(0)
	f.RegisterTransition(0, 1, nil)
	f.RegisterTransition(1, 0, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Transition(int32(1 - f.CurrentState()))
	}
}

func BenchmarkTransition_WithCallback(b *testing.B) {
	f := NewFSM(0)
	f.RegisterTransition(0, 1, func() error { return nil })
	f.RegisterTransition(1, 0, func() error { return nil })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Transition(int32(1 - f.CurrentState()))
	}
}

func BenchmarkTransition_Wildcard(b *testing.B) {
	f := NewFSM(0)
	f.RegisterTransition(StateAny, 1, nil)
	f.RegisterTransition(StateAny, 0, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.Transition(int32(1 - f.CurrentState()))
	}
}

func BenchmarkCurrentState(b *testing.B) {
	f := NewFSM(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.CurrentState()
	}
}

func BenchmarkCanTransition(b *testing.B) {
	f := NewFSM(0)
	f.RegisterTransition(0, 1, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.CanTransition(1)
	}
}

func BenchmarkCanTransition_Wildcard(b *testing.B) {
	f := NewFSM(0)
	f.RegisterTransition(StateAny, 1, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = f.CanTransition(1)
	}
}
