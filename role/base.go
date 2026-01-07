package role

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"wx_game/cfg"
	"wx_game/fw"
	"wx_game/fw/persistence"
	"wx_game/fw/persistence/mongoop"
	"wx_game/msg"

	"github.com/donnie4w/go-logger/logger"
	cmap "github.com/orcaman/concurrent-map/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/protobuf/proto"
)

const (
	// MongoDB集合名称
	CollectionRole       = "role"
	CollectionItem       = "item"
	CollectionWatermelon = "watermelon"

	// MongoDB键名前缀
	KeyPrefixRole       = "role_"
	KeyPrefixItem       = "item_"
	KeyPrefixWatermelon = "watermelon_"
)

type Info struct {
	rwLock     sync.RWMutex
	OpenID     string
	Role       *msg.DBRole
	Item       *msg.DBItem
	Watermelon *msg.DBWaterMelon
	dirty      atomic.Bool // 标记是否需要保存
}

// Key 返回对象的唯一标识符
func (info *Info) Key() string {
	info.rwLock.RLock()
	defer info.rwLock.RUnlock()
	if info.Role == nil || info.Role.Base == nil {
		return ""
	}
	return strconv.FormatInt(info.Role.Base.RoleId, 10)
}

// IsValid 判断对象是否有效
func (info *Info) IsValid() bool {
	info.rwLock.RLock()
	defer info.rwLock.RUnlock()
	// 检查基本字段是否有效
	if info.Role == nil || info.Role.Base == nil {
		return false
	}
	if info.Role.Base.RoleId <= 0 {
		return false
	}
	return true
}

// Save 实现 Saveable 接口，返回需要保存的数据列表
func (info *Info) Save() ([]persistence.SaveData, error) {
	// 检查是否需要保存，并尝试清除dirty标记
	// 使用原子操作：只有当dirty为true时，才将其设置为false
	// 如果清除失败（dirty已经是false），说明保存期间有新变化，不应该保存这次的数据
	if !info.dirty.CompareAndSwap(true, false) {
		// dirty不是true，或者已经被其他goroutine清除，返回空列表
		return []persistence.SaveData{}, nil
	}

	// 加锁读取数据
	info.rwLock.RLock()
	// 检查 Role 和 Base 是否有效
	if info.Role == nil || info.Role.Base == nil {
		info.rwLock.RUnlock()
		// 无效的数据，恢复dirty标记
		info.dirty.Store(true)
		return nil, nil
	}

	roleIdInt := info.Role.Base.RoleId
	if roleIdInt <= 0 {
		info.rwLock.RUnlock()
		// 无效的角色ID，恢复dirty标记
		info.dirty.Store(true)
		return nil, nil
	}

	roleIdStr := strconv.FormatInt(roleIdInt, 10)

	// 深拷贝 protobuf 消息，避免在保存过程中数据被其他 goroutine 修改
	roleData := proto.Clone(info.Role).(*msg.DBRole)
	itemData := proto.Clone(info.Item).(*msg.DBItem)
	watermelonData := proto.Clone(info.Watermelon).(*msg.DBWaterMelon)
	info.rwLock.RUnlock()

	var saveDataList []persistence.SaveData

	// 使用计数器跟踪该角色的保存失败数量
	// 如果有任何数据项保存失败，需要恢复dirty标记
	var failCount atomic.Int32
	onSaveSuccess := func() {
		// 保存成功，dirty已经在上面清除了，不需要再操作
	}
	onSaveFailure := func() {
		if failCount.Add(1) == 1 {
			// 第一个失败，恢复dirty标记，确保下次会重试
			info.dirty.Store(true)
		}
	}

	// 添加角色数据
	saveDataList = append(saveDataList, persistence.SaveData{
		Collection: CollectionRole,
		ID:         KeyPrefixRole + roleIdStr,
		Data:       roleData,
		OnSuccess:  onSaveSuccess,
		OnFailure:  onSaveFailure,
	})

	// 添加物品数据
	saveDataList = append(saveDataList, persistence.SaveData{
		Collection: CollectionItem,
		ID:         KeyPrefixItem + roleIdStr,
		Data:       itemData,
		OnSuccess:  onSaveSuccess,
		OnFailure:  onSaveFailure,
	})

	// 添加西瓜数据
	saveDataList = append(saveDataList, persistence.SaveData{
		Collection: CollectionWatermelon,
		ID:         KeyPrefixWatermelon + roleIdStr,
		Data:       watermelonData,
		OnSuccess:  onSaveSuccess,
		OnFailure:  onSaveFailure,
	})

	return saveDataList, nil
}

type Mgr struct {
	lockNextId atomic.Int64
	roleIdMap  cmap.ConcurrentMap[string, fw.ObjID] //OpenId->role_id
	roleMap    cmap.ConcurrentMap[string, *Info]    //role_id->Info
	persistMgr *persistence.PersistManager          // 持久化管理器引用 --初始化就设置，不需要竞争锁
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
			r.initRole(openId, v.Role)
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
			r.initRole(openId, v.Role)
		}
		v, _ = r.roleMap.Get(sId)
	}
	v.rwLock.Lock()
	defer v.rwLock.Unlock()
	fn(v)
	// 标记为需要保存
	v.dirty.Store(true)

	if r.persistMgr != nil {
		r.persistMgr.AddPendingObject(v)
	}
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

func (r *Mgr) initRole(openId string, role *msg.DBRole) {
	logger.Infof("initRole open_id[%s] role_id[%d]", openId, role.Base.RoleId)
	role.Base.Name = "player" + strconv.FormatInt(role.Base.RoleId, 10)
	if len(cfg.Tables().TbPlayerAvatar.GetDataList()) > 0 {
		role.Base.AvatarId = cfg.Tables().TbPlayerAvatar.GetDataList()[0].EAvatar
	}

	if len(cfg.Tables().TbPlayerFrame.GetDataList()) > 0 {
		role.Base.FrameId = cfg.Tables().TbPlayerFrame.GetDataList()[0].EFrame
	}
}

// LoadFromMongo 从MongoDB加载所有角色数据到内存
// 如果加载失败，返回错误
func (r *Mgr) LoadFromMongo(mongoClient *mongoop.MongoClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db := mongoClient.C.Database(mongoClient.Cfg.Database)
	roleColl := db.Collection(CollectionRole)

	logger.Info("Starting to load role data from MongoDB...")

	// 查询所有角色数据
	cursor, err := roleColl.Find(ctx, bson.D{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	loadedCount := 0
	roleIdSet := make(map[int64]bool) // 用于记录已加载的角色ID

	// 遍历所有角色数据
	for cursor.Next(ctx) {
		var roleData msg.DBRole
		if err := cursor.Decode(&roleData); err != nil {
			logger.Errorf("Failed to decode role data: %v", err)
			continue
		}

		if roleData.Base == nil {
			logger.Warnf("Role data has nil Base, skipping")
			continue
		}

		roleId := roleData.Base.RoleId
		if roleId <= 0 {
			logger.Warnf("Invalid role_id: %d, skipping", roleId)
			continue
		}

		roleIdStr := strconv.FormatInt(roleId, 10)
		roleIdInt64 := fw.ObjID(roleId)

		// 更新最大角色ID
		currentMax := r.lockNextId.Load()
		if roleIdInt64 > fw.ObjID(currentMax) {
			r.lockNextId.Store(int64(roleIdInt64))
		}

		// 加载物品数据
		itemColl := db.Collection(CollectionItem)
		itemData := &msg.DBItem{}
		err := itemColl.FindOne(ctx, bson.D{{"_id", KeyPrefixItem + roleIdStr}}).Decode(itemData)
		if err != nil && err != mongo.ErrNoDocuments {
			logger.Errorf("Failed to load item data for role_id=%d: %v", roleId, err)
			return err
		}
		if err == mongo.ErrNoDocuments {
			// 如果没有物品数据，创建默认的
			itemData = &msg.DBItem{
				RoleId:  roleId,
				MapItem: make(map[int32]int32),
			}
		}

		// 加载西瓜数据
		watermelonColl := db.Collection(CollectionWatermelon)
		watermelonData := &msg.DBWaterMelon{}
		err = watermelonColl.FindOne(ctx, bson.D{{"_id", KeyPrefixWatermelon + roleIdStr}}).Decode(watermelonData)
		if err != nil && err != mongo.ErrNoDocuments {
			logger.Errorf("Failed to load watermelon data for role_id=%d: %v", roleId, err)
			return err
		}
		if err == mongo.ErrNoDocuments {
			// 如果没有西瓜数据，创建默认的
			watermelonData = &msg.DBWaterMelon{
				RoleId:               roleId,
				Snapshot:             &msg.WaterMelonRecordSnapshot{},
				MapMergeRecord:       make(map[int32]int32),
				MapMergeInsideRecord: make(map[int32]int32),
				MapInsideItemCount:   make(map[int32]int32),
			}
		}

		// 创建角色信息
		info := &Info{
			OpenID:     "", // OpenID需要从其他地方获取，这里先留空
			Role:       &roleData,
			Item:       itemData,
			Watermelon: watermelonData,
		}
		info.dirty.Store(false) // 从数据库加载的数据不需要保存

		// 存储到内存
		r.roleMap.Set(roleIdStr, info)
		roleIdSet[roleId] = true
		loadedCount++
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	logger.Infof("Successfully loaded %d roles from MongoDB", loadedCount)

	// 更新lockNextId，确保新创建的角色ID不会冲突
	if loadedCount > 0 {
		maxRoleId := int64(0)
		for roleId := range roleIdSet {
			if roleId > maxRoleId {
				maxRoleId = roleId
			}
		}
		if maxRoleId > 0 {
			r.lockNextId.Store(maxRoleId)
		}
	}

	return nil
}

// RegisterPersistFuncs 注册角色的保存函数到持久化管理器
// 保存 persistMgr 的引用，以便在 WriteRole 中使用
func (r *Mgr) RegisterPersistFuncs(persistMgr *persistence.PersistManager) {
	r.persistMgr = persistMgr
}
