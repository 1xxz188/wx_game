package watermelon

import (
	"math/rand/v2"
	"strconv"
	"wx_game/cfg"
	cfgCode "wx_game/cfg/code"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/websocket/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/protobuf/proto"
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
		func() proto.Message { return &msg.WATERMELON_START_Request{} },
		s.Start,
	)

	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonFall),
		func() proto.Message { return &msg.WATERMELON_FALL_Request{} },
		s.Fall,
	)

	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonSync),
		func() proto.Message { return &msg.WATERMELON_SYNC_Request{} },
		s.Sync,
	)

	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonEnd),
		func() proto.Message { return &msg.WATERMELON_END_Request{} },
		s.End,
	)

	handler.Register(fw.MessageID(msg.WatermelonMsgWatermelonUseItem),
		func() proto.Message { return &msg.WATERMELON_USE_ITEM_Request{} },
		s.UseItem,
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
		s.collectsMap.SetIfAbsent(sId, v)
		v, _ = s.collectsMap.Get(sId)
	}
	return v
}

func (s *Model) Start(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	var dataSnapshot interface{}
	var dataNext interface{}
	var dataItemCount interface{}
	var err error
	resp := &msg.WATERMELON_START_Response{}

	s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Info) {
		if r.Watermelon.Snapshot == nil {
			r.Watermelon.Snapshot = &msg.WaterMelonRecordSnapshot{}
		}

		dataSnapshot, err = fw.DeepCopyInterface(r.Watermelon.Snapshot)
		if err != nil {
			logger.Error(err)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}

		code := s.makeNextList(r.Watermelon)
		if code != 0 {
			logger.Errorf("open_id[%s] Start makeNextList code[%d]", ctx.OpenID, code)
			resp.ErrorCode = int32(code)
			return
		}

		resp.HistoryScore = r.Watermelon.HistoryScore
		resp.Score = r.Watermelon.Score
		dataNext, err = fw.DeepCopyInterface(r.Watermelon.NextLst)
		if err != nil {
			logger.Errorf("err[%s] data[%+v]", err, r.Watermelon.NextLst)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}

		if len(r.Watermelon.Snapshot.Records) == 0 {
			//初始化道具
			r.Watermelon.MapInsideItemCount[cfgCode.EItem_WatermelonErase] = 1
			r.Watermelon.MapInsideItemCount[cfgCode.EItem_WatermelonSwap] = 1
			r.Watermelon.MapInsideItemCount[cfgCode.EItem_WatermelonUpgrade] = 1
		}
		dataItemCount, err = fw.DeepCopyInterface(r.Watermelon.MapInsideItemCount)
		if err != nil {
			logger.Errorf("err[%s] data[%+v]", err, r.Watermelon.NextLst)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}
	})

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.Snapshot = dataSnapshot.(*msg.WaterMelonRecordSnapshot)
	resp.EntityLst = dataNext.([]*msg.WaterMelonEntity)
	resp.MapItemCount = dataItemCount.(map[int32]int32)
	logger.Debugf("open_id[%s] start records[%d] next_list[%v]", ctx.OpenID, len(resp.Snapshot.Records), resp.EntityLst)
	return resp, nil
}

func (s *Model) Fall(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WATERMELON_FALL_Request)
	resp := &msg.WATERMELON_FALL_Response{}

	var dataNext interface{}
	var err error
	s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Info) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if len(r.Watermelon.NextLst) <= 0 {
			logger.Debugf("len(r.Watermelon.NextLst) <= 0")
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		if r.Watermelon.NextLst[0].Id != req.WaterMelonId {
			logger.Debugf("open_id[%s] r.Watermelon.NextLst[0].Id[%d] != req.WaterMelonId[%d]", ctx.OpenID, r.Watermelon.NextLst[0].Id, req.WaterMelonId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		for _, record := range req.Snapshot.Records {
			if record.Id > req.WaterMelonId {
				logger.Debugf("open_id[%s] record.Id[%d] req.WaterMelonId[%d]", ctx.OpenID, record.Id, req.WaterMelonId)
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}
		}

		r.Watermelon.NextLst = r.Watermelon.NextLst[1:]
		r.Watermelon.Snapshot.Records = req.Snapshot.Records
		code := s.makeNextList(r.Watermelon)
		if code != 0 {
			logger.Errorf("open_id[%s] Fall makeNextList code[%d]", ctx.OpenID, code)
			resp.ErrorCode = int32(code)
			return
		}
		dataNext, err = fw.DeepCopyInterface(r.Watermelon.NextLst)
		if err != nil {
			logger.Errorf("err[%s] data[%+v]", err, r.Watermelon.NextLst)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}
	})

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.EntityLst = dataNext.([]*msg.WaterMelonEntity)
	logger.Debugf("open_id[%s] fall id[%d]", ctx.OpenID, req.WaterMelonId)
	return resp, nil
}

func (s *Model) Sync(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WATERMELON_SYNC_Request)
	resp := &msg.WATERMELON_SYNC_Response{}

	s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Info) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		if len(req.MergeLst) > 0 {
			if len(req.Snapshot.Records)+1 == len(r.Watermelon.Snapshot.Records) {
				mapSaveWaterMelonLevel := make(map[int32]int32, len(r.Watermelon.Snapshot.Records))
				mapMergeLevelCount := make(map[int32]int32)

				for _, record := range r.Watermelon.Snapshot.Records {
					mapSaveWaterMelonLevel[record.Id] = record.Level
				}

				for _, detail := range req.MergeLst {
					_, ok := mapSaveWaterMelonLevel[detail.FromId]
					if !ok {
						logger.Debugf("not find mapSaveWaterMelonLevel FromId[%d]", detail.FromId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					_, ok = mapSaveWaterMelonLevel[detail.ToId]
					if !ok {
						logger.Debugf("not find mapSaveWaterMelonLevel ToId[%d]", detail.ToId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					if detail.FromId >= detail.ToId {
						logger.Debugf("detail.FromId[%d] >= detail.ToId[%d]", detail.FromId, detail.ToId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					//必须相同等级
					if mapSaveWaterMelonLevel[detail.FromId] != mapSaveWaterMelonLevel[detail.ToId] {
						logger.Debugf("detail.FromId == detail.ToId || mapSaveWaterMelonLevel[detail.FromId] != mapSaveWaterMelonLevel[detail.ToId]")
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}

					delete(mapSaveWaterMelonLevel, detail.FromId)
					mapSaveWaterMelonLevel[detail.ToId]++ // 目标等级加1
					mapMergeLevelCount[mapSaveWaterMelonLevel[detail.ToId]]++
				}

				maxLvl := int32(0)
				addScore := int32(0)
				for lvl, cnt := range mapMergeLevelCount {
					config := cfg.Tables().TbWaterMelonLevel.Get(lvl)
					if config == nil {
						logger.Errorf("cant find lvl[%d]", lvl)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Cfg)
						return
					}
					if lvl > maxLvl {
						maxLvl = lvl
					}
					addScore += config.Point * cnt
				}

				for lvl, cnt := range mapMergeLevelCount {
					r.Watermelon.MapMergeRecord[lvl] += cnt
					r.Watermelon.MapMergeInsideRecord[lvl] += cnt
				}

				if maxLvl > r.Watermelon.InsideGameMaxLv {
					r.Watermelon.InsideGameMaxLv = maxLvl
				}

				r.Watermelon.Score += addScore
			} else {
				logger.Debugf("open_id[%s] req_records_len[%d] + 1 != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(r.Watermelon.Snapshot.Records))
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}
		} else if len(req.Snapshot.Records) != len(r.Watermelon.Snapshot.Records) { //只做位置同步
			logger.Debugf("open_id[%s] req_records_len[%d] != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(r.Watermelon.Snapshot.Records))
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		r.Watermelon.Snapshot.Records = req.Snapshot.Records
		resp.Score = r.Watermelon.Score
	})

	logger.Debugf("open_id[%s] merge [%v]", ctx.OpenID, req.MergeLst)
	return resp, nil
}

func (s *Model) End(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	resp := &msg.WATERMELON_END_Response{}

	s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Info) {
		if r.Watermelon.Snapshot != nil {
			if r.Watermelon.Score > r.Watermelon.HistoryScore {
				r.Watermelon.HistoryScore = r.Watermelon.Score
			}
			r.Watermelon.Snapshot.Reset()
		}

		clear(r.Watermelon.NextLst)
		r.Watermelon.NextLst = r.Watermelon.NextLst[:0]
		clear(r.Watermelon.MapMergeInsideRecord)
		clear(r.Watermelon.MapInsideItemCount)
		r.Watermelon.InsideGameMaxLv = 0
		r.Watermelon.AutoIncrId = 0
		r.Watermelon.Score = 0
	})
	logger.Debugf("open_id[%s] end", ctx.OpenID)
	return resp, nil
}

func (s *Model) UseItem(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WATERMELON_USE_ITEM_Request)
	resp := &msg.WATERMELON_USE_ITEM_Response{}

	if req.ItemNum != 1 {
		logger.Debugf("open_id[%s] req.ItemNum[%d] != 1", ctx.OpenID, req.ItemNum)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
		return resp, nil
	}

	s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Info) {
		if r.Watermelon.Snapshot != nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if len(r.Watermelon.Snapshot.Records) <= 0 {
			logger.Debugf("open_id[%s] UseItem len(r.Watermelon.Snapshot.Records) <= 0", ctx.OpenID)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		if len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records) {
			logger.Debugf("open_id[%s] len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records)", ctx.OpenID)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		switch req.ItemId {
		case cfgCode.EItem_WatermelonErase:
			fallthrough
		case cfgCode.EItem_WatermelonSwap:
			fallthrough
		case cfgCode.EItem_WatermelonUpgrade:
			if r.Watermelon.MapInsideItemCount[req.ItemId] < req.ItemNum {
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_UseItemCountLimit)
				return
			}
			r.Watermelon.MapInsideItemCount[req.ItemId] -= req.ItemNum
			resp.ItemNum = r.Watermelon.MapInsideItemCount[req.ItemId]
		default:
			logger.Debugf("open_id[%s] unknown req.ItemId[%d]", ctx.OpenID, req.ItemId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		r.Watermelon.Snapshot = req.Snapshot
		//_ = cfgcode.CommonItemId_WatermelonMagnet
	})
	resp.ItemId = req.ItemId
	logger.Debugf("open_id[%s] end", ctx.OpenID)
	return resp, nil
}

func (s *Model) makeNextList(r *msg.DBWaterMelon) int32 {
	const cfgDefaultId = 1
	config := cfg.Tables().TbWaterMelonConfig.Get(cfgDefaultId)
	if config == nil {
		logger.Errorf("cant find TbWaterMelonConfig cfgDefaultId[%d]", cfgDefaultId)
		return cfgCode.EErrorCode_Activity_WaterMelon_Cfg
	}
	if len(r.NextLst) >= int(config.NextMaxCnt) {
		return 0
	}
	if r.InsideGameMaxLv <= 0 {
		r.InsideGameMaxLv = 1
	}

	cnt := int(config.NextMaxCnt) - len(r.NextLst)
	if cnt == int(config.NextMaxCnt) {
		if len(r.Snapshot.Records) > 0 {
			logger.Errorf("len(r.NextLst)[%d] len(r.Snapshot.Records)[%d] > 0", len(r.NextLst), len(r.Snapshot.Records))
			return cfgCode.EErrorCode_Activity_WaterMelon_Logic
		}
		for _, lvl := range config.InitFruit {
			r.AutoIncrId++
			autoId := r.AutoIncrId
			r.NextLst = append(r.NextLst, &msg.WaterMelonEntity{
				Id:    autoId,
				Level: lvl,
			})
			if lvl > r.InsideGameMaxLv {
				r.InsideGameMaxLv = lvl
			}
		}
		return 0
	}

	weight, ok := s.LvlToWeight[r.InsideGameMaxLv]
	if !ok {
		logger.Errorf("cant find InsideGameMaxLv[%d]", r.InsideGameMaxLv)
		return cfgCode.EErrorCode_Activity_WaterMelon_Cfg
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
			logger.Errorf("makeNextList lvl[%d] < 0, rand_num[%d] weight[%d]", lvl, num, weight)
			return cfgCode.EErrorCode_Activity_WaterMelon_Logic
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
