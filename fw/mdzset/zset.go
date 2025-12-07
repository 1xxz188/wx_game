package mdzset

import (
	"errors"
	"math"
	"math/rand/v2"
	"sync"
)

const zSkipListMaxLevel = 32

type UpdateOperator = uint8

const (
	INCR UpdateOperator = iota
	SET
)

type KT interface {
	int64 | string
}

func zero[T KT]() T {
	var z T
	return z
}

func minValue[T KT]() T {
	var z T
	switch any(z).(type) {
	case int64:
		return any(int64(math.MinInt64)).(T)
	case string:
		return any(z).(T)
	default:
		return z
	}
}

type (
	Scores []float64

	skipListLevel[T KT] struct {
		forward *skipListNode[T]
		span    uint64
	}

	skipListNode[T KT] struct {
		objID    T
		score    Scores
		backward *skipListNode[T]
		level    []*skipListLevel[T]
	}
	Item[T KT] struct {
		Key        T
		Score      Scores
		Attachment any
	}

	skipList[T KT] struct {
		header *skipListNode[T]
		tail   *skipListNode[T]
		length int
		level  int16
		lts    int // -1 means large to small, 1 means small to large
	}
	// SortedSet ..
	SortedSet[T KT] struct {
		name        string
		mu          sync.RWMutex
		dict        map[T]*Item[T]
		zsl         *skipList[T]
		dimensional int
		limit       int // limited use
		zeroValue   T
		iterMu      sync.Mutex
	}
	zRangeSpec struct {
		min   float64
		max   float64
		minEx bool
		maxEx bool
	}
	zLexRangeSpec[T KT] struct {
		minKey T
		maxKey T
		minEx  bool
		maxEx  bool
	}
)

func (s Scores) equal(o Scores) bool {
	// not check the length of s and o are equal, handle it by users
	for i := 0; i < len(s); i++ {
		if s[i] != o[i] {
			return false
		}
	}
	return true
}

func (s Scores) isZero() bool {
	for i := 0; i < len(s); i++ {
		if s[i] != 0 {
			return false
		}
	}
	return true
}

func (s Scores) lessThan(o Scores) int {
	for i := 0; i < len(s); i++ {
		if s[i] != o[i] {
			if s[i] < o[i] {
				return 1 // less
			} else {
				return -1 // more
			}
		}
		continue
	}
	return 0 // equal
}

func (s Scores) incrBy(idx int, delta float64) Scores {
	newS := make([]float64, len(s))
	for i := 0; i < len(s); i++ {
		if i == idx {
			newS[i] = s[i] + delta
			continue
		}
		newS[i] = s[i]
	}
	return newS
}

func (s Scores) incrByScores(delta Scores) Scores {
	newS := make([]float64, len(s))
	for i := 0; i < len(s); i++ {
		newS[i] = s[i] + delta[i]
	}
	return newS
}

func zslCreateNode[T KT](level int16, score Scores, id T) *skipListNode[T] {
	n := &skipListNode[T]{
		score: score,
		objID: id,
		level: make([]*skipListLevel[T], level),
	}
	for i := range n.level {
		n.level[i] = new(skipListLevel[T])
	}
	return n
}

func zslCreate[T KT](d int) *skipList[T] {
	s := make([]float64, d)
	for i := 0; i < d; i++ {
		s[i] = math.Inf(-1)
	}
	return &skipList[T]{
		level:  1,
		header: zslCreateNode[T](zSkipListMaxLevel, s, minValue[T]()),
	}
}

const zSkipListP = 0.25 /* Skiplist P = 1/4 */

/* Returns a random level for the new skiplist node we are going to create.
 * The return value of this function is between 1 and _ZSKIPLIST_MAXLEVEL
 * (both inclusive), with a powerlaw-alike distribution where higher
 * levels are less likely to be returned. */
func randomLevel() int16 {
	level := int16(1)
	for float32(rand.Int32()&0xFFFF) < (zSkipListP * 0xFFFF) {
		level++
	}
	if level < zSkipListMaxLevel {
		return level
	}
	return zSkipListMaxLevel
}

/* zslInsert a new node in the skiplist. Assumes the element does not already
 * exist (up to the caller to enforce that). The skiplist takes ownership
 * of the passed SDS string 'Item'. */
func (zsl *skipList[T]) zslInsert(score Scores, id T) *skipListNode[T] {
	update := make([]*skipListNode[T], zSkipListMaxLevel)
	rank := make([]uint64, zSkipListMaxLevel)
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		/* store rank that is crossed to reach the insert position */
		if i == zsl.level-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}
		if x.level[i] != nil {
			for x.level[i].forward != nil &&
				(x.level[i].forward.score.lessThan(score) == zsl.lts ||
					(x.level[i].forward.score.equal(score) && x.level[i].forward.objID < id)) {
				rank[i] += x.level[i].span
				x = x.level[i].forward
			}
		}
		update[i] = x
	}
	/* we assume the element is not already inside, since we allow duplicated
	 * scores, reinserting the same element should never happen since the
	 * caller of zslInsert() should test in the hash table if the element is
	 * already inside or not. */
	level := randomLevel()
	if level > zsl.level {
		for i := zsl.level; i < level; i++ {
			rank[i] = 0
			update[i] = zsl.header
			update[i].level[i].span = uint64(zsl.length)
		}
		zsl.level = level
	}
	x = zslCreateNode(level, score, id)
	for i := int16(0); i < level; i++ {
		x.level[i].forward = update[i].level[i].forward
		update[i].level[i].forward = x

		/* update span covered by update[i] as x is inserted here */
		x.level[i].span = update[i].level[i].span - (rank[0] - rank[i])
		update[i].level[i].span = (rank[0] - rank[i]) + 1
	}

	/* increment span for untouched levels */
	for i := level; i < zsl.level; i++ {
		update[i].level[i].span++
	}

	if update[0] == zsl.header {
		x.backward = nil
	} else {
		x.backward = update[0]

	}
	if x.level[0].forward != nil {
		x.level[0].forward.backward = x
	} else {
		zsl.tail = x
	}
	zsl.length++
	return x
}

// zslDeleteNode Internal function used by zslDelete, zslDeleteByScore and zslDeleteByRank
func (zsl *skipList[T]) zslDeleteNode(x *skipListNode[T], update []*skipListNode[T]) {
	for i := int16(0); i < zsl.level; i++ {
		if update[i].level[i].forward == x {
			update[i].level[i].span += x.level[i].span - 1
			update[i].level[i].forward = x.level[i].forward
		} else {
			update[i].level[i].span--
		}
	}
	if x.level[0].forward != nil {
		x.level[0].forward.backward = x.backward
	} else {
		zsl.tail = x.backward
	}
	for zsl.level > 1 && zsl.header.level[zsl.level-1].forward == nil {
		zsl.level--
	}
	zsl.length--
}

/* Delete an element with matching score/element from the skiplist.
 * The function returns 1 if the node was found and deleted, otherwise
 * 0 is returned.
 *
 * If 'node' is NULL the deleted node is freed by zslFreeNode(), otherwise
 * it is not freed (but just unlinked) and *node is set to the node pointer,
 * so that it is possible for the caller to reuse the node (including the
 * referenced SDS string at node->Item). */
func (zsl *skipList[T]) zslDelete(score Scores, id T) int {
	update := make([]*skipListNode[T], zSkipListMaxLevel)
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil &&
			(x.level[i].forward.score.lessThan(score) == zsl.lts ||
				(x.level[i].forward.score.equal(score) && x.level[i].forward.objID < id)) {
			x = x.level[i].forward
		}
		update[i] = x
	}
	/* We may have multiple elements with the same score, what we need
	 * is to find the element with both the right score and object. */
	x = x.level[0].forward
	if x != nil && score.equal(x.score) && x.objID == id {
		zsl.zslDeleteNode(x, update)
		return 1
	}
	return 0 /* not found */
}

func zslValueGteMin(value float64, spec *zRangeSpec) bool {
	if spec.minEx {
		return value > spec.min
	}
	return value >= spec.min
}

func zslValueLteMax(value float64, spec *zRangeSpec) bool {
	if spec.maxEx {
		return value < spec.max
	}
	return value <= spec.max
}

// zslIsInRange returns if there is a part of the zset is in range.
func (zsl *skipList[T]) zslIsInRange(ran *zRangeSpec) bool {
	/* Test for ranges that will always be empty. */
	if ran.min > ran.max || (ran.min == ran.max && (ran.minEx || ran.maxEx)) {
		return false
	}
	var x *skipListNode[T]
	if zsl.lts == -1 { // from large to small
		x = zsl.header.level[0].forward
	} else { // from small to large
		x = zsl.tail
	}
	if x == nil || !zslValueGteMin(x.score[0], ran) { // bigger than min 最大值与min比较
		return false
	}
	if zsl.lts == -1 {
		x = zsl.tail
	} else {
		x = zsl.header.level[0].forward
	}
	if x == nil || !zslValueLteMax(x.score[0], ran) { // 最小值与max比较
		return false
	}
	return true
}

// zslFirstInRange find the first node that is contained in the specified range
func (zsl *skipList[T]) zslFirstInRange(ran *zRangeSpec) *skipListNode[T] {
	/* If everything is out of range, return early. */
	if !zsl.zslIsInRange(ran) {
		return nil
	}

	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		/* Go forward while *OUT* of range. */
		if zsl.lts == -1 { // from large to small
			for x.level[i].forward != nil && !zslValueLteMax(x.level[i].forward.score[0], ran) {
				x = x.level[i].forward
			}
		} else { // from small to large
			for x.level[i].forward != nil && !zslValueGteMin(x.level[i].forward.score[0], ran) {
				x = x.level[i].forward
			}
		}
	}
	/* This is an inner range, so the next node cannot be NULL. */
	x = x.level[0].forward
	//serverAssert(x != NULL);

	/* Check if score <= max. */
	if !zslValueLteMax(x.score[0], ran) {
		return nil
	}
	return x
}

// zslLastInRange find the last node that is contained in the specified range
func (zsl *skipList[T]) zslLastInRange(ran *zRangeSpec) *skipListNode[T] {
	// If everything is out of range, return early
	if !zsl.zslIsInRange(ran) {
		return nil
	}
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		/* Go forward while *IN* range. */
		if zsl.lts == -1 {
			for x.level[i].forward != nil &&
				zslValueGteMin(x.level[i].forward.score[0], ran) {
				x = x.level[i].forward
			}
		} else {
			for x.level[i].forward != nil &&
				zslValueLteMax(x.level[i].forward.score[0], ran) {
				x = x.level[i].forward
			}
		}
	}
	/* This is an inner range, so this node cannot be NULL. */
	//serverAssert(x != NULL);

	// Check if score >= min
	if !zslValueGteMin(x.score[0], ran) {
		return nil
	}
	return x
}

/* Delete all the elements with score between min and max from the skiplist.
 * min and max are inclusive, so a score >= min || score <= max is deleted.
 * Note that this function takes the reference to the hash table view of the
 * sorted set, in order to remove the elements from the hash table too. */
func (zsl *skipList[T]) zslDeleteRangeByScore(ran *zRangeSpec, dict map[T]*Item[T]) uint64 {
	removed := uint64(0)
	update := make([]*skipListNode[T], zSkipListMaxLevel)
	x := zsl.header

	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil {
			var condition bool
			if zsl.lts == -1 {
				if ran.maxEx {
					condition = x.level[i].forward.score[0] >= ran.max
				} else {
					condition = x.level[i].forward.score[0] > ran.max
				}
			} else {
				if ran.minEx {
					condition = x.level[i].forward.score[0] <= ran.min
				} else {
					condition = x.level[i].forward.score[0] < ran.min
				}
			}
			if !condition {
				break
			}
			x = x.level[i].forward
		}
		update[i] = x
	}

	/* Current node is the last with score < or <= min. */
	x = x.level[0].forward

	/* Delete nodes while in range. */
	for x != nil {
		var condition bool
		if zsl.lts == -1 {
			if ran.minEx {
				condition = x.score[0] > ran.min
			} else {
				condition = x.score[0] >= ran.min
			}
		} else {
			if ran.maxEx {
				condition = x.score[0] < ran.max
			} else {
				condition = x.score[0] <= ran.max
			}
		}
		if !condition {
			break
		}
		next := x.level[0].forward
		zsl.zslDeleteNode(x, update)
		delete(dict, x.objID)
		// Here is where x->Item is actually released.
		// And golang has GC, don't need to free manually anymore
		//zslFreeNode(x)
		removed++
		x = next
	}
	return removed
}

func (zsl *skipList[T]) zslDeleteRangeByLex(ran *zLexRangeSpec[T], dict map[T]*Item[T]) uint64 {
	removed := uint64(0)

	update := make([]*skipListNode[T], zSkipListMaxLevel)
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && !zslLexValueGteMin(x.level[i].forward.objID, ran) {
			x = x.level[i].forward
		}
		update[i] = x
	}

	/* Current node is the last with score < or <= min. */
	x = x.level[0].forward

	/* Delete nodes while in range. */
	for x != nil && zslLexValueLteMax(x.objID, ran) {
		next := x.level[0].forward
		zsl.zslDeleteNode(x, update)
		delete(dict, x.objID)
		removed++
		x = next
	}
	return removed
}

func zslLexValueGteMin[T KT](id T, spec *zLexRangeSpec[T]) bool {
	if spec.minEx {
		return compareKey(id, spec.minKey) > 0
	}
	return compareKey(id, spec.minKey) >= 0
}

func compareKey[T KT](a, b T) int8 {
	if a == b {
		return 0
	} else if a > b {
		return 1
	}
	return -1
}

func zslLexValueLteMax[T KT](id T, spec *zLexRangeSpec[T]) bool {
	if spec.maxEx {
		return compareKey(id, spec.maxKey) < 0
	}
	return compareKey(id, spec.maxKey) <= 0
}

/* Delete all the elements with rank between start and end from the skiplist.
 * Start and end are inclusive. Note that start and end need to be 1-based */
func (zsl *skipList[T]) zslDeleteRangeByRank(start, end uint64, dict map[T]*Item[T]) int {
	update := make([]*skipListNode[T], zSkipListMaxLevel)
	var traversed uint64
	var removed int

	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && (traversed+x.level[i].span) < start+1 {
			traversed += x.level[i].span
			x = x.level[i].forward
		}
		update[i] = x
	}

	x = x.level[0].forward
	for x != nil && traversed <= end {
		next := x.level[0].forward
		zsl.zslDeleteNode(x, update)
		delete(dict, x.objID)
		removed++
		traversed++
		x = next
	}
	return removed
}

/* Find the rank for an element by both score and Item.
 * Returns 0 when the element cannot be found, rank otherwise.
 * Note that the rank is 1-based due to the span of zsl->header to the
 * first element. */
func (zsl *skipList[T]) zslGetRank(score Scores, key T) int {
	rank := uint64(0)
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil &&
			(x.level[i].forward.score.lessThan(score) == zsl.lts ||
				(x.level[i].forward.score.equal(score) &&
					x.level[i].forward.objID <= key)) {
			rank += x.level[i].span
			x = x.level[i].forward
		}

		/* x might be equal to zsl->header, so test if Item is non-NULL */
		if x.objID == key {
			return int(rank)
		}
	}
	return 0
}

/* Finds an element by its rank. The rank argument needs to be 1-based. */
func (zsl *skipList[T]) zslGetElementByRank(rank uint64) *skipListNode[T] {
	traversed := uint64(0)
	x := zsl.header
	for i := zsl.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil && (traversed+x.level[i].span) <= rank {
			traversed += x.level[i].span
			x = x.level[i].forward
		}
		if traversed == rank {
			return x
		}
	}
	return nil
}

// New creates a new SortedSet and return its pointer, default lts(large to small) is true
func New[T KT](name string, d int, lts ...bool) *SortedSet[T] {
	if d < 1 {
		return nil
	}
	s := &SortedSet[T]{
		name:        name,
		dict:        make(map[T]*Item[T]),
		zsl:         zslCreate[T](d),
		dimensional: d,
		zeroValue:   zero[T](),
	}
	if len(lts) != 0 {
		if lts[0] {
			s.zsl.lts = -1
		} else {
			s.zsl.lts = 1
		}
	} else {
		s.zsl.lts = -1
	}
	return s
}

// NewWithFixedSize  create a mdzset with fixed capacity, compared with actual needs, the capacity must be redundant enough.
// Otherwise, some items may not enter topN as expected.
func NewWithFixedSize[T KT](name string, d int, capacity uint64, lts ...bool) *SortedSet[T] {
	if d < 1 || capacity == 0 {
		return nil
	}
	s := &SortedSet[T]{
		name:        name,
		dict:        make(map[T]*Item[T]),
		zsl:         zslCreate[T](d),
		dimensional: d,
		limit:       int(capacity),
		zeroValue:   zero[T](),
	}
	if len(lts) != 0 {
		if lts[0] {
			s.zsl.lts = -1
		} else {
			s.zsl.lts = 1
		}
	} else {
		s.zsl.lts = -1
	}
	return s
}

// Name returns name of SortedSet
func (z *SortedSet[T]) Name() string {
	return z.name
}

// Len returns counts of elements
func (z *SortedSet[T]) Len() int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.zsl.length
}

// Add is used to add or update an element, if the new score is equal to the current, any changes are discarded.
func (z *SortedSet[T]) Add(score Scores, key T, dat interface{}) error {
	if len(score) != z.dimensional {
		return errors.New("add len(score) != z.dimensional")
	}
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if ok {
		if !score.equal(v.Score) {
			z.zsl.zslDelete(v.Score, key)
			v.Key = key
			v.Score = score
			v.Attachment = dat
			z.zsl.zslInsert(score, key)
		}
	} else {
		if z.limit != 0 && z.limit <= z.zsl.length { // limit enable and full
			lastNode := z.zsl.zslGetElementByRank(uint64(z.zsl.length))
			cmp := lastNode.score.lessThan(score)
			if cmp == z.zsl.lts || cmp == 0 { // lastNode <= this
				return nil
			} else {
				// lastNode.objID not equal to key, otherwise will not reach this.
				z.zsl.zslDelete(lastNode.score, lastNode.objID)
				delete(z.dict, lastNode.objID)
				z.dict[key] = &Item[T]{Attachment: dat, Key: key, Score: score}
				z.zsl.zslInsert(score, key)
			}
		} else {
			z.dict[key] = &Item[T]{Attachment: dat, Key: key, Score: score}
			z.zsl.zslInsert(score, key)
		}
	}
	return nil
}

type UpdateUnit struct {
	Idx      uint8
	Operator UpdateOperator //INCR,SET
	Val      float64
}

// UpdateF2  update first two scores
func (z *SortedSet[T]) UpdateF2(key T, op1 UpdateOperator, val1 float64, op2 UpdateOperator, val2 float64) (Scores, interface{}) {
	if z.dimensional < 2 {
		return nil, nil
	}
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if !ok {
		return nil, nil
	}
	if val1 == 0 && val2 == 0 {
		return v.Score, v.Attachment
	}
	z.zsl.zslDelete(v.Score, key)
	if op1 == INCR {
		v.Score[0] += val1
	} else {
		v.Score[0] = val1
	}
	if op2 == INCR {
		v.Score[1] += val2
	} else {
		v.Score[1] = val2
	}
	z.zsl.zslInsert(v.Score, key)
	return v.Score, v.Attachment
}

// UpdateF3  update first three scores
func (z *SortedSet[T]) UpdateF3(key T, op1 UpdateOperator, val1 float64, op2 UpdateOperator, val2 float64, op3 UpdateOperator, val3 float64) (Scores, interface{}) {
	if z.dimensional < 3 {
		return nil, nil
	}
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if !ok {
		return nil, nil
	}
	if val1 == 0 && val2 == 0 && val3 == 0 {
		return v.Score, v.Attachment
	}
	z.zsl.zslDelete(v.Score, key)
	if op1 == INCR {
		v.Score[0] += val1
	} else {
		v.Score[0] = val1
	}
	if op2 == INCR {
		v.Score[1] += val2
	} else {
		v.Score[1] = val2
	}
	if op3 == INCR {
		v.Score[2] += val3
	} else {
		v.Score[2] = val3
	}
	z.zsl.zslInsert(v.Score, key)
	return v.Score, v.Attachment
}

// UpdateUints ...
func (z *SortedSet[T]) UpdateUints(key T, uu ...UpdateUnit) (Scores, interface{}) {
	if len(uu) == 0 {
		return nil, nil
	}
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if !ok {
		return nil, nil
	}
	z.zsl.zslDelete(v.Score, key)
	for _, v1 := range uu {
		if v1.Operator == INCR {
			if v1.Val != 0 && int(v1.Idx) < z.dimensional {
				v.Score[v1.Idx] += v1.Val
			}
		} else {
			if int(v1.Idx) < z.dimensional {
				v.Score[v1.Idx] = v1.Val
			}
		}
	}
	z.zsl.zslInsert(v.Score, key)
	return v.Score, v.Attachment
}

// IncrBy ..
func (z *SortedSet[T]) IncrBy(score Scores, key T) (Scores, interface{}) {
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if !ok {
		return nil, nil
	}
	if !score.isZero() {
		z.zsl.zslDelete(v.Score, key)
		v.Score = v.Score.incrByScores(score)
		z.zsl.zslInsert(v.Score, key)
	}
	return v.Score, v.Attachment
}

// IncrByIndex ..
func (z *SortedSet[T]) IncrByIndex(idx int, deltaScore float64, key T) (Scores, interface{}) {
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if !ok {
		return nil, nil
	}
	if deltaScore != 0 && idx < z.dimensional {
		z.zsl.zslDelete(v.Score, key)
		v.Score = v.Score.incrBy(idx, deltaScore)
		z.zsl.zslInsert(v.Score, key)
	}
	return v.Score, v.Attachment
}

// Delete removes an element from the SortedSet by its key.
func (z *SortedSet[T]) Delete(key T) bool {
	z.mu.Lock()
	defer z.mu.Unlock()
	v, ok := z.dict[key]
	if ok {
		z.zsl.zslDelete(v.Score, key)
		delete(z.dict, key)
		return true
	}
	return false
}

// Rank returns position,score and extra data of an element which
// found by the parameter key.
// The parameter reverse determines the rank is descent or ascend，
// true means descend and false means ascend.
func (z *SortedSet[T]) Rank(key T, reverse bool) (int, Scores, any) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	v, ok := z.dict[key]
	if !ok {
		return -1, nil, nil
	}
	r := z.zsl.zslGetRank(v.Score, key)
	if reverse {
		r = z.zsl.length - r
	} else {
		r--
	}
	return r, v.Score, v.Attachment

}

// TopN returns the items of topN
func (z *SortedSet[T]) TopN(n int, reverse bool) []Item[T] {
	if reverse {
		return z.RevRange(0, n)
	}
	return z.Range(0, n)
}

// SetData set Attachment data for the item
func (z *SortedSet[T]) SetData(key T, dat any) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	o, ok := z.dict[key]
	if ok {
		o.Attachment = dat
	}
}

// GetData returns Attachment data stored in the map by its key
func (z *SortedSet[T]) GetData(key T) (any, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	o, ok := z.dict[key]
	if !ok {
		return nil, false
	}
	return o.Attachment, true
}

// Contains returns whether the value exists in sorted set.
func (z *SortedSet[T]) Contains(key T) bool {
	z.mu.RLock()
	defer z.mu.RUnlock()
	_, ok := z.dict[key]
	return ok
}

// Score implements ZScore
func (z *SortedSet[T]) Score(key T) (Scores, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	o, ok := z.dict[key]
	if !ok {
		return nil, false
	}
	return o.Score, true
}

// Count implements ZCOUNT
func (z *SortedSet[T]) Count(min, max float64, minEx, maxEx bool) int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	ran := zRangeSpec{
		min:   min,
		max:   max,
		minEx: minEx,
		maxEx: maxEx,
	}
	first := z.zsl.zslFirstInRange(&ran)
	if first == nil {
		return 0
	}
	firstRank := z.zsl.zslGetRank(first.score, first.objID)
	last := z.zsl.zslLastInRange(&ran)
	if last == nil {
		return z.zsl.length - firstRank
	}
	lastRank := z.zsl.zslGetRank(last.score, last.objID)
	return lastRank - firstRank + 1
}

func (z *SortedSet[T]) RangeByScore(min, max float64, minEx, maxEx bool) []Item[T] {
	z.mu.RLock()
	defer z.mu.RUnlock()
	ran := zRangeSpec{
		min:   min,
		max:   max,
		minEx: minEx,
		maxEx: maxEx,
	}
	first := z.zsl.zslFirstInRange(&ran)
	if first == nil {
		return nil
	}
	firstRank := z.zsl.zslGetRank(first.score, first.objID)
	firstRank-- // start from index 0

	last := z.zsl.zslLastInRange(&ran)
	var lastRank int
	if last == nil {
		lastRank = -1
	} else {
		lastRank = z.zsl.zslGetRank(last.score, last.objID)
		lastRank--
	}
	return z.Range(firstRank, lastRank)
}

func (z *SortedSet[T]) RemoveRangeByRank(start, stop int) int {
	z.mu.Lock()
	defer z.mu.Unlock()
	if start < 0 {
		start = z.zsl.length + start
	}
	if stop < 0 {
		stop = z.zsl.length + stop
	}
	return z.zsl.zslDeleteRangeByRank(uint64(start), uint64(stop), z.dict)
}

func (z *SortedSet[T]) RemoveRangeByScore(min, max float64, minEx, maxEx bool) int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	ran := zRangeSpec{
		min:   min,
		max:   max,
		minEx: minEx,
		maxEx: maxEx,
	}
	return int(z.zsl.zslDeleteRangeByScore(&ran, z.dict))
}

// GetDataByRank returns the id,score and extra data of an element which
// found by position in the rank.
// The parameter rank is the position, reverse says if in the descend rank.
func (z *SortedSet[T]) GetDataByRank(rank int, reverse bool) (T, Scores, interface{}) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	if rank < 0 || rank > z.zsl.length {
		return z.zeroValue, nil, nil
	}
	if reverse {
		rank = z.zsl.length - rank
	} else {
		rank++
	}
	n := z.zsl.zslGetElementByRank(uint64(rank))
	if n == nil {
		return z.zeroValue, nil, nil
	}
	dat, _ := z.dict[n.objID]
	if dat == nil {
		return z.zeroValue, nil, nil
	}
	return dat.Key, dat.Score, dat.Attachment
}

// Range implements ZRANGE
func (z *SortedSet[T]) Range(start, end int) []Item[T] {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.commonRange(start, end, false)
}

// RevRange implements ZREVRANGE
func (z *SortedSet[T]) RevRange(start, end int) []Item[T] {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.commonRange(start, end, true)
}

// UpdateAttachment update attachment if item is exist
func (z *SortedSet[T]) UpdateAttachment(key T, attachment interface{}) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if _, ok := z.dict[key]; ok {
		z.dict[key].Attachment = attachment
	}
}

func (z *SortedSet[T]) commonRange(start, end int, reverse bool) []Item[T] {
	l := z.zsl.length
	if start < 0 {
		start += l
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += l
	}

	if start > end || start >= l {
		return nil
	}
	if end >= l {
		end = l - 1
	}
	span := (end - start) + 1

	var node *skipListNode[T]
	if reverse {
		node = z.zsl.tail
		if start > 0 {
			node = z.zsl.zslGetElementByRank(uint64(l - start))
		}
	} else {
		node = z.zsl.header.level[0].forward
		if start > 0 {
			node = z.zsl.zslGetElementByRank(uint64(start + 1))
		}
	}
	items := make([]Item[T], 0, span)
	for span > 0 {
		span--
		items = append(items, Item[T]{
			Key:        node.objID,
			Score:      node.score,
			Attachment: z.dict[node.objID].Attachment,
		})
		if reverse {
			node = node.backward
		} else {
			node = node.level[0].forward
		}
	}
	return items
}

func (z *SortedSet[T]) commonIterator(start, end int, reverse bool, f func(T, []float64, interface{})) {
	l := z.zsl.length
	if start < 0 {
		start += l
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += l
	}

	if start > end || start >= l {
		return
	}
	if end >= l {
		end = l - 1
	}
	span := (end - start) + 1

	var node *skipListNode[T]
	if reverse {
		node = z.zsl.tail
		if start > 0 {
			node = z.zsl.zslGetElementByRank(uint64(l - start))
		}
	} else {
		node = z.zsl.header.level[0].forward
		if start > 0 {
			node = z.zsl.zslGetElementByRank(uint64(start + 1))
		}
	}
	scoreCopy := make([]float64, z.dimensional)
	for span > 0 {
		span--
		copy(scoreCopy, node.score)
		f(node.objID, scoreCopy, z.dict[node.objID].Attachment)
		if reverse {
			node = node.backward
		} else {
			node = node.level[0].forward
		}
	}
}

// Iterator in the iterator, the value modification of key and scores is invalid.
// When data is a pointer, the modification is valid.
// !!! Iterator hold the read lock, so you can't call any function that applies for write lock in f
// otherwise, it will deadlock.
func (z *SortedSet[T]) Iterator(start, end int, reverse bool, f func(T, []float64, interface{})) {
	z.iterMu.Lock()
	defer z.iterMu.Unlock()
	z.mu.RLock()
	defer z.mu.RUnlock()
	z.commonIterator(start, end, reverse, f)
}
