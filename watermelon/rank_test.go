package watermelon

import (
	"testing"
	"time"
	"wx_game/fw/mdzset"
)

func noErr(t testing.TB, err error) {
	t.Helper() //让失败行号指向测试代码行，而不是 Must 内部。
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRank(t *testing.T) {
	s := mdzset.NewWithFixedSize[int64]("watermelon", 2, 5)
	//t.Cleanup(func() { _ = f.Close() }) 保证文件、临时目录、数据库连接都能被回收，测试之间互不污染。
	err := s.Add([]float64{0, 5}, 1, "test1")
	noErr(t, err)

	err = s.Add([]float64{1, 1}, 2, "test2")
	noErr(t, err)

	err = s.Add([]float64{3, 3}, 3, "test3")
	noErr(t, err)

	err = s.Add([]float64{1, 2}, 4, "test4")
	noErr(t, err)

	err = s.Add([]float64{10, -float64(time.Now().Unix())}, 5, "test5")
	time.Sleep(time.Second)
	err = s.Add([]float64{10, -float64(time.Now().Unix())}, 6, "test6")

	for _, v := range s.Range(0, -1) {
		t.Log(v.Key)
	}

	s.UpdateF2(3, mdzset.SET, 0, mdzset.SET, 4)
	t.Log("--------------------")
	for _, v := range s.Range(0, -1) {
		t.Log(v.Key, v.Score, v.Attachment)
	}

	r, v, data := s.Rank(2, false)
	t.Log("get key2: ", r, v, data)
}
