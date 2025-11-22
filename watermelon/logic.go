package watermelon

import (
	"github.com/gofiber/websocket/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/protobuf/proto"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"
)

type Model struct {
	roleMgr     *role.Mgr
	collectsMap cmap.ConcurrentMap[fw.ObjID, *msg.DBWaterMelon]
}

func (s *Model) Init(handler fw.MsgInterface, roleMgr *role.Mgr) {
	s.roleMgr = roleMgr
	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonStart),
		func() proto.Message { return &msg.Ping_Request{} },
		s.Start,
	)
}
func (s *Model) GetOrCreate(roleId fw.ObjID) *msg.DBWaterMelon {
	v, ok := s.collectsMap.Get(roleId)
	if !ok {
		v = &msg.DBWaterMelon{
			RoleId: int64(roleId),
		}
		s.collectsMap.Set(roleId, v)
	}
	return v
}

func (s *Model) Start(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	//req := m.(*msg.WATERMELON_START_Request)
	roleId := s.roleMgr.GetRoleIdOrCreate(ctx.OpenID)
	r := s.GetOrCreate(roleId)
	return &msg.WATERMELON_START_Response{
		Snapshot: r.Snapshot,
	}, nil
}

func (s *Model) Next(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {

	return &msg.WATERMELON_NEXT_Response{}, nil
}
