package role

import (
	"context"
	"errors"
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

type Role struct {
	rwLock     sync.RWMutex
	sId        string // roleId 的字符串缓存，创建后不变
	Role       *msg.DbRole
	Item       *msg.DbItem
	Watermelon *msg.DbWatermelon
	dirty      atomic.Bool // 标记是否需要保存
}

// Key 返回对象的唯一标识符（直接返回缓存的 sId，无需加锁）
func (r *Role) Key() string {
	return r.sId
}

// IsValid 判断对象是否有效
func (r *Role) IsValid() bool {
	r.rwLock.RLock()
	defer r.rwLock.RUnlock()
	// 检查基本字段是否有效
	if r.Role == nil || r.Role.Base == nil {
		return false
	}
	if r.Role.Base.RoleId <= 0 {
		return false
	}
	return true
}

// Save 实现 Savable 接口，返回需要保存的数据列表
func (r *Role) Save() ([]persistence.SaveData, error) {
	// 检查是否需要保存，并尝试清除dirty标记
	// 使用原子操作：只有当dirty为true时，才将其设置为false
	// 如果清除失败（dirty已经是false），说明保存期间有新变化，不应该保存这次的数据
	if !r.dirty.CompareAndSwap(true, false) {
		// dirty不是true，或者已经被其他goroutine清除，返回空列表
		return []persistence.SaveData{}, nil
	}

	// 加锁读取数据
	r.rwLock.RLock()
	// 检查 Role 和 Base 是否有效
	if r.Role == nil || r.Role.Base == nil {
		r.rwLock.RUnlock()
		// 无效的数据，恢复dirty标记
		r.dirty.Store(true)
		return nil, nil
	}

	roleIdInt := r.Role.Base.RoleId
	if roleIdInt <= 0 {
		r.rwLock.RUnlock()
		// 无效的角色ID，恢复dirty标记
		r.dirty.Store(true)
		return nil, nil
	}

	roleIdStr := strconv.FormatInt(roleIdInt, 10)

	// 深拷贝 protobuf 消息，避免在保存过程中数据被其他 goroutine 修改
	roleData := proto.Clone(r.Role).(*msg.DbRole)
	itemData := proto.Clone(r.Item).(*msg.DbItem)
	watermelonData := proto.Clone(r.Watermelon).(*msg.DbWatermelon)
	r.rwLock.RUnlock()

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
			r.dirty.Store(true)
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
	lockNextId     atomic.Int64
	userIdMap      cmap.ConcurrentMap[string, fw.ObjID] //user_id->role_id
	roleMap        cmap.ConcurrentMap[string, *Role]    //role_id->Role
	persistMgr     *persistence.PersistManager          // 持久化管理器引用 --初始化就设置，不需要竞争锁
	mongoClient    *mongoop.MongoClient                 // MongoDB客户端引用
	userLoginLocks sync.Map                             // userId -> *sync.Mutex，用于防止同一用户的并发LoginRole调用
}

func New() *Mgr {
	return &Mgr{
		userIdMap: cmap.New[fw.ObjID](),
		roleMap:   cmap.New[*Role](),
	}
}

func (r *Mgr) GetRoleId(userId string) (fw.ObjID, bool) {
	return r.userIdMap.Get(userId)
}

func (r *Mgr) getRoleIdOrCreate(userId string) fw.ObjID {
	roleId, ok := r.userIdMap.Get(userId)
	if !ok {
		newRoleId := r.lockNextId.Add(1)
		r.userIdMap.SetIfAbsent(userId, fw.ObjID(newRoleId))
		roleId, _ = r.userIdMap.Get(userId)
	}
	return roleId
}

// getOrCreateUserLock 获取或创建指定userId的互斥锁
// 用于防止同一用户的并发LoginRole调用
func (r *Mgr) getOrCreateUserLock(userId string) *sync.Mutex {
	// 尝试获取已存在的锁
	if lock, ok := r.userLoginLocks.Load(userId); ok {
		return lock.(*sync.Mutex)
	}
	// 创建新锁
	newLock := &sync.Mutex{}
	// 使用LoadOrStore确保只有一个goroutine能成功存储新锁
	lock, _ := r.userLoginLocks.LoadOrStore(userId, newLock)
	return lock.(*sync.Mutex)
}

func (r *Mgr) LoginRole(userId string, fn func(*Role)) {
	roleId := r.getRoleIdOrCreate(userId)
	sId := strconv.FormatInt(int64(roleId), 10)

	v, ok := r.roleMap.Get(sId)
	if !ok {
		userLock := r.getOrCreateUserLock(userId)
		userLock.Lock()
		v, ok = r.roleMap.Get(sId)
		if !ok {
			// 尝试从MongoDB加载数据
			v = r.loadInfoFromMongo(userId, roleId)
			if v == nil {
				// MongoDB中没有数据，创建新的Info
				v = r.newRole(userId, roleId)
				if r.roleMap.SetIfAbsent(sId, v) {
					r.initRole(userId, v.Role)
				}
				v, _ = r.roleMap.Get(sId)
			} else {
				// 从MongoDB加载成功，存储到内存
				r.roleMap.SetIfAbsent(sId, v)
				v, _ = r.roleMap.Get(sId)
			}
		}

		userLock.Unlock()
	}

	v.rwLock.Lock()
	defer v.rwLock.Unlock()
	v.Role.LastLoginTm = time.Now().Unix()
	fn(v)
}

func (r *Mgr) ReadRole(userId string, fn func(*Role)) error {
	roleId, ok := r.GetRoleId(userId)
	if !ok {
		return errors.New("userIdMap not found")
	}
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := r.roleMap.Get(sId)
	if !ok {
		return errors.New("roleMap not found")
	}
	v.rwLock.RLock()
	defer v.rwLock.RUnlock()
	fn(v)
	return nil
}

func (r *Mgr) WriteRole(userId string, fn func(*Role)) error {
	roleId, ok := r.GetRoleId(userId)
	if !ok {
		return errors.New("userIdMap not found")
	}
	sId := strconv.FormatInt(int64(roleId), 10)
	v, ok := r.roleMap.Get(sId)
	if !ok {
		return errors.New("roleMap not found")
	}
	v.rwLock.Lock()
	defer v.rwLock.Unlock()
	fn(v)
	// 标记为需要保存
	v.dirty.Store(true)

	if r.persistMgr != nil {
		// 直接使用缓存的 v.sId 作为 key，避免调用 v.Key() 导致死锁（因为已持有写锁）
		r.persistMgr.AddPendingObject(v.sId, v)
	}
	return nil
}

func (r *Mgr) newRole(userId string, roleId fw.ObjID) *Role {
	return &Role{
		sId: strconv.FormatInt(int64(roleId), 10),
		Role: &msg.DbRole{
			Base: &msg.RoleBase{
				RoleId:     int64(roleId),
				RegisterTm: time.Now().Unix(),
			},
			UserId: userId,
		},
		Item: &msg.DbItem{
			RoleId:  int64(roleId),
			MapItem: make(map[int32]int32),
		},
		Watermelon: &msg.DbWatermelon{
			RoleId:               int64(roleId),
			Snapshot:             &msg.WatermelonRecordSnapshot{},
			MapMergeRecord:       make(map[int32]int32),
			MapMergeInsideRecord: make(map[int32]int32),
			MapInsideItemCount:   make(map[int32]int32),
		},
	}
}

func (r *Mgr) initRole(userId string, role *msg.DbRole) {
	logger.Infof("initRole open_id[%s] role_id[%d]", userId, role.Base.RoleId)
	role.Base.Name = "player" + strconv.FormatInt(role.Base.RoleId, 10)
	if len(cfg.Tables().TbPlayerAvatar.GetDataList()) > 0 {
		role.Base.AvatarId = cfg.Tables().TbPlayerAvatar.GetDataList()[0].EAvatar
	}

	if len(cfg.Tables().TbPlayerFrame.GetDataList()) > 0 {
		role.Base.FrameId = cfg.Tables().TbPlayerFrame.GetDataList()[0].EFrame
	}
}

// LoadFromMongo 从MongoDB加载userIdMap映射和lockNextId
// 只恢复用户ID到角色ID的映射关系，不加载完整的Info数据
func (r *Mgr) LoadFromMongo(mongoClient *mongoop.MongoClient) error {
	// 保存mongoClient引用，供后续使用
	r.mongoClient = mongoClient

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db := mongoClient.C.Database(mongoClient.Cfg.Database)
	roleColl := db.Collection(CollectionRole)

	logger.Info("Starting to load userIdMap and lockNextId from MongoDB...")

	// 查询所有角色数据（只读取Role集合，获取userId和roleId的映射）
	cursor, err := roleColl.Find(ctx, bson.D{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	loadedCount := 0
	maxRoleId := int64(0)

	// 遍历所有角色数据
	for cursor.Next(ctx) {
		var roleData msg.DbRole
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

		roleIdInt64 := fw.ObjID(roleId)

		// 更新最大角色ID
		currentMax := r.lockNextId.Load()
		if roleIdInt64 > fw.ObjID(currentMax) {
			r.lockNextId.Store(int64(roleIdInt64))
		}

		// 只恢复userIdMap映射，不加载Info数据
		if roleData.UserId != "" {
			r.userIdMap.Set(roleData.UserId, roleIdInt64)
			if roleId > maxRoleId {
				maxRoleId = roleId
			}
			loadedCount++
		}
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	logger.Infof("Successfully loaded %d userId mappings from MongoDB", loadedCount)

	// 更新lockNextId，确保新创建的角色ID不会冲突
	if maxRoleId > 0 {
		r.lockNextId.Store(maxRoleId)
		logger.Infof("lockNextId set to %d", maxRoleId)
	}

	return nil
}

// loadInfoFromMongo 从MongoDB加载单个角色的Info数据
// 如果MongoDB中没有数据，返回nil
func (r *Mgr) loadInfoFromMongo(userId string, roleId fw.ObjID) *Role {
	if r.mongoClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := r.mongoClient.C.Database(r.mongoClient.Cfg.Database)
	roleIdStr := strconv.FormatInt(int64(roleId), 10)

	// 加载角色数据
	roleColl := db.Collection(CollectionRole)
	roleData := &msg.DbRole{}
	err := roleColl.FindOne(ctx, bson.D{{"_id", KeyPrefixRole + roleIdStr}}).Decode(roleData)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// MongoDB中没有数据，返回nil
			return nil
		}
		logger.Errorf("Failed to load role data for role_id=%d: %v", roleId, err)
		return nil
	}

	// 验证数据有效性
	if roleData.Base == nil || roleData.Base.RoleId != int64(roleId) {
		logger.Warnf("Invalid role data for role_id=%d", roleId)
		return nil
	}

	// 加载物品数据
	itemColl := db.Collection(CollectionItem)
	itemData := &msg.DbItem{}
	err = itemColl.FindOne(ctx, bson.D{{"_id", KeyPrefixItem + roleIdStr}}).Decode(itemData)
	if err != nil && err != mongo.ErrNoDocuments {
		logger.Errorf("Failed to load item data for role_id=%d: %v", roleId, err)
		return nil
	}
	if err == mongo.ErrNoDocuments {
		// 如果没有物品数据，创建默认的
		itemData = &msg.DbItem{
			RoleId:  int64(roleId),
			MapItem: make(map[int32]int32),
		}
	}

	// 加载西瓜数据
	watermelonColl := db.Collection(CollectionWatermelon)
	watermelonData := &msg.DbWatermelon{}
	err = watermelonColl.FindOne(ctx, bson.D{{"_id", KeyPrefixWatermelon + roleIdStr}}).Decode(watermelonData)
	if err != nil && err != mongo.ErrNoDocuments {
		logger.Errorf("Failed to load watermelon data for role_id=%d: %v", roleId, err)
		return nil
	}
	if err == mongo.ErrNoDocuments {
		// 如果没有西瓜数据，创建默认的
		watermelonData = &msg.DbWatermelon{
			RoleId:               int64(roleId),
			Snapshot:             &msg.WatermelonRecordSnapshot{},
			MapMergeRecord:       make(map[int32]int32),
			MapMergeInsideRecord: make(map[int32]int32),
			MapInsideItemCount:   make(map[int32]int32),
		}
	}

	// 创建角色信息
	info := &Role{
		sId:        strconv.FormatInt(int64(roleId), 10),
		Role:       roleData,
		Item:       itemData,
		Watermelon: watermelonData,
	}
	info.dirty.Store(false) // 从数据库加载的数据不需要保存

	logger.Infof("Loaded role data from MongoDB for userId=%s, roleId=%d", userId, roleId)
	return info
}

// RegisterPersistFunc 注册角色的保存函数到持久化管理器
// 保存 persistMgr 的引用，以便在 WriteRole 中使用
func (r *Mgr) RegisterPersistFunc(persistMgr *persistence.PersistManager) {
	r.persistMgr = persistMgr
}
