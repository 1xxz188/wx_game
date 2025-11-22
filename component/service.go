package component

import "reflect"

func GetServiceName(comp Component, opts []Option) string {
	op := &options{}
	for i := range opts {
		opt := opts[i]
		opt(op)
	}
	if op.name != "" {
		return op.name
	} else {
		return reflect.Indirect(reflect.ValueOf(comp)).Type().Name()
	}
}
