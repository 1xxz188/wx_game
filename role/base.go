package role

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/msg"
)

type Info struct {
	rwLock     sync.RWMutex
	OpenID     string
	Role       *msg.DBRole
	Item       *msg.DBItem
	Watermelon *msg.DBWaterMelon
}

type Mgr struct {
	lockNextId atomic.Int64
	roleIdMap  cmap.ConcurrentMap[string, fw.ObjID] //OpenId->RoleId
	roleMap    cmap.ConcurrentMap[string, *Info]
}

func New() *Mgr {
	return &Mgr{
		roleIdMap: cmap.New[fw.ObjID](),
		roleMap:   cmap.New[*Info](),
	}
}

func (r *Mgr) GetRoleIdOrCreate(openId string) fw.ObjID {
	roleId, ok := r.roleIdMap.Get(openId)
	if !ok {
		newRoleId := r.lockNextId.Add(1)
		r.roleIdMap.SetIfAbsent(openId, fw.ObjID(newRoleId))
		roleId, _ = r.roleIdMap.Get(openId)
	}
	return roleId
}

func (r *Mgr) ReadRole(openId string, fn func(*Info)) {
	roleId := r.GetRoleIdOrCreate(openId)
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := r.roleMap.Get(sId)
	if !ok {
		v = r.newInfo(openId, roleId)
		if r.roleMap.SetIfAbsent(sId, v) {
			r.initRole(v.Role)
		}
		v, _ = r.roleMap.Get(sId)
	}
	v.rwLock.RLock()
	defer v.rwLock.RUnlock()
	fn(v)
}

func (r *Mgr) WriteRole(openId string, fn func(*Info)) {
	roleId := r.GetRoleIdOrCreate(openId)
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := r.roleMap.Get(sId)
	if !ok {
		v = r.newInfo(openId, roleId)
		if r.roleMap.SetIfAbsent(sId, v) {
			r.initRole(v.Role)
		}
		v, _ = r.roleMap.Get(sId)
	}
	v.rwLock.Lock()
	defer v.rwLock.Unlock()
	fn(v)
}

func (r *Mgr) newInfo(openId string, roleId fw.ObjID) *Info {
	return &Info{
		OpenID: openId,
		Role: &msg.DBRole{
			Base: &msg.RoleBase{
				RoleId: int64(roleId),
			},
		},
		Item: &msg.DBItem{
			RoleId:  int64(roleId),
			MapItem: make(map[int32]int32),
		},
		Watermelon: &msg.DBWaterMelon{
			RoleId:               int64(roleId),
			Snapshot:             &msg.WaterMelonRecordSnapshot{},
			MapMergeRecord:       make(map[int32]int32),
			MapMergeInsideRecord: make(map[int32]int32),
			MapInsideItemCount:   make(map[int32]int32),
		},
	}
}

func (r *Mgr) initRole(role *msg.DBRole) {
	role.Base.Name = "player" + strconv.FormatInt(role.Base.RoleId, 10)

	if len(cfg.Tables().TbPlayerAvatar.GetDataList()) > 0 {
		role.Base.AvatarId = cfg.Tables().TbPlayerAvatar.GetDataList()[0].EAvatar
	}

	if len(cfg.Tables().TbPlayerFrame.GetDataList()) > 0 {
		role.Base.FrameId = cfg.Tables().TbPlayerFrame.GetDataList()[0].EFrame
	}
}
