package rank

import (
	"testing"
)

// 辅助函数：检查错误
func noErr(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// 测试附加数据结构
type TestAttachment struct {
	Name  string
	Level int
}

// 创建测试用的 Manager
func newTestManager[K KT, A any](name string, dimensional int, capacity int) *Manager[K, A] {
	config := Config{
		Name:        name,
		Collection:  "test_" + name,
		Key:         "test_key_" + name,
		Dimensional: dimensional,
		Capacity:    capacity,
	}
	return New[K, A](config)
}

func TestNew(t *testing.T) {
	m := newTestManager[int64, string]("test_rank", 2, 100)
	if m == nil {
		t.Fatal("Manager should not be nil")
	}
	if m.rank == nil {
		t.Fatal("rank should not be nil")
	}
	if m.config.Name != "test_rank" {
		t.Errorf("expected name 'test_rank', got '%s'", m.config.Name)
	}
	if m.config.Dimensional != 2 {
		t.Errorf("expected dimensional 2, got %d", m.config.Dimensional)
	}
	if m.config.Capacity != 100 {
		t.Errorf("expected capacity 100, got %d", m.config.Capacity)
	}
}

func TestAdd(t *testing.T) {
	m := newTestManager[int64, string]("test_add", 2, 10)

	// 测试正常添加
	err := m.Add(1001, []float64{100, 50}, "Player1")
	noErr(t, err)

	err = m.Add(1002, []float64{200, 30}, "Player2")
	noErr(t, err)

	err = m.Add(1003, []float64{150, 40}, "Player3")
	noErr(t, err)

	if m.Len() != 3 {
		t.Errorf("expected length 3, got %d", m.Len())
	}
}

func TestAddWithWrongDimension(t *testing.T) {
	m := newTestManager[int64, string]("test_wrong_dim", 2, 10)

	// 测试维度错误（不会返回错误，但会记录日志）
	err := m.Add(1001, []float64{100}, "Player1") // 只有1维，需要2维
	if err != nil {
		t.Errorf("Add with wrong dimension should not return error, but got: %v", err)
	}

	// 数据不应该被添加
	if m.Len() != 0 {
		t.Errorf("expected length 0 after adding wrong dimension, got %d", m.Len())
	}

	// 测试维度过多
	err = m.Add(1002, []float64{100, 50, 30}, "Player2") // 3维，需要2维
	if err != nil {
		t.Errorf("Add with wrong dimension should not return error, but got: %v", err)
	}

	if m.Len() != 0 {
		t.Errorf("expected length 0 after adding wrong dimension, got %d", m.Len())
	}
}

func TestContains(t *testing.T) {
	m := newTestManager[int64, string]("test_contains", 1, 10)

	err := m.Add(1001, []float64{100}, "Player1")
	noErr(t, err)

	if !m.Contains(1001) {
		t.Error("should contain key 1001")
	}

	if m.Contains(9999) {
		t.Error("should not contain key 9999")
	}
}

func TestScore(t *testing.T) {
	m := newTestManager[int64, string]("test_score", 2, 10)

	err := m.Add(1001, []float64{100, 50}, "Player1")
	noErr(t, err)

	scores, ok := m.Score(1001)
	if !ok {
		t.Fatal("should find score for key 1001")
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	if scores[0] != 100 {
		t.Errorf("expected first score 100, got %f", scores[0])
	}
	if scores[1] != 50 {
		t.Errorf("expected second score 50, got %f", scores[1])
	}

	// 测试不存在的 key
	_, ok = m.Score(9999)
	if ok {
		t.Error("should not find score for non-existent key")
	}
}

func TestRank(t *testing.T) {
	m := newTestManager[int64, string]("test_rank_query", 1, 10)

	// 添加数据（分数从大到小排序）
	err := m.Add(1001, []float64{100}, "Player1")
	noErr(t, err)
	err = m.Add(1002, []float64{200}, "Player2")
	noErr(t, err)
	err = m.Add(1003, []float64{150}, "Player3")
	noErr(t, err)

	// 排名：200(1002) > 150(1003) > 100(1001)
	// 正向排序：1002=0, 1003=1, 1001=2
	rank, scores, attachment := m.Rank(1002, false)
	if rank != 0 {
		t.Errorf("expected rank 0 for key 1002, got %d", rank)
	}
	if scores[0] != 200 {
		t.Errorf("expected score 200, got %f", scores[0])
	}
	if attachment != "Player2" {
		t.Errorf("expected attachment 'Player2', got %v", attachment)
	}

	rank, _, _ = m.Rank(1003, false)
	if rank != 1 {
		t.Errorf("expected rank 1 for key 1003, got %d", rank)
	}

	rank, _, _ = m.Rank(1001, false)
	if rank != 2 {
		t.Errorf("expected rank 2 for key 1001, got %d", rank)
	}

	// 测试不存在的 key
	rank, _, _ = m.Rank(9999, false)
	if rank != -1 {
		t.Errorf("expected rank -1 for non-existent key, got %d", rank)
	}
}

func TestRange(t *testing.T) {
	m := newTestManager[int64, string]("test_range", 1, 10)

	err := m.Add(1001, []float64{100}, "Player1")
	noErr(t, err)
	err = m.Add(1002, []float64{200}, "Player2")
	noErr(t, err)
	err = m.Add(1003, []float64{150}, "Player3")
	noErr(t, err)
	err = m.Add(1004, []float64{50}, "Player4")
	noErr(t, err)

	// 获取前3名
	items := m.Range(0, 2)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// 验证顺序：200(1002) > 150(1003) > 100(1001)
	if items[0].Key != 1002 {
		t.Errorf("expected first key 1002, got %d", items[0].Key)
	}
	if items[1].Key != 1003 {
		t.Errorf("expected second key 1003, got %d", items[1].Key)
	}
	if items[2].Key != 1001 {
		t.Errorf("expected third key 1001, got %d", items[2].Key)
	}

	// 获取全部（使用 -1）
	allItems := m.Range(0, -1)
	if len(allItems) != 4 {
		t.Errorf("expected 4 items, got %d", len(allItems))
	}
}

func TestGetConfig(t *testing.T) {
	m := newTestManager[int64, string]("test_config", 3, 50)

	config := m.GetConfig()
	if config.Name != "test_config" {
		t.Errorf("expected name 'test_config', got '%s'", config.Name)
	}
	if config.Dimensional != 3 {
		t.Errorf("expected dimensional 3, got %d", config.Dimensional)
	}
	if config.Capacity != 50 {
		t.Errorf("expected capacity 50, got %d", config.Capacity)
	}
}

func TestIsValid(t *testing.T) {
	m := newTestManager[int64, string]("test_valid", 1, 10)

	if !m.IsValid() {
		t.Error("Manager should be valid after creation")
	}

	// 创建一个无效的 Manager（直接赋值 nil）
	m2 := &Manager[int64, string]{}
	if m2.IsValid() {
		t.Error("Manager with nil rank should be invalid")
	}
}

func TestSave(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_save", 2, 10)

	// 没有脏数据时，Save 应该返回空列表
	saveData, err := m.Save()
	noErr(t, err)
	if len(saveData) != 0 {
		t.Errorf("expected 0 save data when no dirty data, got %d", len(saveData))
	}

	// 添加数据（会标记为脏数据）
	err = m.Add(1001, []float64{100, 50}, TestAttachment{Name: "Player1", Level: 10})
	noErr(t, err)
	err = m.Add(1002, []float64{200, 30}, TestAttachment{Name: "Player2", Level: 20})
	noErr(t, err)

	// Save 应该返回脏数据
	saveData, err = m.Save()
	noErr(t, err)
	if len(saveData) != 2 {
		t.Errorf("expected 2 save data, got %d", len(saveData))
	}

	// 验证保存数据的内容
	for _, sd := range saveData {
		if sd.Collection != "test_test_save" {
			t.Errorf("expected collection 'test_test_save', got '%s'", sd.Collection)
		}
		dbItem, ok := sd.Data.(DbRankItem[int64, TestAttachment])
		if !ok {
			t.Fatal("data should be DbRankItem type")
		}
		if dbItem.Key != 1001 && dbItem.Key != 1002 {
			t.Errorf("unexpected key: %d", dbItem.Key)
		}
	}

	// 再次 Save 应该返回空（脏数据已清除）
	saveData, err = m.Save()
	noErr(t, err)
	if len(saveData) != 0 {
		t.Errorf("expected 0 save data after save, got %d", len(saveData))
	}
}

func TestRefreshEntry_Existing(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_existing", 2, 10)

	// 先添加数据
	err := m.Add(1001, []float64{100, 50}, TestAttachment{Name: "Player1", Level: 10})
	noErr(t, err)

	// 清除脏数据标记
	m.Save()

	// 刷新已存在的条目（更新附加数据）
	m.RefreshEntry(int64(1001), nil, TestAttachment{Name: "UpdatedPlayer1", Level: 15})

	// 验证附加数据已更新
	_, _, attachment := m.Rank(1001, false)
	att, ok := attachment.(TestAttachment)
	if !ok {
		t.Fatal("attachment should be TestAttachment type")
	}
	if att.Name != "UpdatedPlayer1" {
		t.Errorf("expected name 'UpdatedPlayer1', got '%s'", att.Name)
	}
	if att.Level != 15 {
		t.Errorf("expected level 15, got %d", att.Level)
	}
}

func TestRefreshEntry_WithHistoryScores(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_history", 2, 10)

	// 刷新不存在的条目但有历史分数
	m.RefreshEntry(int64(1001), []float64{100, 50}, TestAttachment{Name: "Player1", Level: 10})

	// 验证数据已添加
	if !m.Contains(1001) {
		t.Error("should contain key 1001 after refresh with history scores")
	}

	scores, ok := m.Score(1001)
	if !ok {
		t.Fatal("should find score for key 1001")
	}
	if scores[0] != 100 || scores[1] != 50 {
		t.Errorf("expected scores [100, 50], got %v", scores)
	}
}

func TestRefreshEntry_WithZeroScores(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_zero", 2, 10)

	// 刷新不存在的条目但历史分数全为零
	m.RefreshEntry(int64(1001), []float64{0, 0}, TestAttachment{Name: "Player1", Level: 10})

	// 验证数据未被添加
	if m.Contains(1001) {
		t.Error("should not contain key 1001 with zero history scores")
	}
}

func TestRefreshEntry_WithWrongDimension(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_wrong_dim", 2, 10)

	// 刷新不存在的条目但历史分数维度错误
	m.RefreshEntry(int64(1001), []float64{100}, TestAttachment{Name: "Player1", Level: 10})

	// 验证数据未被添加
	if m.Contains(1001) {
		t.Error("should not contain key 1001 with wrong dimension history scores")
	}
}

func TestRefreshEntry_InvalidKeyType(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_invalid_key", 2, 10)

	// 使用错误的 key 类型（应该记录错误日志但不 panic）
	m.RefreshEntry("wrong_type", []float64{100, 50}, TestAttachment{Name: "Player1", Level: 10})

	// 排行榜应该为空
	if m.Len() != 0 {
		t.Error("should not add entry with invalid key type")
	}
}

func TestRefreshEntry_InvalidAttachmentType(t *testing.T) {
	m := newTestManager[int64, TestAttachment]("test_refresh_invalid_att", 2, 10)

	// 使用错误的 attachment 类型（应该记录错误日志但不 panic）
	m.RefreshEntry(int64(1001), []float64{100, 50}, "wrong_type")

	// 排行榜应该为空
	if m.Len() != 0 {
		t.Error("should not add entry with invalid attachment type")
	}
}

func TestIsZeroScores(t *testing.T) {
	testCases := []struct {
		name     string
		scores   []float64
		expected bool
	}{
		{"all zeros", []float64{0, 0, 0}, true},
		{"empty", []float64{}, true},
		{"single zero", []float64{0}, true},
		{"has non-zero", []float64{0, 1, 0}, false},
		{"all non-zero", []float64{1, 2, 3}, false},
		{"negative", []float64{-1, 0, 0}, false},
		{"float", []float64{0.1, 0, 0}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isZeroScores(tc.scores)
			if result != tc.expected {
				t.Errorf("isZeroScores(%v) = %v, expected %v", tc.scores, result, tc.expected)
			}
		})
	}
}

func TestStringKey(t *testing.T) {
	m := newTestManager[string, int]("test_string_key", 1, 10)

	err := m.Add("user_001", []float64{100}, 1)
	noErr(t, err)
	err = m.Add("user_002", []float64{200}, 2)
	noErr(t, err)
	err = m.Add("user_003", []float64{150}, 3)
	noErr(t, err)

	if m.Len() != 3 {
		t.Errorf("expected length 3, got %d", m.Len())
	}

	if !m.Contains("user_001") {
		t.Error("should contain key 'user_001'")
	}

	rank, scores, attachment := m.Rank("user_002", false)
	if rank != 0 {
		t.Errorf("expected rank 0 for user_002, got %d", rank)
	}
	if scores[0] != 200 {
		t.Errorf("expected score 200, got %f", scores[0])
	}
	if attachment != 2 {
		t.Errorf("expected attachment 2, got %v", attachment)
	}
}

func TestCapacity(t *testing.T) {
	m := newTestManager[int64, string]("test_capacity", 1, 3)

	// 添加超过容量的数据
	for i := int64(1); i <= 5; i++ {
		err := m.Add(i, []float64{float64(i * 10)}, "Player")
		noErr(t, err)
	}

	// 应该只保留分数最高的3个
	if m.Len() != 3 {
		t.Errorf("expected length 3 (capacity), got %d", m.Len())
	}

	// 验证保留的是分数最高的
	items := m.Range(0, -1)
	expectedKeys := []int64{5, 4, 3} // 50, 40, 30
	for i, item := range items {
		if item.Key != expectedKeys[i] {
			t.Errorf("expected key %d at position %d, got %d", expectedKeys[i], i, item.Key)
		}
	}

	// 低分数的应该被移除
	if m.Contains(1) {
		t.Error("key 1 should have been removed due to capacity")
	}
	if m.Contains(2) {
		t.Error("key 2 should have been removed due to capacity")
	}
}

func TestUpdateExisting(t *testing.T) {
	m := newTestManager[int64, string]("test_update", 2, 10)

	// 添加数据
	err := m.Add(1001, []float64{100, 50}, "Player1")
	noErr(t, err)

	// 更新同一个 key
	err = m.Add(1001, []float64{200, 60}, "UpdatedPlayer1")
	noErr(t, err)

	// 应该只有1条数据
	if m.Len() != 1 {
		t.Errorf("expected length 1 after update, got %d", m.Len())
	}

	// 验证数据已更新
	scores, ok := m.Score(1001)
	if !ok {
		t.Fatal("should find score for key 1001")
	}
	if scores[0] != 200 || scores[1] != 60 {
		t.Errorf("expected scores [200, 60], got %v", scores)
	}

	_, _, attachment := m.Rank(1001, false)
	if attachment != "UpdatedPlayer1" {
		t.Errorf("expected attachment 'UpdatedPlayer1', got %v", attachment)
	}
}

func TestMultiDimensional(t *testing.T) {
	// 测试4维排行榜
	m := newTestManager[int64, string]("test_multi_dim", 4, 10)

	err := m.Add(1001, []float64{100, 80, 90, 85}, "Player1")
	noErr(t, err)
	err = m.Add(1002, []float64{100, 80, 90, 95}, "Player2")
	noErr(t, err)
	err = m.Add(1003, []float64{100, 80, 95, 80}, "Player3")
	noErr(t, err)

	if m.Len() != 3 {
		t.Errorf("expected length 3, got %d", m.Len())
	}

	// 验证多维排序（按每维依次比较）
	// 第1维都是100，第2维都是80
	// 第3维：95(1003) > 90(1001,1002)
	// 第4维比较：95(1002) > 85(1001)
	// 所以排序应该是：1003 > 1002 > 1001
	items := m.Range(0, -1)
	if items[0].Key != 1003 {
		t.Errorf("expected first key 1003, got %d", items[0].Key)
	}
	if items[1].Key != 1002 {
		t.Errorf("expected second key 1002, got %d", items[1].Key)
	}
	if items[2].Key != 1001 {
		t.Errorf("expected third key 1001, got %d", items[2].Key)
	}
}

// 基准测试
func BenchmarkAdd(b *testing.B) {
	m := newTestManager[int64, string]("bench_add", 2, 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Add(int64(i), []float64{float64(i), float64(i % 100)}, "Player")
	}
}

func BenchmarkRank(b *testing.B) {
	m := newTestManager[int64, string]("bench_rank", 2, 10000)
	// 预先添加数据
	for i := 0; i < 10000; i++ {
		m.Add(int64(i), []float64{float64(i), float64(i % 100)}, "Player")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Rank(int64(i%10000), false)
	}
}

func BenchmarkRange(b *testing.B) {
	m := newTestManager[int64, string]("bench_range", 2, 10000)
	// 预先添加数据
	for i := 0; i < 10000; i++ {
		m.Add(int64(i), []float64{float64(i), float64(i % 100)}, "Player")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Range(0, 99) // 获取前100名
	}
}
