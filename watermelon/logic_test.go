package watermelon

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"
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
	s.Init(registry, roleMgr)

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
	s.Init(registry, roleMgr)

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
