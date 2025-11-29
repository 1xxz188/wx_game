package watermelon

import (
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
	cfg.SetDataDir("../cfg/data/")
	err := cfg.Init()
	if err != nil {
		t.Fatal(err)
	}

	s := New()
	registry := fw.NewMessageRegistry()
	roleMgr := role.New()
	s.Init(registry, roleMgr)

	data := &msg.DBWaterMelon{
		NextLst: make([]*msg.WaterMelonEntity, 0),
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
	cfg.SetDataDir("../cfg/data/")
	err := cfg.Init()
	if err != nil {
		t.Fatal(err)
	}

}
