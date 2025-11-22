package watermelon

import (
	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/websocket/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/protobuf/proto"
	"math/rand/v2"
	"strconv"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"
)

type Model struct {
	roleMgr     *role.Mgr
	collectsMap cmap.ConcurrentMap[string, *msg.DBWaterMelon]

	//配置表缓存
	LvlToWeight map[int32]int32
	cfgWeight   []cfgWeight
}

type cfgWeight struct {
	totalWeight int32
	lvl         int32
}

func New() *Model {
	return &Model{
		collectsMap: cmap.New[*msg.DBWaterMelon](),
		LvlToWeight: make(map[int32]int32),
		cfgWeight:   make([]cfgWeight, 0),
	}
}

func (s *Model) Init(handler fw.MsgInterface, roleMgr *role.Mgr) {
	s.roleMgr = roleMgr
	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonStart),
		func() proto.Message { return &msg.Ping_Request{} },
		s.Start,
	)

	totalWeight := int32(0)
	for _, v := range cfg.Tables().TbWaterMelonLevel.GetDataList() {
		if v.Weight > 0 {
			totalWeight += v.Weight
			s.cfgWeight = append(s.cfgWeight, cfgWeight{
				totalWeight: totalWeight,
				lvl:         v.Id,
			})
		}
		s.LvlToWeight[v.Id] = totalWeight
	}
}
func (s *Model) GetOrCreate(roleId fw.ObjID) *msg.DBWaterMelon {
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := s.collectsMap.Get(sId)
	if !ok {
		v = &msg.DBWaterMelon{
			RoleId: int64(roleId),
		}
		s.collectsMap.Set(sId, v)
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
	roleId := s.roleMgr.GetRoleIdOrCreate(ctx.OpenID)
	r := s.GetOrCreate(roleId)
	s.makeNextList(r)
	return &msg.WATERMELON_NEXT_Response{
		EntityLst: r.NextLst,
	}, nil
}

func (s *Model) makeNextList(r *msg.DBWaterMelon) msg.ErrorCode {
	const cfgDefaultId = 1
	config := cfg.Tables().TbWaterMelonConfig.Get(cfgDefaultId)
	if config == nil {
		logger.Errorf("cant find TbWaterMelonConfig cfgDefaultId[%d]", cfgDefaultId)
		return msg.ErrorCode_E_ErrorCode_Activity_WaterMelon_Cfg
	}
	if len(r.NextLst) >= int(config.NextMaxCnt) {
		return 0
	}
	cnt := int(config.NextMaxCnt) - len(r.NextLst)
	if cnt == 0 {
		for _, lvl := range config.InitFruit {
			r.AutoIncrId++
			autoId := r.AutoIncrId
			r.NextLst = append(r.NextLst, &msg.WaterMelonEntity{
				Id:    autoId,
				Level: lvl,
			})
		}
		return 0
	}

	if r.InsideGameMaxLv <= 0 {
		r.InsideGameMaxLv = 1
	}

	weight, ok := s.LvlToWeight[r.InsideGameMaxLv]
	if !ok {
		logger.Errorf("cant find InsideGameMaxLv[%d]", r.InsideGameMaxLv)
		return msg.ErrorCode_E_ErrorCode_Activity_WaterMelon_Cfg
	}

	for cnt > 0 {
		cnt--
		num := rand.Int32N(weight)
		lvl := int32(0)
		for _, v := range s.cfgWeight {
			if num < v.totalWeight {
				lvl = v.lvl
				break
			}
		}
		if lvl < 0 {
			return msg.ErrorCode_E_ErrorCode_Activity_WaterMelon_Logic
		}

		r.AutoIncrId++
		autoId := r.AutoIncrId
		r.NextLst = append(r.NextLst, &msg.WaterMelonEntity{
			Id:    autoId,
			Level: lvl,
		})
	}
	return 0
}
