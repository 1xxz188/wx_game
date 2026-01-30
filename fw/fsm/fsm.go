// Package fsm 提供一个简单的有限状态机实现。
//
// FSM 使用 int32 作为状态类型，支持状态转换回调和通配符转换。
// 本实现假设运行环境保证协程安全，因此不使用任何锁或原子操作。
//
// 基本用法：
//
//	const (
//	    StateIdle int32 = iota
//	    StateRunning
//	    StateStopped
//	)
//
//	f := fsm.NewFSM(StateIdle)
//	f.RegisterTransition(StateIdle, StateRunning, func() error {
//	    log.Println("starting...")
//	    return nil
//	})
//	f.RegisterTransition(StateRunning, StateStopped, func() error {
//	    log.Println("stopping...")
//	    return nil
//	})
//
//	if err := f.Transition(StateRunning); err != nil {
//	    log.Fatal(err)
//	}
//
// 通配符用法：
//
//	// 从任意状态都可以转换到 StateStopped
//	f.RegisterTransition(fsm.StateAny, StateStopped, func() error {
//	    log.Println("emergency stop!")
//	    return nil
//	})
package fsm

import (
	"errors"
	"fmt"
)

// ErrTransitionExists 表示转换已注册。
var ErrTransitionExists = errors.New("transition already registered")

// StateAny 表示通配符状态，匹配任意源状态。
// 当注册转换时，from 参数使用 StateAny 表示从任意状态都可以转换到目标状态。
// 转换时优先匹配精确注册，找不到时才使用通配符。
const StateAny int32 = -1

// FSM 有限状态机。
// 注意：本实现不是线程安全的，调用者需要保证协程安全。
type FSM struct {
	state     int32
	callbacks map[int32]map[int32]func() error
}

// NewFSM 创建一个新的有限状态机。
// initialState 为初始状态，可以是任意 int32 值（除了 StateAny）。
func NewFSM(initialState int32) *FSM {
	return &FSM{
		state:     initialState,
		callbacks: make(map[int32]map[int32]func() error),
	}
}

// RegisterTransition 注册状态转换回调。
// from 为源状态，可以使用 StateAny 表示从任意状态转换。
// to 为目标状态。
// callback 为转换时执行的回调函数，可以为 nil（表示允许转换但不执行任何操作）。
// 如果 callback 返回错误，转换将被中止，状态保持不变。
// 重复注册相同的 from→to 转换返回 ErrTransitionExists。
//
// 注意：应在初始化阶段调用此方法，运行时调用需要调用者保证协程安全。
func (f *FSM) RegisterTransition(from, to int32, callback func() error) error {
	if f.callbacks[from] == nil {
		f.callbacks[from] = make(map[int32]func() error)
	}
	if _, exists := f.callbacks[from][to]; exists {
		return ErrTransitionExists
	}
	f.callbacks[from][to] = callback
	return nil
}

// Transition 执行状态转换到目标状态。
// 转换规则：
//  1. 优先查找精确匹配：当前状态 → 目标状态
//  2. 找不到则查找通配符：StateAny → 目标状态
//  3. 都找不到返回错误
//
// 如果找到的回调返回错误，状态保持不变，错误返回给调用者。
// 如果回调为 nil 或返回 nil，状态更新为目标状态。
// 支持同状态转换（当前状态 == 目标状态）。
func (f *FSM) Transition(to int32) error {
	from := f.state

	callback, ok := f.findCallback(from, to)
	if !ok {
		callback, ok = f.findCallback(StateAny, to)
	}

	if !ok {
		return fmt.Errorf("invalid transition from state %d to state %d", from, to)
	}

	if callback != nil {
		if err := callback(); err != nil {
			return err
		}
	}

	f.state = to
	return nil
}

// findCallback 查找指定转换的回调函数。
func (f *FSM) findCallback(from, to int32) (func() error, bool) {
	toMap, ok := f.callbacks[from]
	if !ok {
		return nil, false
	}
	callback, ok := toMap[to]
	return callback, ok
}

// CurrentState 返回当前状态。
func (f *FSM) CurrentState() int32 {
	return f.state
}

// CanTransition 检查是否可以转换到目标状态。
// 检查精确匹配或通配符匹配是否存在。
func (f *FSM) CanTransition(to int32) bool {
	from := f.state

	if _, ok := f.findCallback(from, to); ok {
		return true
	}

	if _, ok := f.findCallback(StateAny, to); ok {
		return true
	}

	return false
}
