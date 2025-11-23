package fw

import (
	"bytes"
	"encoding/gob"
	"reflect"
)

// DeepCopyInterface 通过 gob 序列化/反序列化实现对任意 interface{} 的深拷贝
// 如果 data 里含有不可导出的字段、chan、func 等会失败
func DeepCopyInterface(src interface{}) (interface{}, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return nil, err
	}
	// 需要让 gob 知道具体类型，否则 Decode 时会变成 map[string]interface{}
	// 这里用反射拿到原始类型，再 new 一个指针，最后 elem 得到值
	rt := reflect.TypeOf(src)
	ptr := reflect.New(rt)
	if err := gob.NewDecoder(&buf).Decode(ptr.Interface()); err != nil {
		return nil, err
	}
	return ptr.Elem().Interface(), nil
}
