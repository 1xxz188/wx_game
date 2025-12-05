### mdzset
---

#### 介绍

mdzset提供了并发安全排序集，可以用作Redis 的 zset的本地替代。

#### 介绍

* Go >= 1.22

#### 特征

- 支持多维(可以自定义任意维度数量)数据项排序

  | 参与方 | 金牌 | 银牌 | 铜牌 | 
    |-----|----|----|----|
  | A   | 32 | 21 | 16 |
  | B   | 21 | 12 | 21 |
  | C   | 20 | 47 | 12 |
  | D   | 13 | 21 | 19 |
  | E   | 13 | 21 | 18 |
  | F   | 13 | 17 | 14 |

- 并发安全的API
- 支持泛型key, 目前: int64 | string
- 默认从大到小排列，创建zset实例时可指定从小到大排列
- 每个数据项可以携带any类型的扩展信息字段，比如用于放置玩家摘要信息
- 相当于redis zset的实现
- 快速跳表级别随机化

#### API和算法复杂度
- New

  New(name string, d int, lts ...bool)
  新建一个zset实例，可指定实例名称、维度数量、从大到小(默认)/从小到大排列、 支持最大元素个数: 2^32-1
- NewWithFixedSize

  新建一个有限大小的zset实例，可指定维度数量、容量、是否大值在上；
  
  > 相比实际的业务需要，容量需要设置足够的冗余量。否则TopN结果可能是非预期的。
  >
  > 例如，业务需要TOP10，容量也只设置了10。 新加入了一个元素A排在TOP11, 因为容量的限制，它并不会被真实加入。
  > 之后，TOP10中的某个元素B的分值更新（变小），且B < A。 但元素B仍然在TOP10中，但A已经没有机会再次添加了。
  >
  > 所以，用户在使用这个实例时，需要对capacity做一定的冗余量。
- Len
  
  获取当前zset实例的元素个数；  O(1)
- Add
  
  给当前zset添加一个元素,若元素已存在则**更新**该元素；  O(logN)
- IncrBy
  
  分数值(完整差量数组)更新；
- IncrByIndex

  分数值(更新某一项分值)更新；
- UpdateF2

  更新前2个元素，并指定更新值的操作（增量/设置）
- UpdateF3

  更新前3个元素，并指定更新值的操作（增量/设置）
- UpdateUints

  更新N个元素，并指定更新的索引位置、值操作类型（增量/设置）、分值
- UpdateAttachment

  更新指定key的扩展属性对象
- Delete

  移除指定的数据项
- Rank

  获取指定ID的正向/反向排序号
- Range
  
  获取起点位置到终点位置的元素
- RevRange

  获取起点位置到终点位置的元素，反向
- Iterator

  迭代器，用于遍历指定范围内的元素；在迭代器中，修改key和scores是无效的（只读），当data是个指针时，允许修改其值
  注意：
  1. 不支持对同一个zset并行多个迭代器（有CAS锁限制）
  2. 不允许在回调函数f的实现中使用任何申请写锁的其他API，否则会发生死锁；
- GetData

  获取指定的key的Attachment数据
- Contains

  获取是否存在指定的key的元素
- Score

  获取指定元素的分数值
- Count

  获取处在指定的最小值和最大值的元素个数，并可指定是否排除最大、最小值本身
- RangeByScore

  获取处在的最小值和最大值的第一维分值的元素个数，并可指定是否排除最大、最小值本身
- RemoveRangeByRank

  通过排序位置的起点位置到终点位置的数据项删除
- RemoveRangeByScore

  通过删除第一维分值处于最小值和最大值的数据项目，可并可指定是否排除最大、最小值本身
- GetDataByRank

  通过排序位置获取该元素的key，score和data

#### Example
```
  z := New(4) // 4维数据项
  z.Add([]float64{88, 90, 92, 91}, 1, "Tom")
  z.Add([]float64{90, 78, 82, 81}, 2, "Mike")
  z.Add([]float64{88, 90, 91, 93}, 3, "Jason")
  fmt.Println("Total: ", z.Len())
  fmt.Println("leaderboard: ", z.Range(0, -1))
  s, ok := z.Score(1)
  if ok {
     fmt.Println("Tom's scores:", s)
  }
  if z.Contains(4) {
  
  }
  name, ok := z.GetData(2)
  if ok {
     fmt.Println("key: 2, name: ", name.(string))
  }
  z.RemoveRangeByRank(0, -1) // clear all

```

#### Benchmark
3个维度值
```
Add 1000000 items, cost: 3787 ms, avg: 263/ms  worker: 1
zset len: 1000000
zset count (100-500): 40231
zset top 10:
key: uid62071, score: [0 102 4635]
key: uid608428, score: [0 123 9627]
key: uid960053, score: [0 157 2703]
key: uid574571, score: [0 250 2840]
key: uid936880, score: [0 484 7960]
key: uid861327, score: [0 614 920]
key: uid181921, score: [0 622 1962]
key: uid636310, score: [0 690 8084]
key: uid554832, score: [0 776 1334]
key: uid439779, score: [0 851 9588]
key: uid838564, score: [0 870 816]

Update 1000000 items, cost: 8625 ms, avg: 80/ms  worker: 1
zset len: 1000000
zset count (1000-2000): 100546
zset top 10:
key: uid645326, score: [10992 5488 4735]
key: uid147684, score: [10992 5140 3756]
key: uid33664, score: [10992 4008 661]
key: uid249708, score: [10990 8252 8004]
key: uid805563, score: [10990 1388 6809]
key: uid154363, score: [10989 5606 7083]
key: uid695606, score: [10986 10411 10250]
key: uid633272, score: [10986 4400 4653]
key: uid846282, score: [10986 3311 2084]
key: uid319905, score: [10985 5005 7159]
key: uid339764, score: [10984 10263 3160]
```

4个维度的值 （相比3维度的要慢5-8%）
```
Add 100000 items, cost: 158 ms, avg: 628/ms  worker: 1
zset len: 100000
zset count (100-500): 3987
zset top 10:
key: uid918, score: [0 790 455 8662]
key: uid96447, score: [0 996 6187 6348]
key: uid74512, score: [0 1632 6256 9996]
key: uid19666, score: [0 1750 49 7333]
key: uid50228, score: [0 2024 2658 8641]
key: uid22035, score: [0 6796 1051 7377]
key: uid32535, score: [0 6972 4909 9928]
key: uid46603, score: [0 7848 1696 5213]
key: uid62402, score: [0 8753 1181 7609]
key: uid17517, score: [1 83 1348 2717]
key: uid98571, score: [1 2830 9233 5445]

Update 100000 items, cost: 330 ms, avg: 203/ms  worker: 1
zset len: 100000
zset count (1000-2000): 10053
zset last 10:
key: uid91763, score: [10980 10032 5578 6559]
key: uid46402, score: [10978 6834 6505 7516]
key: uid54175, score: [10970 3837 4984 2524]
key: uid98367, score: [10965 9538 2536 7981]
key: uid377, score: [10964 6771 592 8404]
key: uid90975, score: [10964 1010 3899 3287]
key: uid33274, score: [10962 5599 2763 9581]
key: uid10684, score: [10961 4230 10224 3109]
key: uid50125, score: [10954 9605 10120 5323]
key: uid96041, score: [10951 5207 9917 817]
key: uid67304, score: [10944 5302 234 5507]

Add 1000000 items, cost: 4100 ms, avg: 243/ms  worker: 1
zset len: 1000000
zset count (100-500): 40097
zset top 10:
key: uid263827, score: [0 72 9629 3403]
key: uid612996, score: [0 176 8942 5492]
key: uid995466, score: [0 692 662 6283]
key: uid262071, score: [0 1020 4207 6628]
key: uid611398, score: [0 1633 2176 222]
key: uid316579, score: [0 1819 5476 4564]
key: uid179464, score: [0 1960 4098 6671]
key: uid325461, score: [0 1972 3775 4922]
key: uid19193, score: [0 2037 7028 5091]
key: uid41071, score: [0 2113 8024 5246]
key: uid733309, score: [0 2211 9525 3808]
Update 1000000 items, cost: 9137 ms, avg: 75/ms  worker: 1
zset len: 1000000
zset count (1000-2000): 100329
zset last 10:
key: uid27077, score: [10996 2797 3659 6699]
key: uid589504, score: [10995 3246 2265 4252]
key: uid864590, score: [10987 10558 5618 4775]
key: uid358353, score: [10987 2539 7893 6929]
key: uid294482, score: [10986 7741 9661 10716]
key: uid372966, score: [10986 5479 3986 1948]
key: uid430713, score: [10986 5177 5587 8018]
key: uid183247, score: [10985 9724 2532 5073]
key: uid189737, score: [10985 5428 9586 8115]
key: uid426635, score: [10985 4967 1275 6206]
key: uid832650, score: [10985 527 643 8769]
```