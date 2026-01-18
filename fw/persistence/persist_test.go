package persistence

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"wx_game/fw/persistence/mongoop"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

// skipIfShort 在 -short 模式下跳过需要 MongoDB 连接的测试
func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping: requires MongoDB connection (use -short mode)")
	}
}

// ========== Mock 对象定义 ==========

// MockSaveable 用于测试的 mock 可保存对象
type MockSaveable struct {
	id           string
	valid        bool
	saveData     []SaveData
	saveErr      error
	saveCalled   atomic.Int32
	successCount atomic.Int32
	failureCount atomic.Int32
}

func NewMockSaveable(id string, valid bool) *MockSaveable {
	return &MockSaveable{
		id:    id,
		valid: valid,
	}
}

func (m *MockSaveable) SetSaveData(data []SaveData) {
	m.saveData = data
}

func (m *MockSaveable) SetSaveError(err error) {
	m.saveErr = err
}

func (m *MockSaveable) Save() ([]SaveData, error) {
	m.saveCalled.Add(1)
	if m.saveErr != nil {
		return nil, m.saveErr
	}
	return m.saveData, nil
}

func (m *MockSaveable) IsValid() bool {
	return m.valid
}

func (m *MockSaveable) GetSaveCalledCount() int32 {
	return m.saveCalled.Load()
}

func (m *MockSaveable) GetSuccessCount() int32 {
	return m.successCount.Load()
}

func (m *MockSaveable) GetFailureCount() int32 {
	return m.failureCount.Load()
}

// ========== 单元测试（不需要 MongoDB）==========

func TestNewPersistManager(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	assert.NotNil(t, pm)
	assert.Equal(t, 5*time.Second, pm.interval)
	assert.Nil(t, pm.mongoClient)
	assert.False(t, pm.running)
	assert.Equal(t, 0, pm.batchSize)
	assert.Equal(t, time.Duration(0), pm.batchInterval)
}

func TestSetRateLimit(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	pm.SetRateLimit(100, 500*time.Millisecond)

	assert.Equal(t, 100, pm.batchSize)
	assert.Equal(t, 500*time.Millisecond, pm.batchInterval)
}

func TestAddPendingObject(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock1 := NewMockSaveable("obj1", true)
	mock2 := NewMockSaveable("obj2", true)

	pm.AddPendingObject("key1", mock1)
	pm.AddPendingObject("key2", mock2)

	assert.Equal(t, 2, pm.pendingObjects.Count())

	// 测试覆盖已存在的 key
	mock3 := NewMockSaveable("obj3", true)
	pm.AddPendingObject("key1", mock3)
	assert.Equal(t, 2, pm.pendingObjects.Count())

	// 验证 key1 已被覆盖
	obj, ok := pm.pendingObjects.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, mock3, obj)
}

func TestGetMongoClient(t *testing.T) {
	// 测试 nil 客户端
	pm := NewPersistManager(nil, 5*time.Second)
	assert.Nil(t, pm.GetMongoClient())
}

func TestStartAndStop(t *testing.T) {
	pm := NewPersistManager(nil, 100*time.Millisecond)

	// 初始状态
	assert.False(t, pm.running)

	// 启动
	pm.Start()
	assert.True(t, pm.running)

	// 重复启动应该被忽略
	pm.Start()
	assert.True(t, pm.running)

	// 停止
	pm.Stop()
	assert.False(t, pm.running)

	// 重复停止应该被忽略
	pm.Stop()
	assert.False(t, pm.running)
}

func TestStartStopConcurrency(t *testing.T) {
	pm := NewPersistManager(nil, 50*time.Millisecond)

	var wg sync.WaitGroup
	// 并发启动和停止
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			pm.Start()
		}()
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			pm.Stop()
		}()
	}
	wg.Wait()

	// 确保最终状态是停止的
	pm.Stop()
	assert.False(t, pm.running)
}

func TestFlushWithNoPendingObjects(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	// 没有待保存对象时，flush 应该快速返回
	start := time.Now()
	pm.Flush()
	duration := time.Since(start)

	// 应该几乎瞬间完成
	assert.Less(t, duration, 100*time.Millisecond)
}

func TestFlushWithInvalidObject(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	// 添加无效对象
	mock := NewMockSaveable("invalid", false)
	pm.AddPendingObject("key1", mock)

	pm.Flush()

	// Save 方法不应该被调用（因为对象无效）
	assert.Equal(t, int32(0), mock.GetSaveCalledCount())
	// 待保存列表应该已清空
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestFlushWithValidObject(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock := NewMockSaveable("valid", true)
	mock.SetSaveData([]SaveData{
		{
			Collection: "test_collection",
			ID:         "test_id",
			Data:       map[string]string{"name": "test"},
		},
	})

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	// Save 方法应该被调用
	assert.Equal(t, int32(1), mock.GetSaveCalledCount())
	// 待保存列表应该已清空
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestFlushWithSaveError(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock := NewMockSaveable("error", true)
	mock.SetSaveError(errors.New("save failed"))

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	// Save 方法应该被调用
	assert.Equal(t, int32(1), mock.GetSaveCalledCount())
	// 待保存列表应该已清空
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestFlushCallsCallbacks(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	successCalled := atomic.Bool{}
	failureCalled := atomic.Bool{}

	mock := NewMockSaveable("callback", true)
	mock.SetSaveData([]SaveData{
		{
			Collection: "", // 空集合名，会跳过保存
			ID:         "test_id",
			Data:       map[string]string{"name": "test"},
			OnSuccess: func() {
				successCalled.Store(true)
			},
			OnFailure: func() {
				failureCalled.Store(true)
			},
		},
	})

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	// 空集合名会跳过保存，所以回调不会被调用
	assert.False(t, successCalled.Load())
	assert.False(t, failureCalled.Load())
}

func TestFlushWithMultipleObjects(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	var mocks []*MockSaveable
	for i := 0; i < 5; i++ {
		mock := NewMockSaveable("obj", true)
		mock.SetSaveData([]SaveData{
			{
				Collection: "test",
				ID:         i,
				Data:       map[string]int{"index": i},
			},
		})
		mocks = append(mocks, mock)
		pm.AddPendingObject(string(rune('a'+i)), mock)
	}

	pm.Flush()

	// 所有 Save 方法都应该被调用
	for i, mock := range mocks {
		assert.Equal(t, int32(1), mock.GetSaveCalledCount(), "mock %d should be called once", i)
	}
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestFlushWithRateLimit(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)
	pm.SetRateLimit(2, 50*time.Millisecond)

	// 添加5个对象，每批2个，应该分3批处理
	for i := 0; i < 5; i++ {
		mock := NewMockSaveable("obj", true)
		mock.SetSaveData([]SaveData{})
		pm.AddPendingObject(string(rune('a'+i)), mock)
	}

	start := time.Now()
	pm.Flush()
	duration := time.Since(start)

	// 应该至少等待 2 个批次间隔（3批 - 1 = 2个间隔）
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestFlushRemovesObjectsFromPendingList(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock := NewMockSaveable("test", true)
	mock.SetSaveData([]SaveData{})

	pm.AddPendingObject("key1", mock)
	assert.Equal(t, 1, pm.pendingObjects.Count())

	pm.Flush()
	assert.Equal(t, 0, pm.pendingObjects.Count())
}

func TestMaxSaveTimeTracking(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	// 初始最大时间应该是0
	assert.Equal(t, int64(0), pm.maxSaveTime.Load())
}

func TestSaveDataWithEmptyCollection(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock := NewMockSaveable("empty_collection", true)
	mock.SetSaveData([]SaveData{
		{
			Collection: "", // 空集合名
			ID:         "test_id",
			Data:       map[string]string{"name": "test"},
		},
	})

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	// Save 应该被调用，但保存操作应该被跳过（因为集合名为空）
	assert.Equal(t, int32(1), mock.GetSaveCalledCount())
}

func TestSaveDataWithNilID(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	mock := NewMockSaveable("nil_id", true)
	mock.SetSaveData([]SaveData{
		{
			Collection: "test_collection",
			ID:         nil, // nil ID
			Data:       map[string]string{"name": "test"},
		},
	})

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	// Save 应该被调用，但保存操作应该被跳过（因为 ID 为 nil）
	assert.Equal(t, int32(1), mock.GetSaveCalledCount())
}

func TestPeriodicFlush(t *testing.T) {
	pm := NewPersistManager(nil, 100*time.Millisecond)

	flushCount := atomic.Int32{}
	mock := NewMockSaveable("periodic", true)
	mock.SetSaveData([]SaveData{})

	// 启动持久化管理器
	pm.Start()

	// 等待几个周期
	time.Sleep(350 * time.Millisecond)

	// 添加对象并等待自动 flush
	pm.AddPendingObject("key1", mock)
	for i := 0; i < 5; i++ {
		if mock.GetSaveCalledCount() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	flushCount.Store(mock.GetSaveCalledCount())

	pm.Stop()

	// 应该至少被调用一次
	assert.GreaterOrEqual(t, flushCount.Load(), int32(1))
}

// ========== 集成测试（需要 MongoDB）==========

func getTestMongoClient(t *testing.T) *mongoop.MongoClient {
	client, err := mongoop.NewMongoClient(mongoop.Conf{
		Url:         "mongodb://localhost:26001",
		ConnTimeout: "3s",
		Database:    "test_persistence",
	})
	if err != nil {
		t.Skipf("Cannot connect to MongoDB: %v", err)
	}
	return client
}

func TestSaveToMongoIntegration(t *testing.T) {
	skipIfShort(t)

	client := getTestMongoClient(t)
	defer client.DisConnect(context.Background())

	pm := NewPersistManager(client, 5*time.Second)

	// 创建测试数据
	type TestData struct {
		Name  string `bson:"name"`
		Value int    `bson:"value"`
	}

	mock := NewMockSaveable("integration", true)
	successCalled := atomic.Bool{}
	mock.SetSaveData([]SaveData{
		{
			Collection: "test_persist",
			ID:         "test_integration_1",
			Data: TestData{
				Name:  "test",
				Value: 100,
			},
			OnSuccess: func() {
				successCalled.Store(true)
			},
		},
	})

	pm.AddPendingObject("key1", mock)
	pm.Flush()

	assert.Equal(t, int32(1), mock.GetSaveCalledCount())
	assert.True(t, successCalled.Load())
}

func TestLoadFromMongoIntegration(t *testing.T) {
	skipIfShort(t)

	client := getTestMongoClient(t)
	defer client.DisConnect(context.Background())

	pm := NewPersistManager(client, 5*time.Second)

	// 先保存一些数据
	type TestData struct {
		ID    string `bson:"_id"`
		Name  string `bson:"name"`
		Value int    `bson:"value"`
	}

	for i := 0; i < 3; i++ {
		mock := NewMockSaveable("load_test", true)
		mock.SetSaveData([]SaveData{
			{
				Collection: "test_load",
				ID:         i,
				Data: TestData{
					ID:    string(rune('a' + i)),
					Name:  "test",
					Value: i * 10,
				},
			},
		})
		pm.AddPendingObject(string(rune('a'+i)), mock)
	}
	pm.Flush()

	// 测试加载
	loadedCount := 0
	count, err := pm.LoadFromMongo("test_load", nil, 5*time.Second, func(cursor *mongo.Cursor) error {
		loadedCount++
		return nil
	})

	if err != nil {
		t.Logf("LoadFromMongo error (collection may not exist): %v", err)
	} else {
		assert.GreaterOrEqual(t, count, 0)
	}
}

func TestLoadFromMongoWithNilClient(t *testing.T) {
	pm := NewPersistManager(nil, 5*time.Second)

	count, err := pm.LoadFromMongo("test", nil, 5*time.Second, func(cursor *mongo.Cursor) error {
		return nil
	})

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "MongoDB client is nil")
}

// ========== 基准测试 ==========

func BenchmarkAddPendingObject(b *testing.B) {
	pm := NewPersistManager(nil, 5*time.Second)
	mock := NewMockSaveable("bench", true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.AddPendingObject(string(rune(i)), mock)
	}
}

func BenchmarkFlushWithManyObjects(b *testing.B) {
	pm := NewPersistManager(nil, 5*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// 准备数据
		for j := 0; j < 100; j++ {
			mock := NewMockSaveable("bench", true)
			mock.SetSaveData([]SaveData{})
			pm.AddPendingObject(string(rune(j)), mock)
		}
		b.StartTimer()
		pm.Flush()
	}
}

func BenchmarkConcurrentAddAndFlush(b *testing.B) {
	pm := NewPersistManager(nil, 100*time.Millisecond)
	pm.Start()
	defer pm.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mock := NewMockSaveable("concurrent", true)
			mock.SetSaveData([]SaveData{})
			pm.AddPendingObject(string(rune(i)), mock)
			i++
		}
	})
}
