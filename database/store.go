package database

import "sync"

// store 是一个带读写锁的泛型 map，用来替代第三方 hashmap 包。
//
// 换掉它的原因：awesome-cap/hashmap 在 Set 里先调 resize()，而 resize 判断
// dilate() 时读的 size / table.len() 并没有加锁，和别的 goroutine 的原子写
// 构成数据竞争（go test -race 能稳定复现）。并发建连时不仅有 race，扩容瞬间
// 还可能读到正在被替换的旧表。这里用最直白的 RWMutex + 内置 map，行为可预期。
//
// 注意：所有遍历都先取快照再执行回调，回调里可以安全地继续读写 store，
// 不会死锁，也不会边遍历边修改同一个 map。
type store[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func newStore[K comparable, V any]() *store[K, V] {
	return &store[K, V]{m: make(map[K]V)}
}

func (s *store[K, V]) Get(k K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *store[K, V]) Set(k K, v V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

func (s *store[K, V]) Del(k K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
}

func (s *store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// Snapshot 返回一份浅拷贝，用于需要遍历的场景。
func (s *store[K, V]) Snapshot() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[K]V, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

// Values 返回所有值（顺序不确定）。
func (s *store[K, V]) Values() []V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]V, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, v)
	}
	return out
}

// memberSet 是房间成员集合（座位上的人 / 旁观的人），自带读写锁。
//
// 之前成员直接是一个裸 map，广播、渲染、观战列表在别的协程里 range 它，
// 而进房/离房在房间锁里改它，既会触发 race，也可能撞上
// "concurrent map iteration and map write" 直接 panic。
// 这里把锁收进集合内部：读的一方做快照就走，不需要房间锁。
type memberSet struct {
	mu    sync.RWMutex
	m     map[int64]int // 值表示加入顺序，旁观补位按这个顺序来
	order int
}

func newMemberSet() *memberSet {
	return &memberSet{m: make(map[int64]int)}
}

func (s *memberSet) Add(id int64) {
	if s == nil { // 房间可能已经被清理掉，静默忽略
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; !ok {
		s.order++
		s.m[id] = s.order
	}
}

func (s *memberSet) Del(id int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
}

func (s *memberSet) Has(id int64) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[id]
	return ok
}

func (s *memberSet) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// IDs 返回成员 id 的快照。
func (s *memberSet) IDs() []int64 {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int64, 0, len(s.m))
	for id := range s.m {
		out = append(out, id)
	}
	return out
}

// Copy 返回 "id → 加入顺序" 的快照，旁观补位需要按顺序排。
func (s *memberSet) Copy() map[int64]int {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64]int, len(s.m))
	for id, order := range s.m {
		out[id] = order
	}
	return out
}
