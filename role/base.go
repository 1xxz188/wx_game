package role

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"sync/atomic"
	"wx_game/fw"
	"wx_game/msg"
)

type Info struct {
	OpenID string
	msg.DBWaterMelon
}

type Mgr struct {
	lockNextId atomic.Int64
	roleIdMap  cmap.ConcurrentMap[string, fw.ObjID]
}

func New() *Mgr {
	return &Mgr{
		roleIdMap: cmap.New[fw.ObjID](),
	}
}

func (r *Mgr) GetRoleIdOrCreate(id string) fw.ObjID {
	roleId, ok := r.roleIdMap.Get(id)
	if !ok {
		roleId := r.lockNextId.Add(1)
		r.roleIdMap.Set(id, fw.ObjID(roleId))
	}
	return roleId
}
