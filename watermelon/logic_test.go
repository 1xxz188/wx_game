package watermelon

import (
	"testing"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/msg"
	"wx_game/role"
)

func TestNext(t *testing.T) {
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

	data.NextLst = data.NextLst[:1]

	data.InsideGameMaxLv = 3
	s.makeNextList(data)
	t.Log(data.NextLst)

	data.NextLst = data.NextLst[:1]
	data.InsideGameMaxLv = 8
	s.makeNextList(data)
	t.Log(data.NextLst)
}
