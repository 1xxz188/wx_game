package watermelon

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"time"
	"wx_game/cfg"
	cfgCode "wx_game/cfg/code"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/msg/msg_id"
	"wx_game/rank"
	"wx_game/role"

	"github.com/donnie4w/go-logger/logger"
	"github.com/gofiber/websocket/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/protobuf/proto"
)

const cfgWatermelonDefaultId = 1 //西瓜配置表默认索引

type Model struct {
	roleMgr     *role.Mgr
	rankMgr     *rank.Manager[int64, *msg.RankStRole]
	collectsMap cmap.ConcurrentMap[string, *msg.DbWatermelon]

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
	}
}

func (s *Model) Init(handler fw.MsgInterface, roleMgr *role.Mgr, rankMgr *rank.Manager[int64, *msg.RankStRole]) error {
	s.roleMgr = roleMgr
	s.rankMgr = rankMgr
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

	handler.Register(fw.MessageID(msg_id.BugFeedback),
		func() proto.Message { return &msg.BugFeedbackRequest{} },
		s.BugFeedback,
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

	for _, v := range cfg.Tables().TbWaterMelonConfig.GetDataList() {
		//检测v.Stage1Level是否有配置权重
		if weight, ok := s.LvlToWeight[v.Stage1Level]; !ok || weight <= 0 {
			return fmt.Errorf("Stage1Level[%d] not found in LvlToWeight or weight[%d] <= 0", v.Stage1Level, weight)
		}
	}
	return nil
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
	config := cfg.Tables().TbWaterMelonConfig.Get(cfgWatermelonDefaultId)
	if config == nil {
		logger.Errorf("cant find TbWaterMelonConfig cfgDefaultId[%d]", cfgWatermelonDefaultId)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Cfg)
		return resp, nil
	}

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
			logger.Errorf("user_id[%s] Start makeNextList code[%d]", ctx.OpenID, code)
			resp.ErrorCode = int32(code)
			return
		}

		resp.HistoryScore = r.Watermelon.HistoryScore
		resp.Score = r.Watermelon.Score
		resp.Stage = r.Watermelon.Stage
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
		resp.FallCnt = r.Watermelon.FallCnt
	})

	if err != nil {
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.Snapshot = dataSnapshot.(*msg.WatermelonRecordSnapshot)
	resp.EntityLst = dataNext.([]*msg.WatermelonEntity)
	resp.MapItemCount = dataItemCount.(map[int32]int32)
	resp.Stage1Round = config.Stage1Round
	resp.Stage1Lvl = config.Stage1Level
	logger.Debugf("role_id[%d] id[%s] user_id[%s] start records[%d] next_list[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, len(resp.Snapshot.Records), resp.EntityLst)
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
			logger.Errorf("user_id[%s] r.Watermelon.NextLst[0].Id[%d] != req.WatermelonId[%d]", ctx.OpenID, r.Watermelon.NextLst[0].Id, req.WatermelonId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		for _, record := range req.Snapshot.Records {
			if record.Id > req.WatermelonId {
				logger.Errorf("user_id[%s] record.Id[%d] req.WatermelonId[%d]", ctx.OpenID, record.Id, req.WatermelonId)
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}
		}

		r.Watermelon.NextLst = r.Watermelon.NextLst[1:]
		r.Watermelon.Snapshot.Records = req.Snapshot.Records
		code := s.makeNextList(r.Watermelon)
		if code != 0 {
			logger.Errorf("user_id[%s] Fall makeNextList code[%d]", ctx.OpenID, code)
			resp.ErrorCode = int32(code)
			return
		}
		dataNext, err = fw.DeepCopyInterface(r.Watermelon.NextLst)
		if err != nil {
			logger.Errorf("err[%s] data[%+v]", err, r.Watermelon.NextLst)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
			return
		}
		r.Watermelon.FallCnt++
	})

	if err != nil {
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	if resp.ErrorCode != 0 {
		return resp, nil
	}

	resp.EntityLst = dataNext.([]*msg.WatermelonEntity)
	logger.Debugf("role_id[%d] id[%s] user_id[%s] fall id[%d]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req.WatermelonId)
	return resp, nil
}

func (s *Model) Sync(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonSyncRequest)
	resp := &msg.WatermelonSyncResponse{}

	// 用于在锁外更新排行榜的数据
	var rankData *msg.RankStRole
	var rankScores []float64

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		if len(req.MergeLst) > 0 { //合并的情况
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
				logger.Errorf("user_id[%s] req_records_len[%d] + req.MergeLst_len[%d] != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(req.MergeLst), len(r.Watermelon.Snapshot.Records))
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}

			if r.Watermelon.Score > r.Watermelon.HistoryScore {
				r.Watermelon.HistoryScore = r.Watermelon.Score
				// 收集排行榜数据，在锁外更新
				rankScores = []float64{float64(r.Watermelon.Score), -float64(time.Now().Unix())}
				rankData = &msg.RankStRole{
					RoleId:    r.Role.Base.RoleId,
					Name:      r.Role.Base.Name,
					AvatarId:  r.Role.Base.AvatarId,
					FrameId:   r.Role.Base.FrameId,
					AvatarUrl: r.Role.Base.AvatarUrl,
				}
			}
		} else { //只做位置同步或升级阶段
			if len(req.Snapshot.Records) != len(r.Watermelon.Snapshot.Records) {
				logger.Errorf("user_id[%s] req_records_len[%d] != data_records_len[%d]", ctx.OpenID, len(req.Snapshot.Records), len(r.Watermelon.Snapshot.Records))
				resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
				return
			}
			if r.Watermelon.Stage < req.Stage {
				if req.Stage != 1 {
					logger.Errorf("user_id[%s] role_id[%d] req.Stage[%d] < r.Watermelon.Stage[%d]", ctx.OpenID, ctx.RoleId, req.Stage, r.Watermelon.Stage)
					resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
					return
				}
				//升级阶段
				r.Watermelon.Stage = req.Stage

				//升级NextLst
				s.makeNextList(r.Watermelon)
			}
		}

		r.Watermelon.Snapshot.Records = req.Snapshot.Records
		resp.Score = r.Watermelon.Score
	})

	if err != nil {
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	// 在锁外更新排行榜
	if rankData != nil {
		if err := s.rankMgr.Add(rankData.RoleId, rankScores, rankData); err != nil {
			logger.Error("add rank fail", err)
		}
	}

	logger.Debugf("role_id[%d] id[%s] user_id[%s] merge [%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req.MergeLst)
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
		r.Watermelon.FallCnt = 0
	})
	if err != nil {
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		// End 函数没有 ErrorCode 字段，只记录日志
	}
	logger.Debugf("role_id[%d] id[%s] user_id[%s] end", ctx.RoleId, ctx.ConnectionID, ctx.OpenID)
	return resp, nil
}

func (s *Model) UseItem(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.WatermelonUseItemRequest)
	resp := &msg.WatermelonUseItemResponse{}

	if req.ItemNum != 1 {
		logger.Errorf("user_id[%s] req.ItemNum[%d] != 1", ctx.OpenID, req.ItemNum)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
		return resp, nil
	}

	err := s.roleMgr.WriteRole(ctx.OpenID, func(r *role.Role) {
		if r.Watermelon.Snapshot == nil {
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}

		if len(r.Watermelon.Snapshot.Records) <= 0 {
			logger.Errorf("user_id[%s] UseItem len(r.Watermelon.Snapshot.Records) <= 0", ctx.OpenID)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Condition)
			return
		}

		if len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records) {
			logger.Errorf("user_id[%s] len(req.Snapshot.Records) > len(r.Watermelon.Snapshot.Records)", ctx.OpenID)
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
			logger.Errorf("user_id[%s] unknown req.ItemId[%d]", ctx.OpenID, req.ItemId)
			resp.ErrorCode = int32(cfgCode.EErrorCode_Activity_WaterMelon_Parameter)
			return
		}
		r.Watermelon.Snapshot = req.Snapshot
		//_ = cfgcode.CommonItemId_WatermelonMagnet
	})
	if err != nil {
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}
	resp.ItemId = req.ItemId
	logger.Debugf("role_id[%d] id[%s] user_id[%s] UserItem", ctx.RoleId, ctx.ConnectionID, ctx.OpenID)
	return resp, nil
}

func (s *Model) makeNextList(r *msg.DbWatermelon) int32 {
	config := cfg.Tables().TbWaterMelonConfig.Get(cfgWatermelonDefaultId)
	if config == nil {
		logger.Errorf("cant find TbWaterMelonConfig cfgWatermelonDefaultId[%d]", cfgWatermelonDefaultId)
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
	if cnt == int(config.NextMaxCnt) { //初始化
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

	// 当阶段为1时，最小生成等级为 Stage1Level
	minLevelWeightOffset := int32(0)
	if r.Stage == 1 && config.Stage1Level > 1 {
		// 获取 Stage1Level-1 的累积权重作为偏移量，跳过低等级的权重范围
		if offsetWeight, ok := s.LvlToWeight[config.Stage1Level-1]; ok {
			minLevelWeightOffset = offsetWeight
		}
	}

	for cnt > 0 {
		cnt--
		// 计算可用权重范围（排除低于最小等级的部分）
		availableWeight := weight - minLevelWeightOffset
		if availableWeight <= 0 {
			// 如果 InsideGameMaxLv < Stage1Level，回退到原始逻辑
			availableWeight = weight
			minLevelWeightOffset = 0
		}
		num := rand.Int32N(availableWeight) + minLevelWeightOffset
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
		logger.Errorf("user_id[%s] req.ItemNum[%d] <= 0", ctx.OpenID, req.ItemNum)
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
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.ErrorCode = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}
	resp.ItemId = req.ItemId
	logger.Debugf("role_id[%d] id[%s] user_id[%s] AddItem[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}

func (s *Model) Rank(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.RankRequest)
	resp := &msg.RankResponse{}

	resp.NumPerPage = 30
	resp.TotalPage = int32(math.Ceil(float64(s.rankMgr.Len()) / float64(resp.NumPerPage)))

	if req.Page > resp.TotalPage {
		return resp, nil
	}

	rankNum := req.Page * resp.NumPerPage
	for _, v := range s.rankMgr.Range(int(req.Page*resp.NumPerPage), int(req.Page*resp.NumPerPage+resp.NumPerPage)) {
		rankNum++
		resp.Items = append(resp.Items, &msg.RankStItem{
			Rank:   rankNum,
			Role:   v.Attachment.(*msg.RankStRole),
			Score:  int64(v.Score[0]),
			Score2: int64(v.Score[1]),
		})
	}

	var roleId int64
	err := s.roleMgr.ReadRole(ctx.OpenID, func(r *role.Role) {
		roleId = r.Role.Base.RoleId
	})
	if err != nil {
		logger.Error("ReadRole fail", err)
		resp.Code = int32(cfgCode.EErrorCode_Activity_WaterMelon_Logic)
		return resp, nil
	}

	ran, scores, data := s.rankMgr.Rank(roleId, false)
	if scores != nil && data != nil {
		if len(scores) == 2 {
			resp.Self = &msg.RankStItem{
				Rank:   int32(ran + 1),
				Role:   data.(*msg.RankStRole),
				Score:  int64(scores[0]),
				Score2: int64(scores[1]),
			}
		} else {
			logger.Errorf("role_id[%d] rank scores len[%d] != 2", roleId, len(scores))
		}
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
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	logger.Debugf("role_id[%d] id[%s] user_id[%s] AlterName[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
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
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	logger.Debugf("role_id[%d] id[%s] user_id[%s] AlterFace[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
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
		logger.Errorf("user_id[%s] WriteRole failed: %v", ctx.OpenID, err)
		resp.Code = int32(cfgCode.EErrorCode_Internal)
		return resp, nil
	}

	resp.Step = req.Step
	logger.Debugf("role_id[%d] id[%s] user_id[%s] AlterStep[%v]", ctx.RoleId, ctx.ConnectionID, ctx.OpenID, req)
	return resp, nil
}

func (s *Model) BugFeedback(c *websocket.Conn, msgID fw.MessageID, m proto.Message, ctx *fw.ConnectionContext) (proto.Message, error) {
	req := m.(*msg.BugFeedbackRequest)
	resp := &msg.BugFeedbackResponse{}
	logger.Infof("BugFeedback role_id[%d] user_id[%s] [%v]", ctx.RoleId, ctx.OpenID, req.Msg)
	return resp, nil
}
