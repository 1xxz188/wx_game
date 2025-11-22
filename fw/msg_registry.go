package fw

import "google.golang.org/protobuf/proto"

// MessageRegistry 消息注册表（合并后的单一 map）
type MessageRegistry struct {
	// msgID -> 消息信息（包含工厂函数和处理函数）
	messages map[MessageID]*MessageInfo
}

// NewMessageRegistry 创建消息注册表
func NewMessageRegistry() *MessageRegistry {
	return &MessageRegistry{
		messages: make(map[MessageID]*MessageInfo),
	}
}

// Register 注册消息类型和处理函数
// msgID: 消息 ID（4 字节）
// factory: 创建空消息实例的函数，例如：func() proto.Message { return &msg.AuthRequest{} }
// handler: 消息处理函数
func (r *MessageRegistry) Register(msgID MessageID, factory func() proto.Message, handler MessageHandler) {
	r.messages[msgID] = &MessageInfo{
		Factory: factory,
		Handler: handler,
	}
}

// GetFactory 获取消息类型工厂函数
func (r *MessageRegistry) GetFactory(msgID MessageID) (func() proto.Message, bool) {
	info, ok := r.messages[msgID]
	if !ok {
		return nil, false
	}
	return info.Factory, true
}

// GetHandler 获取消息处理函数
func (r *MessageRegistry) GetHandler(msgID MessageID) (MessageHandler, bool) {
	info, ok := r.messages[msgID]
	if !ok {
		return nil, false
	}
	return info.Handler, true
}

// Get 获取完整的消息信息
func (r *MessageRegistry) Get(msgID MessageID) (*MessageInfo, bool) {
	info, ok := r.messages[msgID]
	return info, ok
}
