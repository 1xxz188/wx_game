package watermelon

import (
	"math"
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

	data := &msg.DBWaterMelon{
		NextLst:  make([]*msg.WaterMelonEntity, 0),
		Snapshot: &msg.WaterMelonRecordSnapshot{},
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
	req1 := &msg.WaterMelonRecordSnapshot{
		Records: make([]*msg.WaterMelonEntity, 0),
	}
	req1.Records = append(req1.Records, &msg.WaterMelonEntity{
		Id:    1,
		Level: 1,
	})

	req2 := &msg.WaterMelonRecordSnapshot{
		Records: make([]*msg.WaterMelonEntity, 0),
	}
	req2.Records = append(req2.Records, &msg.WaterMelonEntity{
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

	respStart1, err := s.Start(nil, 0, nil, ctx)
	respStart := respStart1.(*msg.WATERMELON_START_Response)
	noErr(t, err)

	snapshot := respStart.Snapshot
	snapshot.Records = append(snapshot.Records, respStart.EntityLst[0])
	reqFall := &msg.WATERMELON_FALL_Request{
		WaterMelonId: respStart.EntityLst[0].Id,
		Snapshot:     snapshot,
	}
	_, err = s.Fall(nil, 0, reqFall, ctx)
	noErr(t, err)

	snapshot.Records = append(snapshot.Records, respStart.EntityLst[1])
	reqFall1 := &msg.WATERMELON_FALL_Request{
		WaterMelonId: 2,
		Snapshot:     snapshot,
	}
	_, err = s.Fall(nil, 0, reqFall1, ctx)
	noErr(t, err)

	snapshot.Records = snapshot.Records[1:]
	newSnapshot, err := fw.DeepCopyInterface(snapshot)
	noErr(t, err)
	newSnapshot.(*msg.WaterMelonRecordSnapshot).Records[0].Level = 2
	reqSync := &msg.WATERMELON_SYNC_Request{
		MergeLst: append([]*msg.WATER_MELON_MERGE_DETAIL{}, &msg.WATER_MELON_MERGE_DETAIL{
			FromId: 1,
			ToId:   2,
		}),
		Snapshot: newSnapshot.(*msg.WaterMelonRecordSnapshot),
	}
	t.Log("Sync snapshot: ", reqSync.Snapshot)
	respSync, err := s.Sync(nil, 0, reqSync, ctx)
	noErr(t, err)
	if respSync.(*msg.WATERMELON_SYNC_Response).ErrorCode != 0 {
		t.Fatal(respSync.(*msg.WATERMELON_SYNC_Response).ErrorCode)
	}

	reqRank := &msg.Rank_Request{
		Page: 0,
	}
	respRank1, err := s.Rank(nil, 0, reqRank, ctx)
	noErr(t, err)
	t.Log(respRank1)
}

func TestFloat(t *testing.T) {
	t.Log(int32(math.Ceil(float64(1) / float64(30))))
}
