package mdzset

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"
	"time"
)

var s *SortedSet[int64]

func init() {
	s = New[int64]("test", 3)
}

func TestZeroAndMin(t *testing.T) {
	z := zero[string]()
	if z != "" {
		t.FailNow()
	}
	zz := zero[int64]()
	if zz != int64(0) {
		t.FailNow()
	}

	m := minValue[string]()
	if m != "" {
		t.FailNow()
	}
	m1 := minValue[int64]()
	if m1 != int64(-9223372036854775808) {
		t.FailNow()
	}
}

func TestNew(t *testing.T) {
	if s == nil {
		t.Failed()
	}
	s.Add([]float64{23, 2, 1}, 1001, "test1")
	s.Add([]float64{3, 1, 1}, 1002, "test2")
	s.Add([]float64{3, 1, 1}, 1003, "test3")
	s.Add([]float64{3, 1, 1}, 1004, "liyiheng")
	s.Add([]float64{3, 1, 1}, 1005, "test4")
	s.Add([]float64{3, 1, 1}, 1006, "test5")
	s.Add([]float64{3, 1, 1}, 1001, "test1")

	rank, score, extra := s.Rank(1004, false)
	if rank == 3 {
		t.Log("Key:", 1004, "Rank:", rank, "Score:", score, "Extra:", extra)
	} else {
		t.Error("Key:", 1004, "Rank:", rank, "Score:", score, "Extra:", extra)
	}
	rank, score, extra = s.Rank(1001, false)
	if rank == 0 {
		t.Log("Key:", 1001, "Rank:", rank, "Score:", score, "Extra:", extra)
	} else {
		t.Error("Key:", 1001, "Rank:", rank, "Score:", score, "Extra:", extra)
	}
	rank, score, extra = s.Rank(-1, false)
	if rank == -1 {
		t.Log("Key:", -1, "Rank:", rank, "Score:", score, "Extra:", extra)
	} else {
		t.Error("Key:", -1, "Rank:", rank, "Score:", score, "Extra:", extra)
	}

	id, score, extra := s.GetDataByRank(0, true)
	t.Log("GetData[REVERSE] Rank:", 0, "ID:", id, "Score:", score, "Extra:", extra)
	id, score, extra = s.GetDataByRank(0, false)
	t.Log("GetData[UNREVERSE] Rank:", 0, "ID:", id, "Score:", score, "Extra:", extra)
	_, _, extra = s.GetDataByRank(9999, true)
	if extra != nil {
		t.Error("GetDataByRank is not nil")
	}
	if s.Len() != 6 {
		t.Error("Rank Data Size is wrong")
	}
	s.Delete(1001)
	if s.Len() != 5 {
		t.Error("Rank Data Size is wrong")
	}
	d, ok := s.GetData(1004)
	t.Log(d, ok)
	curScore, dat := s.IncrBy([]float64{3, 1, 1}, 1004)
	t.Log(curScore, dat)
}

func TestIncrBy(t *testing.T) {
	z := New[int64]("incr_test", 3)
	for i := 1000; i < 1100; i++ {
		z.Add([]float64{float64(rand.IntN(1000)), float64(rand.IntN(1000)), float64(rand.IntN(1000))}, int64(i), "Hello world")
	}
	_, score, _ := z.Rank(1050, false)
	curScore, _ := z.IncrBy([]float64{3, 1, 1}, 1050)
	if !curScore.equal(score.incrByScores([]float64{3, 1, 1})) {
		t.Error("unexpect")
	}
}

func TestSortedSet_UpdateF2(t *testing.T) {
	z := New[int64]("uu", 3)
	for i := 1; i < 100; i++ {
		z.Add([]float64{float64(rand.IntN(1000)), float64(rand.IntN(1000)), float64(rand.IntN(1000))}, int64(i), "Hello world")
	}
	_, scores, _ := z.Rank(5, false)
	valIdx1 := scores[0]
	newScores, _ := z.UpdateF2(5, INCR, 2, SET, 23)
	if valIdx1+2 != newScores[0] {
		t.Error("unexpect idx 0")
		t.Log(newScores)
	}
	if newScores[1] != 23 {
		t.Error("unexpect idx 1")
	}
}

func TestSortedSet_UpdateF3(t *testing.T) {
	z := New[int64]("uu", 3)
	for i := 1; i < 100; i++ {
		z.Add([]float64{float64(rand.IntN(1000)), float64(rand.IntN(1000)), float64(rand.IntN(1000))}, int64(i), "Hello world")
	}
	_, scores, _ := z.Rank(5, false)
	valIdx1 := scores[0]
	valIdx3 := scores[2]
	newScores, _ := z.UpdateF3(5, INCR, 2, SET, 123, INCR, -5)
	if valIdx1+2 != newScores[0] {
		t.Error("unexpect idx 0")
		t.Log(newScores)
	}
	if valIdx3-5 != newScores[2] {
		t.Error("unexpect idx 2")
		t.Log(newScores)
	}
	if newScores[1] != 123 {
		t.Error("unexpect idx 1")
	}
}

func TestSortedSet_UpdateUints(t *testing.T) {
	z := New[int64]("uu", 3)
	for i := 1; i < 100; i++ {
		z.Add([]float64{float64(rand.IntN(1000)), float64(rand.IntN(1000)), float64(rand.IntN(1000))}, int64(i), "Hello world")
	}
	_, scores, _ := z.Rank(5, false)
	uu := []UpdateUnit{
		{0, INCR, 8},
		{1, SET, 45},
		{2, SET, -4},
		{3, INCR, 11},
		{4, INCR, -10},
	}
	valIdx1 := scores[0]
	newScores, _ := z.UpdateUints(5, uu...)
	if valIdx1+8 != newScores[0] {
		t.Error("unexpect idx 0")
		t.Log(newScores)
	}
	if newScores[1] != 45 {
		t.Error("unexpect idx 1")
	}
	if newScores[2] != -4 {
		t.Error("unexpcet idx 2")
	}
}

func TestRange(t *testing.T) {
	z := New[int64]("range_test", 1, false)
	z.Add([]float64{1.0}, 1001, nil)
	z.Add([]float64{2.0}, 1002, nil)
	z.Add([]float64{3.0}, 1003, nil)
	z.Add([]float64{4.0}, 1004, nil)
	z.Add([]float64{5.0}, 1005, nil)
	z.Add([]float64{6.0}, 1006, nil)

	items1 := z.Range(0, -1)
	if items1[0].Key != 1001 ||
		items1[1].Key != 1002 ||
		items1[2].Key != 1003 ||
		items1[3].Key != 1004 {
		t.Fail()
	}
	items := z.RevRange(1, 3)
	for _, v := range items {
		t.Logf("key: %d, score: %v\n", v.Key, v.Score)
	}
}

func TestRangeLargeToSmall(t *testing.T) {
	z := New[int64]("range_test", 1)
	z.Add([]float64{1.0}, 1001, nil)
	z.Add([]float64{2.0}, 1002, nil)
	z.Add([]float64{3.0}, 1003, nil)
	z.Add([]float64{4.0}, 1004, nil)
	z.Add([]float64{5.0}, 1005, nil)
	z.Add([]float64{6.0}, 1006, nil)

	items1 := z.Range(0, -1)
	if items1[0].Key != 1006 ||
		items1[1].Key != 1005 ||
		items1[2].Key != 1004 ||
		items1[3].Key != 1003 {
		t.Fail()
	}
	items := z.RevRange(1, 3)
	for _, v := range items {
		t.Logf("key: %d, score: %v\n", v.Key, v.Score)
	}
}

func TestLargeTop(t *testing.T) {
	// large to small, default
	z := New[int64]("lts", 3)
	for i := 0; i < 100; i++ {
		z.Add([]float64{float64(i), float64(rand.IntN(100)), float64(rand.IntN(100))}, int64(i), i)
	}
	for i := 0; i < 100; i++ {
		r, _, val := z.Rank(int64(rand.IntN(100)), false)
		if r != 99-val.(int) {
			t.Fail()
			return
		}
	}
	// small to large
	z1 := New[int64]("stl", 3, false)
	for i := 0; i < 100; i++ {
		z1.Add([]float64{float64(i), float64(rand.IntN(100)), float64(rand.IntN(100))}, int64(i), i)
	}
	for i := 0; i < 100; i++ {
		r1, _, val1 := z1.Rank(int64(rand.IntN(100)), false)
		if r1 != val1.(int) {
			t.Fail()
			return
		}
	}
}

func TestMonkey(t *testing.T) {
	z := New[int64]("monkey", 3, false)
	total := 200
	for iter := 0; iter < 100; iter++ {
		t.Log("iter: ", iter)
		for i := 0; i < total; i++ {
			z.Add([]float64{float64(rand.IntN(1000)), float64(rand.IntN(1000)), float64(rand.IntN(1000))}, int64(i), i)
		}
		if z.Len() != total {
			t.Error("unexpect len")
			return
		}
		key, score, data := z.GetDataByRank(10, false)
		if int(key) != data.(int) {
			t.Error("key not equal with data", score)
			return
		}
		n := z.Count(score[0], score[0], false, false)
		if n < 1 {
			t.Error("unexpect, n:", n, score)
			items := z.Range(0, 11)
			for _, v := range items {
				t.Logf("key: %d, scores: %v, data: %v\n", v.Key, v.Score, v.Attachment.(int))
			}
		}
		m := z.Count(score[0], score[0], false, true)
		if m != 0 {
			t.Error("unexpect count by exclude max")
			return
		}
		score1, ok := z.Score(key)
		if !ok {
			t.Error("get score failed")
			return
		}
		if !score.equal(score1) {
			t.Error("unexpect score")
			return
		}
		data1, ok1 := z.GetData(key)
		if !ok1 || data1 != data {
			t.Error()
			return
		}
		if !z.Contains(key) {
			t.Error("contain it")
			return
		}
		if r, _, _ := z.Rank(key, false); r != 10 {
			t.Error("rank must be 10")
			t.Logf("rank 10 is key: %d score: %v, now rank:%d\n", key, score, r)
			t.Log(z.Rank(key, false))
			items := z.Range(0, 11)
			for _, v := range items {
				t.Logf("key: %d, scores: %v, data: %v\n", v.Key, v.Score, v.Attachment.(int))
			}
			return
		}
		if !z.Delete(key) {
			t.Error("delete failed")
			return
		}
		if z.Contains(key) && z.Len() != total-1 {
			t.Error("the item deleted already")
			return
		}
		dn := z.RemoveRangeByRank(0, 200)
		if z.Len() != total-1-dn {
			t.Error("unexpect length after removeRangeByRank")
			return
		}
		allDelete := z.RemoveRangeByRank(0, -1)
		if allDelete != total-1-dn {
			t.Error("unexpect")
			return
		}
		if z.Len() != 0 {
			t.Error("zset should be empty")
			return
		}
	}
}

func TestDemo(t *testing.T) {
	z := New[int64]("demo", 4) // 4 dimensional
	z.Add([]float64{88, 90, 92, 91}, 1, "Tom")
	z.Add([]float64{90, 78, 82, 81}, 2, "Mike")
	z.Add([]float64{88, 90, 91, 93}, 3, "Jason")
	if z.Len() != 3 {
		t.Fail()
	}
	s, ok := z.Score(1)
	if ok {
		t.Log("Tom's scores:", s)
	}
	if !z.Contains(4) {
		t.Log("Not Found key: 4")
	}
	name, ok := z.GetData(2)
	if ok {
		t.Log("key: 2, name: ", name.(string))
	}
	// start and end are inclusive.
	z.RemoveRangeByRank(0, 3) // clear all
	if z.Len() != 0 {
		t.Fail()
	}
}

func TestLimited(t *testing.T) {
	z := NewWithFixedSize[int64]("limited", 3, 30, true)
	for i := 0; i < 100; i++ {
		z.Add([]float64{float64(i), float64(rand.IntN(30)), float64(rand.IntN(30))}, int64(i), &Item[int64]{Key: 1})
	}
	z.Iterator(0, z.Len(), true, func(key int64, scores []float64, data interface{}) { // hold the read lock
		key = -1
		scores[0] = 999             // invalid operation
		data.(*Item[int64]).Key = 2 // only when data is pointer, valid operation
		// z.Delete(key)        // deadlock!!, in hook func, it is forbidden to use the operation of applying for the write lock
	})
	t.Log("leaderboard: ", z.Range(0, -1))
	if z.Len() != 30 {
		t.Error("unexpect number")
	}
	idx, _, _ := z.Rank(100, false)
	if idx != -1 {
		t.Error("find not exist element?")
	}
}

func TestFixedZset(t *testing.T) {
	z := NewWithFixedSize[int64]("test", 1, 5)
	z.Add([]float64{100}, 1, "User1")
	z.Add([]float64{101}, 2, "User2")
	z.Add([]float64{108}, 3, "User3")
	z.Add([]float64{103}, 4, "User4")
	z.Add([]float64{108}, 5, "User5")
	z.Add([]float64{105}, 6, "User6")
	z.Add([]float64{106}, 7, "User7")
	z.Add([]float64{107}, 8, "User8")
	if z.Len() != 5 {
		t.Fail()
	}
	z.Add([]float64{108}, 1, "User1")
	z.Add([]float64{108}, 2, "User2")
	if z.Len() != 5 {
		t.Fail()
	}
	rank, _, name := z.Rank(8, false)
	if rank != 4 || name != "User8" {
		t.Fail()
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

func strN(length int) string {
	return stringWithCharset(length, charset)
}

func TestNewWithStringKey(t *testing.T) {
	z := New[string]("sss", 3)
	for i := 0; i < 100; i++ {
		z.Add([]float64{float64(i), float64(rand.IntN(30)), float64(rand.IntN(30))}, strN(6)+strconv.Itoa(i+1), nil)
	}
	if z.Len() != 100 {
		t.Fatal()
	}
	key, score, _ := z.GetDataByRank(10, false)
	t.Log(key, score)

	items := z.TopN(10, false)
	for _, v := range items {
		t.Logf("key: %s  scores: %v", v.Key, v.Score)
	}
}

func TestNewWithStringKey2(t *testing.T) {
	z := New[string]("sss", 1)
	for i := 0; i < 100; i++ {
		z.Add([]float64{100}, strN(6)+strconv.Itoa(i+1), nil)
	}
	if z.Len() != 100 {
		t.Fatal()
	}
	key, score, _ := z.GetDataByRank(10, false)
	t.Log(key, score)

	items := z.TopN(10, false)
	for _, v := range items {
		t.Logf("key: %s  scores: %v", v.Key, v.Score)
	}
}

func TestRoleRankListRemove(t *testing.T) {
	rankLimitNum := 10
	removeRoleID := int64(10)
	rankList := NewWithFixedSize[int64](fmt.Sprintf("role_rank_list_%d", 0), 2, uint64(rankLimitNum+100), true)
	now := time.Now().UnixNano()
	t.Logf("start:%d", now)
	for i := 0; i < rankLimitNum; i++ {
		score := int64(rand.IntN(1000000))
		roleID := int64(i + 1)
		rankList.Add([]float64{float64(score), float64(-now)}, roleID, "")
	}
	end := time.Now().UnixNano()
	t.Logf("end:%d", end)
	useTime := end - now
	t.Logf("UseTime: %dns", useTime)
	items := rankList.Range(0, -1)
	if len(items) != rankLimitNum {
		t.Fail()
	}
	for _, item := range items {
		rankID, _, _ := rankList.Rank(item.Key, false)
		t.Logf("rankID: %d, ID:%d, Score: %v", rankID, item.Key, item.Score)
	}

	rankID, _, _ := rankList.Rank(removeRoleID, false) // 获取key ID: 10的正向排序号
	t.Logf("roleID:%d removeRankID:%d", removeRoleID, rankID)
	t.Logf("=== After ===")
	num := rankList.RemoveRangeByRank(rankID, rankID) // 删除排序号的记录
	if num != 1 {
		t.Fail()
	}
	items = rankList.Range(0, -1)
	if len(items) != 9 {
		t.Fail()
	}
	var has bool
	for _, item := range items {
		if item.Key == removeRoleID {
			has = true
		}
		rankId, _, _ := rankList.Rank(item.Key, false)
		t.Logf("rankID: %d, ID:%d, Score: %v", rankId, item.Key, item.Score)
	}
	if has {
		t.Fail()
	}
}

func TestRemoveRangeByScore(t *testing.T) {
	rankLimitNum := 10
	scoreMin := 5
	scoreMax := 100
	rankList := NewWithFixedSize[int64](fmt.Sprintf("role_rank_list_%d", 0), 2, uint64(rankLimitNum+100), false)
	now := time.Now().UnixNano()
	t.Logf("start:%d", now)
	for i := 0; i < rankLimitNum; i++ {
		score := int64(rand.IntN(1000000))
		roleID := int64(i + 1)
		if roleID == 5 {
			score = int64(scoreMin)
		}
		if roleID == 10 {
			score = 20
		}
		if roleID == 9 {
			score = 10
		}
		if roleID == 8 {
			score = int64(scoreMax)
		}
		rankList.Add([]float64{float64(score), float64(-now)}, roleID, "")
	}
	end := time.Now().UnixNano()
	t.Logf("end:%d", end)
	useTime := end - now
	t.Logf("UseTime: %dns", useTime)

	rangeByScoreItems := rankList.RangeByScore(float64(scoreMin), float64(scoreMax), false, false)
	for _, item := range rangeByScoreItems {
		rankId, _, _ := rankList.Rank(item.Key, false)
		t.Logf("rangeByScoreItems(%d-%d) rankID: %d, ID:%d, Score: %v", scoreMin, scoreMax, rankId, item.Key, item.Score)
	}
	if len(rangeByScoreItems) != 4 {
		t.Fail()
	}
	rangeByScoreItems = rankList.RangeByScore(float64(scoreMin), float64(scoreMax), true, false)
	if len(rangeByScoreItems) != 3 {
		t.Fail()
	}
	rangeByScoreItems = rankList.RangeByScore(float64(scoreMin), float64(scoreMax), true, true)
	if len(rangeByScoreItems) != 2 {
		t.Fail()
	}

	count := rankList.Count(float64(scoreMin), float64(scoreMax), false, false)
	if count != 4 {
		t.Fail()
	}
	t.Logf("Remove After ===")

	removeNum := rankList.RemoveRangeByScore(float64(scoreMin), float64(scoreMax), false, false)
	if removeNum != 4 {
		t.Fail()
	}
	removeItems := rankList.Range(0, -1)
	var has bool
	for _, item := range removeItems {
		if item.Score[0] <= float64(scoreMax) && item.Score[0] >= float64(scoreMin) {
			has = true
		}
		rankId, _, _ := rankList.Rank(item.Key, false)
		t.Logf("removeItems(%d-%d) rankID: %d, ID:%d, Score: %v", scoreMin, scoreMax, rankId, item.Key, item.Score)
	}
	if has {
		t.Fail()
	}
}
