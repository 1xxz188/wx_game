package persistence

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"wx_game/fw/persistence/mongoop"

	"github.com/donnie4w/go-logger/logger"
	cmap "github.com/orcaman/concurrent-map/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SaveData 保存数据结构
type SaveData struct {
	Collection string
	ID         string
	Data       interface{}
	OnSuccess  func() // 保存成功后的回调函数
	OnFailure  func() // 保存失败后的回调函数
}

// Saveable 可保存对象接口
// 实现此接口的对象可以被添加到待保存列表中
type Saveable interface {
	// Key 返回对象的唯一标识符
	Key() string
	// Save 返回需要保存的数据列表
	Save() ([]SaveData, error)
	// IsValid 判断对象是否有效，保存前需要先判断
	IsValid() bool
}

// PersistManager 持久化管理器
type PersistManager struct {
	mongoClient    *mongoop.MongoClient
	interval       time.Duration
	pendingObjects cmap.ConcurrentMap[string, Saveable] // 待保存对象列表
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
	running        bool
	maxSaveTime    atomic.Int64 // 保存最大执行时间（纳秒）
}

// NewPersistManager 创建持久化管理器
func NewPersistManager(mongoClient *mongoop.MongoClient, interval time.Duration) *PersistManager {
	return &PersistManager{
		mongoClient:    mongoClient,
		interval:       interval,
		pendingObjects: cmap.New[Saveable](),
		stopChan:       make(chan struct{}),
		running:        false,
	}
}

// AddPendingObject 添加待保存对象到列表
// 调用者需要传入 key（对象的唯一标识符），避免在持有锁时调用 Key() 方法导致死锁
func (pm *PersistManager) AddPendingObject(key string, obj Saveable) {
	pm.pendingObjects.Set(key, obj)
}

// Start 启动定时落库
func (pm *PersistManager) Start() {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.mu.Unlock()

	logger.Infof("Persistence module started, interval: %v", pm.interval)

	pm.wg.Add(1)
	go pm.run()
}

// Stop 停止定时落库并立即执行一次落库
func (pm *PersistManager) Stop() {
	pm.mu.Lock()
	if !pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = false
	pm.mu.Unlock()

	// 停止定时器
	close(pm.stopChan)
	pm.wg.Wait()

	logger.Info("Persistence module stopped completely")
}

// Flush 立即执行一次落库
func (pm *PersistManager) Flush() {
	pm.flush()
}

// run 定时执行落库
func (pm *PersistManager) run() {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	pm.flush()

	for {
		select {
		case <-ticker.C:
			pm.flush()
		case <-pm.stopChan: // 立即执行一次落库
			logger.Info("Persistence module stopping, performing final flush...")
			pm.flush()
			return
		}
	}
}

// flush 执行所有注册的保存函数
func (pm *PersistManager) flush() {
	// 获取待保存对象列表（同时记录 key，避免后续调用 Key() 方法）
	type pendingItem struct {
		key string
		obj Saveable
	}
	var pendingObjects []pendingItem
	pm.pendingObjects.IterCb(func(key string, obj Saveable) {
		pendingObjects = append(pendingObjects, pendingItem{key: key, obj: obj})
	})
	// 使用遍历时记录的 key 删除，避免调用 obj.Key() 导致潜在的锁等待
	for _, item := range pendingObjects {
		pm.pendingObjects.Remove(item.key)
	}

	if len(pendingObjects) == 0 {
		return
	}

	logger.Infof("Starting persistence flush, total tasks: %d pending objects", len(pendingObjects))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	successCount := 0
	failCount := 0

	// 处理待保存对象列表
	for i, item := range pendingObjects {
		obj := item.obj
		// 先判断对象是否有效
		if !obj.IsValid() {
			logger.Warnf("Pending object [%d/%d] is invalid, skipping", i+1, len(pendingObjects))
			continue
		}

		// 调用保存接口
		saveDataList, err := obj.Save()
		if err != nil {
			logger.Errorf("Pending object save failed [%d/%d]: %v", i+1, len(pendingObjects), err)
			failCount++
			continue
		}

		// 在PersistManager内部处理保存
		for j, saveData := range saveDataList {
			if saveData.Collection == "" || saveData.ID == "" {
				continue
			}

			err = pm.saveToMongo(ctx, saveData.Collection, saveData.ID, saveData.Data)
			if err != nil {
				logger.Errorf("Failed to save data to MongoDB [%d/%d] item[%d/%d] collection=%s, id=%s: %v", i+1, len(pendingObjects), j+1, len(saveDataList), saveData.Collection, saveData.ID, err)
				failCount++
				// 保存失败后调用回调函数（如果存在）
				if saveData.OnFailure != nil {
					saveData.OnFailure()
				}
			} else {
				successCount++
				// 保存成功后调用回调函数（如果存在）
				if saveData.OnSuccess != nil {
					saveData.OnSuccess()
				}
			}
		}

		logger.Infof("Pending object save completed [%d/%d], processed %d items", i+1, len(pendingObjects), len(saveDataList))
	}

	logger.Infof("Persistence flush completed, success: %d, failed: %d", successCount, failCount)
}

// saveToMongo 保存数据到MongoDB
func (pm *PersistManager) saveToMongo(ctx context.Context, collection, id string, data interface{}) error {
	if pm.mongoClient == nil {
		return nil // 如果没有配置MongoDB，静默跳过
	}

	// 记录开始时间
	startTime := time.Now()

	db := pm.mongoClient.C.Database(pm.mongoClient.Cfg.Database)
	coll := db.Collection(collection)

	filter := bson.D{{"_id", id}}
	update := bson.D{{"$set", data}}
	opts := options.Update().SetUpsert(true)

	_, err := coll.UpdateOne(ctx, filter, update, opts)

	// 计算执行时间
	duration := time.Since(startTime)
	durationNs := duration.Nanoseconds()

	// 更新最大执行时间（使用原子操作）
	for {
		currentMax := pm.maxSaveTime.Load()
		if durationNs <= currentMax {
			break // 没有超过当前最大值
		}
		// 尝试更新最大值
		if pm.maxSaveTime.CompareAndSwap(currentMax, durationNs) {
			// 更新成功，记录日志
			logger.Infof("New max saveToMongo time: %v (collection=%s, id=%s), previous max: %v",
				duration, collection, id, time.Duration(currentMax))
			break
		}
		// 更新失败，说明有其他goroutine已经更新了，重新检查
	}

	return err
}
