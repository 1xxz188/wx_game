package watermelon

import (
	"math/rand"
	"testing"
	"time"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"

	"github.com/stretchr/testify/require"
)

func TestNext(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	cfg.SetDataDir("../cfg_data/")
	err := cfg.Init()
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	registry := fw.NewMessageRegistry()
	roleMgr := role.New()
	err = s.Init(registry, roleMgr)
	require.NoError(t, err)

	data := &msg.DbWatermelon{
		NextLst:  make([]*msg.WatermelonEntity, 0),
		Snapshot: &msg.WatermelonRecordSnapshot{},
	}

	s.makeNextList(data)
	t.Log(data.NextLst)

	data.NextLst = data.NextLst[1:]

	data.InsideGameMaxLv = 3
	s.makeNextList(data)
	t.Log(data.NextLst)

	data.NextLst = data.NextLst[1:]
	data.InsideGameMaxLv = 11
	s.makeNextList(data)
	t.Log(data.NextLst)

	clear(data.NextLst)
	data.NextLst = data.NextLst[:0]
	t.Log(len(data.NextLst), cap(data.NextLst))

	data.AutoIncrId = 0
	s.makeNextList(data)
	t.Log(data.NextLst)
}

func Test1(t *testing.T) {
	req1 := &msg.WatermelonRecordSnapshot{
		Records: make([]*msg.WatermelonEntity, 0),
	}
	req1.Records = append(req1.Records, &msg.WatermelonEntity{
		Id:    1,
		Level: 1,
	})

	req2 := &msg.WatermelonRecordSnapshot{
		Records: make([]*msg.WatermelonEntity, 0),
	}
	req2.Records = append(req2.Records, &msg.WatermelonEntity{
		Id:    1,
		Level: 1,
	})

	t.Log(EqualSnapshot(req1, req2))
}

func Test2(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	cfg.SetDataDir("../cfg_data/")
	err := cfg.Init()
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	registry := fw.NewMessageRegistry()
	roleMgr := role.New()
	err = s.Init(registry, roleMgr)
	require.NoError(t, err)

	ctx := &fw.ConnectionContext{OpenID: "1"}
	roleMgr.LoginRole(ctx.OpenID, nil)

	respStart1, err := s.Start(nil, 0, nil, ctx)
	respStart := respStart1.(*msg.WatermelonStartResponse)
	noErr(t, err)
	require.Equal(t, int32(0), respStart.ErrorCode)

	snapshot := respStart.Snapshot
	snapshot.Records = append(snapshot.Records, respStart.EntityLst[0])
	reqFall := &msg.WatermelonFallRequest{
		WatermelonId: respStart.EntityLst[0].Id,
		Snapshot:     snapshot,
	}
	_, err = s.Fall(nil, 0, reqFall, ctx)
	noErr(t, err)

	snapshot.Records = append(snapshot.Records, respStart.EntityLst[1])
	reqFall1 := &msg.WatermelonFallRequest{
		WatermelonId: 2,
		Snapshot:     snapshot,
	}
	_, err = s.Fall(nil, 0, reqFall1, ctx)
	noErr(t, err)

	snapshot.Records = snapshot.Records[1:]
	newSnapshot, err := fw.DeepCopyInterface(snapshot)
	noErr(t, err)
	newSnapshot.(*msg.WatermelonRecordSnapshot).Records[0].Level = 2
	reqSync := &msg.WatermelonSyncRequest{
		MergeLst: append([]*msg.WatermelonMergeDetail{}, &msg.WatermelonMergeDetail{
			FromId: 1,
			ToId:   2,
		}),
		Snapshot: newSnapshot.(*msg.WatermelonRecordSnapshot),
	}
	t.Log("Sync snapshot: ", reqSync.Snapshot)
	respSync, err := s.Sync(nil, 0, reqSync, ctx)
	noErr(t, err)
	if respSync.(*msg.WatermelonSyncResponse).ErrorCode != 0 {
		t.Fatal(respSync.(*msg.WatermelonSyncResponse).ErrorCode)
	}

	reqRank := &msg.RankRequest{
		Page: 0,
	}
	respRank1, err := s.Rank(nil, 0, reqRank, ctx)
	noErr(t, err)
	t.Log(respRank1)
}

// TestMakeNextListStage1MinLevel 验证当 Stage=1 时，生成的等级不低于 Stage1Level
func TestMakeNextListStage1MinLevel(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	cfg.SetDataDir("../cfg_data/")
	err := cfg.Init()
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	registry := fw.NewMessageRegistry()
	roleMgr := role.New()
	err = s.Init(registry, roleMgr)
	require.NoError(t, err)

	config := cfg.Tables().TbWaterMelonConfig.Get(1)
	require.NotNil(t, config)
	stage1Level := config.Stage1Level
	nextMaxCnt := config.NextMaxCnt
	t.Logf("Stage1Level from config: %d, NextMaxCnt: %d", stage1Level, nextMaxCnt)

	// 辅助函数：创建预填充的 NextLst（跳过初始化逻辑）
	createPrefilledData := func(stage int32, insideGameMaxLv int32, autoIncrId int32) *msg.DbWatermelon {
		// 预先填充 nextMaxCnt-1 个元素，这样函数只会生成 1 个新元素
		nextLst := make([]*msg.WatermelonEntity, nextMaxCnt-1)
		for i := range nextLst {
			nextLst[i] = &msg.WatermelonEntity{Id: int32(i + 1), Level: 1}
		}
		return &msg.DbWatermelon{
			NextLst:         nextLst,
			Snapshot:        &msg.WatermelonRecordSnapshot{},
			InsideGameMaxLv: insideGameMaxLv,
			Stage:           stage,
			AutoIncrId:      autoIncrId,
		}
	}

	// 测试1: Stage=0 时，可以生成任何等级（包括低于 Stage1Level 的）
	t.Run("Stage0_CanGenerateLowLevel", func(t *testing.T) {
		hasLowLevel := false
		for i := 0; i < 500; i++ {
			data := createPrefilledData(0, 5, int32(i*10))
			errCode := s.makeNextList(data)
			require.Equal(t, int32(0), errCode)
			require.Equal(t, int(nextMaxCnt), len(data.NextLst))

			// 检查新生成的元素（最后一个）
			newEntity := data.NextLst[len(data.NextLst)-1]
			if newEntity.Level < stage1Level {
				hasLowLevel = true
				break
			}
		}
		// Stage=0 时应该能生成低于 Stage1Level 的等级
		require.True(t, hasLowLevel, "Stage=0 时应该能生成低于 Stage1Level(%d) 的等级", stage1Level)
		t.Logf("Stage=0: 成功生成了低于 Stage1Level(%d) 的等级", stage1Level)
	})

	// 测试2: Stage=1 时，所有生成的等级都 >= Stage1Level
	t.Run("Stage1_MinLevelIsStage1Level", func(t *testing.T) {
		for i := 0; i < 500; i++ {
			data := createPrefilledData(1, 7, int32(i*10))
			errCode := s.makeNextList(data)
			require.Equal(t, int32(0), errCode)
			require.Equal(t, int(nextMaxCnt), len(data.NextLst))

			// 检查新生成的元素（最后一个）
			newEntity := data.NextLst[len(data.NextLst)-1]
			require.GreaterOrEqual(t, newEntity.Level, stage1Level,
				"Stage=1 时生成的等级(%d)应该 >= Stage1Level(%d)", newEntity.Level, stage1Level)
		}
		t.Logf("Stage=1: 500次测试全部通过，所有生成等级都 >= Stage1Level(%d)", stage1Level)
	})

	// 测试3: Stage=1 但 InsideGameMaxLv < Stage1Level 时，回退到原始逻辑
	t.Run("Stage1_InsideGameMaxLvLessThanStage1Level_Fallback", func(t *testing.T) {
		data := createPrefilledData(1, 2, 100) // InsideGameMaxLv=2 < Stage1Level=3
		errCode := s.makeNextList(data)
		require.Equal(t, int32(0), errCode)
		require.Equal(t, int(nextMaxCnt), len(data.NextLst), "应该成功生成 NextLst")
		t.Logf("Stage=1, InsideGameMaxLv=2: 回退逻辑正常，生成了实体")
	})

	// 测试4: 统计 Stage=1 时生成等级的分布
	t.Run("Stage1_LevelDistribution", func(t *testing.T) {
		levelCount := make(map[int32]int)
		totalCount := 0

		for i := 0; i < 2000; i++ {
			data := createPrefilledData(1, 7, int32(i*10))
			errCode := s.makeNextList(data)
			require.Equal(t, int32(0), errCode)

			// 统计新生成的元素
			newEntity := data.NextLst[len(data.NextLst)-1]
			levelCount[newEntity.Level]++
			totalCount++
		}

		t.Logf("Stage=1 等级分布统计 (总数: %d):", totalCount)
		for lvl := int32(1); lvl <= 7; lvl++ {
			count := levelCount[lvl]
			percentage := float64(count) / float64(totalCount) * 100
			t.Logf("  Level %d: %d (%.2f%%)", lvl, count, percentage)

			// 验证低于 Stage1Level 的等级数量为0
			if lvl < stage1Level {
				require.Equal(t, 0, count, "Stage=1 时不应该生成等级 %d", lvl)
			}
		}
	})

	// 测试5: 对比 Stage=0 和 Stage=1 的分布差异
	t.Run("CompareStage0AndStage1Distribution", func(t *testing.T) {
		stage0Count := make(map[int32]int)
		stage1Count := make(map[int32]int)

		for i := 0; i < 2000; i++ {
			// Stage=0
			data0 := createPrefilledData(0, 7, int32(i*10))
			s.makeNextList(data0)
			stage0Count[data0.NextLst[len(data0.NextLst)-1].Level]++

			// Stage=1
			data1 := createPrefilledData(1, 7, int32(i*10+5))
			s.makeNextList(data1)
			stage1Count[data1.NextLst[len(data1.NextLst)-1].Level]++
		}

		t.Log("Stage=0 vs Stage=1 等级分布对比:")
		t.Log("Level\tStage0\t\tStage1")
		for lvl := int32(1); lvl <= 7; lvl++ {
			s0 := stage0Count[lvl]
			s1 := stage1Count[lvl]
			t.Logf("  %d\t%d (%.1f%%)\t%d (%.1f%%)", lvl,
				s0, float64(s0)/2000*100,
				s1, float64(s1)/2000*100)
		}

		// 验证 Stage=1 时低等级数量为0
		for lvl := int32(1); lvl < stage1Level; lvl++ {
			require.Equal(t, 0, stage1Count[lvl], "Stage=1 时不应该生成等级 %d", lvl)
			require.Greater(t, stage0Count[lvl], 0, "Stage=0 时应该能生成等级 %d", lvl)
		}
	})
}
