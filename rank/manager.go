package rank

import (
	"sync"
	"sync/atomic"
	"time"
	"wx_game/fw/mdzset"
	"wx_game/fw/persistence"

	"github.com/donnie4w/go-logger/logger"
	"go.mongodb.org/mongo-driver/mongo"
)

// KT 键类型约束（与 mdzset.KT 一致）
type KT interface {
	int64 | string
}

// Config 排行榜配置
type Config struct {
	Name        string // 排行榜名称
	Collection  string // MongoDB集合名称
	Key         string // 持久化管理器中的唯一标识
	Dimensional int    // 排行榜维度（分数字段数量）
	Capacity    int    // 排行榜容量
}

// DbRankItem 排行榜数据库存储结构（通用）
// K 是键的类型（int64 或 string）
// A 是附加数据的类型
type DbRankItem[K KT, A any] struct {
	Key        K         `bson:"key"`
	Scores     []float64 `bson:"scores"` // 多维分数数组
	Attachment A         `bson:"attachment"`
}

// Manager 通用排行榜管理器
// K 是键的类型（int64 或 string）
// A 是附加数据的类型，需要支持 BSON 序列化
// 实现 persistence.Saveable 接口
// 注意：mdzset.SortedSet 内部已有锁，无需额外加锁
type Manager[K KT, A any] struct {
	config     Config
	rank       *mdzset.SortedSet[K]
	persistMgr *persistence.PersistManager
	dirty      atomic.Bool            // 标记是否有数据变更需要保存
	dirtyItems map[K]*dirtyItem[K, A] // 脏数据列表（需要保存的项）
	dirtyMu    sync.Mutex             // 脏数据列表的锁
}

// dirtyItem 脏数据项
type dirtyItem[K KT, A any] struct {
	key        K
	scores     []float64
	attachment A
}

// New 创建排行榜管理器
func New[K KT, A any](config Config) *Manager[K, A] {
	return &Manager[K, A]{
		config:     config,
		rank:       mdzset.NewWithFixedSize[K](config.Name, config.Dimensional, uint64(config.Capacity)),
		dirtyItems: make(map[K]*dirtyItem[K, A]),
	}
}

// Init 初始化排行榜管理器并注册到持久化管理器
func (m *Manager[K, A]) Init(persistMgr *persistence.PersistManager) error {
	m.persistMgr = persistMgr

	// 从MongoDB加载排行榜数据
	if err := m.LoadFromMongo(); err != nil {
		return err
	}

	return nil
}

// GetConfig 获取配置
func (m *Manager[K, A]) GetConfig() Config {
	return m.config
}

// IsValid 实现 Saveable 接口 - 判断是否有效
func (m *Manager[K, A]) IsValid() bool {
	return m.rank != nil
}

// Save 实现 Saveable 接口 - 返回需要保存的数据列表
func (m *Manager[K, A]) Save() ([]persistence.SaveData, error) {
	// 检查是否需要保存
	if !m.dirty.CompareAndSwap(true, false) {
		return []persistence.SaveData{}, nil
	}

	// 获取并清空脏数据列表
	m.dirtyMu.Lock()
	items := m.dirtyItems
	m.dirtyItems = make(map[K]*dirtyItem[K, A])
	m.dirtyMu.Unlock()

	if len(items) == 0 {
		return []persistence.SaveData{}, nil
	}

	var saveDataList []persistence.SaveData

	for key, item := range items {
		dbItem := DbRankItem[K, A]{
			Key:        item.key,
			Scores:     item.scores,
			Attachment: item.attachment,
		}

		// 捕获当前 item 的值，用于失败时恢复
		failedItem := item
		onSaveFailure := func() {
			// 保存失败，把该项重新加回脏数据列表
			m.dirtyMu.Lock()
			m.dirtyItems[failedItem.key] = failedItem
			m.dirtyMu.Unlock()
			m.dirty.Store(true)
		}

		saveDataList = append(saveDataList, persistence.SaveData{
			Collection: m.config.Collection,
			ID:         key, // 使用 key 作为 _id
			Data:       dbItem,
			OnFailure:  onSaveFailure,
		})
	}

	logger.Debugf("Rank[%s] save: %d items to save", m.config.Name, len(saveDataList))
	return saveDataList, nil
}

// LoadFromMongo 从MongoDB加载排行榜数据
func (m *Manager[K, A]) LoadFromMongo() error {
	if m.persistMgr == nil {
		logger.Warnf("Rank[%s]: PersistManager is nil, skipping rank data loading", m.config.Name)
		return nil
	}

	logger.Infof("Rank[%s]: Starting to load rank data from MongoDB...", m.config.Name)

	loadedCount, err := m.persistMgr.LoadFromMongo(
		m.config.Collection,
		nil, // 查询全部
		60*time.Second,
		func(cursor *mongo.Cursor) error {
			var item DbRankItem[K, A]
			if err := cursor.Decode(&item); err != nil {
				return err
			}

			// 验证分数维度
			if len(item.Scores) != m.config.Dimensional {
				logger.Warnf("Rank[%s]: Invalid scores dimension for key=%v: expected %d, got %d, skipping",
					m.config.Name, item.Key, m.config.Dimensional, len(item.Scores))
				return nil
			}

			// 添加到内存排行榜
			err := m.rank.Add(item.Scores, item.Key, item.Attachment)
			if err != nil {
				logger.Errorf("Rank[%s]: Failed to add rank item for key=%v: %v", m.config.Name, item.Key, err)
				return nil // 添加失败也跳过，不中断加载
			}
			return nil
		},
	)

	if err != nil {
		return err
	}

	logger.Infof("Rank[%s]: Successfully loaded %d rank items from MongoDB", m.config.Name, loadedCount)
	return nil
}

// markDirty 标记数据变更
func (m *Manager[K, A]) markDirty(key K, scores []float64, attachment A) {
	// 复制 scores 以避免外部修改
	scoresCopy := make([]float64, len(scores))
	copy(scoresCopy, scores)

	m.dirtyMu.Lock()
	m.dirtyItems[key] = &dirtyItem[K, A]{
		key:        key,
		scores:     scoresCopy,
		attachment: attachment,
	}
	m.dirtyMu.Unlock()

	m.dirty.Store(true)

	// 添加到持久化管理器
	if m.persistMgr != nil {
		m.persistMgr.AddPendingObject(m.config.Key, m)
	}
}

// Add 添加或更新排行榜数据
// scores 的长度必须等于配置的 Dimensional
func (m *Manager[K, A]) Add(key K, scores []float64, attachment A) error {
	if len(scores) != m.config.Dimensional {
		logger.Errorf("Rank[%s]: Invalid scores dimension: expected %d, got %d", m.config.Name, m.config.Dimensional, len(scores))
		return nil
	}

	// mdzset.SortedSet.Add 内部已加锁
	err := m.rank.Add(scores, key, attachment)
	if err != nil {
		return err
	}

	// 标记脏数据
	m.markDirty(key, scores, attachment)

	return nil
}

// RefreshEntry 刷新排行榜条目（例如角色登入时调用）
// 如果条目已存在，更新附加数据；如果不存在但有历史分数，尝试添加
// key 和 attachment 参数类型为 any，用于实现通用的 RankRefresher 接口
func (m *Manager[K, A]) RefreshEntry(key any, historyScores []float64, attachment any) {
	// 类型断言
	typedKey, ok := key.(K)
	if !ok {
		logger.Errorf("Rank[%s]: Invalid key type", m.config.Name)
		return
	}

	typedAttachment, ok := attachment.(A)
	if !ok {
		logger.Errorf("Rank[%s]: Invalid attachment type for key=%v", m.config.Name, typedKey)
		return
	}

	// mdzset.SortedSet 方法内部已加锁
	// 检查是否存在于排行榜中
	if m.rank.Contains(typedKey) {
		// 更新附加数据
		m.rank.UpdateAttachment(typedKey, typedAttachment)

		// 获取当前分数
		scores, ok := m.rank.Score(typedKey)
		if ok {
			m.markDirty(typedKey, scores, typedAttachment)
		}
		logger.Debugf("Rank[%s]: Refreshed entry for key=%v", m.config.Name, typedKey)
	} else if len(historyScores) == m.config.Dimensional && !isZeroScores(historyScores) {
		// 不在排行榜中但有历史分数，尝试添加
		err := m.rank.Add(historyScores, typedKey, typedAttachment)
		if err != nil {
			logger.Errorf("Rank[%s]: Failed to add entry on refresh: key=%v, err=%v", m.config.Name, typedKey, err)
			return
		}
		m.markDirty(typedKey, historyScores, typedAttachment)
		logger.Debugf("Rank[%s]: Added entry on refresh: key=%v, scores=%v", m.config.Name, typedKey, historyScores)
	}
}

// isZeroScores 检查分数是否全为零
func isZeroScores(scores []float64) bool {
	for _, s := range scores {
		if s != 0 {
			return false
		}
	}
	return true
}

// Len 获取排行榜长度
func (m *Manager[K, A]) Len() int {
	return m.rank.Len()
}

// Range 获取排行榜范围数据
func (m *Manager[K, A]) Range(start, end int) []mdzset.Item[K] {
	return m.rank.Range(start, end)
}

// Rank 获取指定键的排名
func (m *Manager[K, A]) Rank(key K, reverse bool) (int, mdzset.Scores, any) {
	return m.rank.Rank(key, reverse)
}

// Contains 检查是否包含指定键
func (m *Manager[K, A]) Contains(key K) bool {
	return m.rank.Contains(key)
}

// Score 获取指定键的分数
func (m *Manager[K, A]) Score(key K) (mdzset.Scores, bool) {
	return m.rank.Score(key)
}
