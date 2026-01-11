package watermelon

import (
	"math"
	"math/rand/v2"
	"strconv"
	"time"
	"wx_game/cfg"
	cfgCode "wx_game/cfg/code"
	"wx_game/fw"
	"wx_game/fw/mdzset"
	"wx_game/msg"
	"wx_game/msg/msg_id"
	"wx_game/role"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/websocket/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/protobuf/proto"
)

type Model struct {
	roleMgr     *role.Mgr
	collectsMap cmap.ConcurrentMap[string, *msg.DbWatermelon]
	rank        *mdzset.SortedSet[int64]

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
		collectsMap: cmap.New[*msg.DbWatermelon](),
		LvlToWeight: make(map[int32]int32),
		cfgWeight:   make([]cfgWeight, 0),
		rank:        mdzset.NewWithFixedSize[int64]("watermelon", 2, 500),
	}
}

func (s *Model) Init(handler fw.MsgInterface, roleMgr *role.Mgr) {
	s.roleMgr = roleMgr
	handler.Register(fw.MessageID(msg_id.WatermelonStart),
		func() proto.Message { return &msg.WatermelonStartRequest{} },
		s.Start,
	)

	handler.Register(fw.MessageID(msg_id.WatermelonFall),
		func() proto.Message { return &msg.WatermelonFallRequest{} },
		s.Fall,
	)

	handler.Register(fw.MessageID(msg_id.WatermelonSync),
		func() proto.Message { return &msg.WatermelonSyncRequest{} },
		s.Sync,
	)

	handler.Register(fw.MessageID(msg_id.WatermelonEnd),
		func() proto.Message { return &msg.WatermelonEndRequest{} },
		s.End,
	)

	handler.Register(fw.MessageID(msg_id.WatermelonUseItem),
		func() proto.Message { return &msg.WatermelonUseItemRequest{} },
		s.UseItem,
	)

	handler.Register(fw.MessageID(msg_id.WatermelonAddItem),
		func() proto.Message { return &msg.WatermelonAddItemRequest{} },
		s.AddItem,
	)

	handler.Register(fw.MessageID(msg_id.Rank),
		func() proto.Message { return &msg.RankRequest{} },
		s.Rank,
	)

	handler.Register(fw.MessageID(msg_id.RoleAlterName),
		func() proto.Message { return &msg.RoleAlterNameRequest{} },
		s.AlterName,
	)

	handler.Register(fw.MessageID(msg_id.RoleAlterFace),
		func() proto.Message { return &msg.RoleAlterFaceRequest{} },
		s.AlterFace,
	)

	handler.Register(fw.MessageID(msg_id.RoleAlterStep),
		func() proto.Message { return &msg.RoleAlterStepRequest{} },
		s.AlterStep,
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

func (s *Model) GetOrCreate(roleId fw.ObjID) *msg.DbWatermelon {
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := s.collectsMap.Get(sId)
	if !ok {
		v = &msg.DbWatermelon{
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
	resp := &msg.WatermelonStartResponse{}

	err = s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			r.Watermelon.Snapshot = &msg.WatermelonRecordSnapshot{}
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
			r.Watermelon.InsideRemainAddCount = 2
		}
		dataItemCount, err = fw.DeepCopyInterface(r.Watermelon.MapInsideItemCount)
		if err != nil {
			logger.Errorf("err[%s] data[%+v]", err, r.Watermelon.NextLst)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}
		resp.RemainAddCount = r.Watermelon.InsideRemainAddCount
	})

	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.Snapshot = dataSnapshot.(*msg.WatermelonRecordSnapshot)
	resp.EntityLst = dataNext.([]*msg.WatermelonEntity)
	resp.MapItemCount = dataItemCount.(map[int32]int32)
	logger.Debugf("role_id[%d] id[%s] open_id[%s] start records[%d] next_list[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, len(resp.Snapshot.Records), resp.EntityLst)
	return resp, nil
}

func (s *Model) Fall(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonFallRequest)
	resp := &msg.WatermelonFallResponse{}

	var dataNext interface{}
	var err error
	err = s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if len(r.Watermelon.NextLst) <= 0 {
			logger.Errorf("len(r.Watermelon.NextLst) <= 0")
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		if r.Watermelon.NextLst[0].Id != req.WatermelonId {
			logger.Errorf("open_id[%s] r.Watermelon.NextLst[0].Id[%d] != req.WatermelonId[%d]", ctx.OpenID, r.Watermelon.NextLst[0].Id, req.WatermelonId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		for _, record := range req.Snapshot.Records {
			if record.Id > req.WatermelonId {
				logger.Errorf("open_id[%s] record.Id[%d] req.WatermelonId[%d]", ctx.OpenID, record.Id, req.WatermelonId)
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

	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.EntityLst = dataNext.([]*msg.WatermelonEntity)
	logger.Debugf("role_id[%d] id[%s] open_id[%s] fall id[%d]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req.WatermelonId)
	return resp, nil
}

func (s *Model) Sync(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonSyncRequest)
	resp := &msg.WatermelonSyncResponse{}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		if len(req.MergeLst) > 0 {
			if len(req.Snapshot.Records)+len(req.MergeLst) == len(r.Watermelon.Snapshot.Records) {
				mapSaveWaterMelonLevel := make(map[int32]int32, len(r.Watermelon.Snapshot.Records))
				mapMergeLevelCount := make(map[int32]int32)

				for _, record := range r.Watermelon.Snapshot.Records {
					mapSaveWaterMelonLevel[record.Id] = record.Level
				}

				for _, detail := range req.MergeLst {
					fromLvl, ok := mapSaveWaterMelonLevel[detail.FromId]
					if !ok {
						logger.Errorf("not find mapSaveWaterMelonLevel FromId[%d]", detail.FromId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					toLvl, ok := mapSaveWaterMelonLevel[detail.ToId]
					if !ok {
						logger.Errorf("not find mapSaveWaterMelonLevel ToId[%d]", detail.ToId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					if detail.FromId >= detail.ToId {
						logger.Errorf("detail.FromId[%d] >= detail.ToId[%d]", detail.FromId, detail.ToId)
						resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
						return
					}
					//必须相同等级
					if fromLvl != toLvl {
						logger.Errorf("detail.FromId == detail.ToId || mapSaveWaterMelonLevel[detail.FromId] != mapSaveWaterMelonLevel[detail.ToId]")
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
				logger.Errorf("open_id[%s] req_records_len[%d] + req.MergeLst_len[%d] != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(req.MergeLst), len(r.Watermelon.Snapshot.Records))
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}

			if r.Watermelon.Score > r.Watermelon.HistoryScore {
				r.Watermelon.HistoryScore = r.Watermelon.Score
				//更新排行榜
				err := s.rank.Add([]float64{float64(r.Watermelon.Score), -float64(time.Now().Unix())}, r.Role.Base.RoleId, &msg.RankStRole{
					RoleId:    r.Role.Base.RoleId,
					Name:      r.Role.Base.Name,
					AvatarId:  r.Role.Base.AvatarId,
					FrameId:   r.Role.Base.FrameId,
					AvatarUrl: r.Role.Base.AvatarUrl,
				})
				if err != nil {
					logger.Error("add rank fail", err)
				}
			}
		} else if len(req.Snapshot.Records) != len(r.Watermelon.Snapshot.Records) { //只做位置同步
			logger.Errorf("open_id[%s] req_records_len[%d] != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(r.Watermelon.Snapshot.Records))
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		r.Watermelon.Snapshot.Records = req.Snapshot.Records
		resp.Score = r.Watermelon.Score
	})

	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	logger.Debugf("role_id[%d] id[%s] open_id[%s] merge [%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req.MergeLst)
	return resp, nil
}

func (s *Model) End(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	resp := &msg.WatermelonEndResponse{}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
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
		r.Watermelon.InsideRemainAddCount = 0
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		// End 函数没有 ErrorCode 字段，只记录日志
	}
	logger.Debugf("role_id[%d] id[%s] open_id[%s] end", ctx.RoleId, ctx.ConnectionID, ctx.OpenID)
	return resp, nil
}

func (s *Model) UseItem(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonUseItemRequest)
	resp := &msg.WatermelonUseItemResponse{}

	if req.ItemNum != 1 {
		logger.Errorf("open_id[%s] req.ItemNum[%d] != 1", ctx.OpenID, req.ItemNum)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
		return resp, nil
	}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if len(r.Watermelon.Snapshot.Records) <= 0 {
			logger.Errorf("open_id[%s] UseItem len(r.Watermelon.Snapshot.Records) <= 0", ctx.OpenID)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		if len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records) {
			logger.Errorf("open_id[%s] len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records)", ctx.OpenID)
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
			logger.Errorf("open_id[%s] unknown req.ItemId[%d]", ctx.OpenID, req.ItemId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		r.Watermelon.Snapshot = req.Snapshot
		//_ = cfgcode.CommonItemId_WatermelonMagnet
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}
	resp.ItemId = req.ItemId
	logger.Debugf("role_id[%d] id[%s] open_id[%s] UserItem", ctx.RoleId, ctx.ConnectionID, ctx.OpenID)
	return resp, nil
}

func (s *Model) makeNextList(r *msg.DbWatermelon) int32 {
	const cfgDefaultId = 1
	config := cfg.Tables().TbWaterMelonConfig.Get(cfgDefaultId)
	if config == nil {
		logger.Errorf("cant find TbWaterMelonConfig cfgDefaultId[%d]", cfgDefaultId)
		return cfgCode.EErrorCode_Activity_WaterMelon_Cfg
	}
	if r.Snapshot == nil {
		logger.Errorf("makeNextList r.Snapshot == nil")
		return cfgCode.EErrorCode_Activity_WaterMelon_Logic
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
			r.NextLst = append(r.NextLst, &msg.WatermelonEntity{
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
		r.NextLst = append(r.NextLst, &msg.WatermelonEntity{
			Id:    autoId,
			Level: lvl,
		})
	}
	return 0
}

func (s *Model) AddItem(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonAddItemRequest)
	resp := &msg.WatermelonAddItemResponse{}

	if req.ItemNum <= 0 {
		logger.Errorf("open_id[%s] req.ItemNum[%d] <= 0", ctx.OpenID, req.ItemNum)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
		return resp, nil
	}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if r.Watermelon.InsideRemainAddCount <= 0 {
			resp.RemainAddCount = r.Watermelon.InsideRemainAddCount
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_ItemNotEnough)
			return
		}

		_, ok := r.Watermelon.MapInsideItemCount[req.ItemId]
		if !ok {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_ItemCantAdd)
			return
		}
		r.Watermelon.MapInsideItemCount[req.ItemId]++
		r.Watermelon.InsideRemainAddCount--
		resp.ItemNum = r.Watermelon.MapInsideItemCount[req.ItemId]
		resp.RemainAddCount = r.Watermelon.InsideRemainAddCount
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}
	resp.ItemId = req.ItemId
	logger.Debugf("role_id[%d] id[%s] open_id[%s] AddItem[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}

func (s *Model) Rank(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.RankRequest)
	resp := &msg.RankResponse{}

	resp.NumPerPage = 30
	resp.TotalPage = int32(math.Ceil(float64(s.rank.Len()) / float64(resp.NumPerPage)))

	if req.Page > resp.TotalPage {
		return resp, nil
	}

	rank := req.Page * resp.NumPerPage
	for _, v := range s.rank.Range(int(req.Page*resp.NumPerPage), int(req.Page*resp.NumPerPage+resp.NumPerPage)) {
		rank++
		resp.Items = append(resp.Items, &msg.RankStItem{
			Rank:   rank,
			Role:   v.Attachment.(*msg.RankStRole),
			Score:  int64(v.Score[0]),
			Score2: int64(v.Score[1]),
		})
	}

	err := s.roleMgr.ReadRole(ctx.OpenID, func(r *role.Role) {
		ran, scores, data := s.rank.Rank(r.Role.Base.RoleId, false)
		if scores == nil {
			return
		}
		if data == nil {
			return
		}
		if len(scores) != 2 {
			logger.Errorf("role_id[%d] rank scores len[%d] != 2", r.Role.Base.RoleId, len(scores))
			return
		}
		resp.Self = &msg.RankStItem{
			Rank:   int32(ran + 1),
			Role:   data.(*msg.RankStRole),
			Score:  int64(scores[0]),
			Score2: int64(scores[1]),
		}
	})

	if err != nil {
		logger.Error("ReadRole fail", err)
		resp.Code = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
		return resp, nil
	}
	return resp, nil
}

func (s *Model) AlterName(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.RoleAlterNameRequest)
	resp := &msg.RoleAlterNameResponse{}

	if len(req.Name) <= 0 {
		logger.Error("req.Name is empty")
		resp.Code = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
		return resp, nil
	}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		r.Role.Base.Name = req.Name
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	logger.Debugf("role_id[%d] id[%s] open_id[%s] AlterName[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}

func (s *Model) AlterFace(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.RoleAlterFaceRequest)
	resp := &msg.RoleAlterFaceResponse{}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		r.Role.Base.AvatarId = req.AvatarId
		r.Role.Base.FrameId = req.FrameId
		r.Role.Base.AvatarUrl = req.AvatarUrl
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	logger.Debugf("role_id[%d] id[%s] open_id[%s] AlterFace[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}

func (s *Model) AlterStep(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.RoleAlterStepRequest)
	resp := &msg.RoleAlterStepResponse{}

	if req.Step < 0 { //TODO 范围判断
		logger.Error("req.Step < 0")
		resp.Code = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter) //TODO 新增错误码
		return resp, nil
	}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		r.Role.Base.Step = req.Step
	})
	if err != nil {
		logger.Errorf("open_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	resp.Step = req.Step
	logger.Debugf("role_id[%d] id[%s] open_id[%s] AlterStep[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}
