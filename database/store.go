package database

import "sync"

// store 是一个带读写锁的 int64 → interface{} map，用来替代第三方 hashmap 包，
// 并保持 Go 1.17 兼容（不使用泛型）。
//
// 换掉 awesome-cap/hashmap 的原因：它在 Set 里先调 resize()，而 resize 判断
// dilate() 时读的 size / table.len() 并没有加锁，和别的 goroutine 的原子写
// 构成数据竞争（go test -race 能稳定复现）。这里用最直白的 RWMutex + 内置 map，
// 行为可预期。
//
// 为了让调用侧保持类型安全，按值类型提供四个薄封装：
// playerStore / roomStore / memberStore / kickedStore。
//
// 注意：所有遍历都先取快照再执行回调，回调里可以安全地继续读写 store，
// 不会死锁，也不会边遍历边修改同一个 map。
type store struct {
	mu sync.RWMutex
	m  map[int64]interface{}
}

func newStore() *store {
	return &store{m: make(map[int64]interface{})}
}

func (s *store) getRaw(k int64) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *store) setRaw(k int64, v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

func (s *store) delRaw(k int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
}

func (s *store) snapshotRaw() map[int64]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64]interface{}, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

// playerStore 存 *Player。
type playerStore struct{ store }

func newPlayerStore() *playerStore { return &playerStore{store: store{m: make(map[int64]interface{})}} }

func (s *playerStore) Get(k int64) (*Player, bool) {
	v, ok := s.getRaw(k)
	if !ok {
		return nil, false
	}
	return v.(*Player), true
}

func (s *playerStore) Set(k int64, v *Player) { s.setRaw(k, v) }

func (s *playerStore) Values() []*Player {
	raw := s.snapshotRaw()
	out := make([]*Player, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(*Player))
	}
	return out
}

// roomStore 存 *Room。
type roomStore struct{ store }

func newRoomStore() *roomStore { return &roomStore{store: store{m: make(map[int64]interface{})}} }

func (s *roomStore) Get(k int64) (*Room, bool) {
	v, ok := s.getRaw(k)
	if !ok {
		return nil, false
	}
	return v.(*Room), true
}

func (s *roomStore) Set(k int64, v *Room) { s.setRaw(k, v) }

func (s *roomStore) Del(k int64) { s.delRaw(k) }

func (s *roomStore) Values() []*Room {
	raw := s.snapshotRaw()
	out := make([]*Room, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(*Room))
	}
	return out
}

// memberStore 存 *memberSet。
type memberStore struct{ store }

func newMemberStore() *memberStore { return &memberStore{store: store{m: make(map[int64]interface{})}} }

func (s *memberStore) Get(k int64) (*memberSet, bool) {
	v, ok := s.getRaw(k)
	if !ok {
		return nil, false
	}
	return v.(*memberSet), true
}

func (s *memberStore) Set(k int64, v *memberSet) { s.setRaw(k, v) }

func (s *memberStore) Del(k int64) { s.delRaw(k) }

// kickedStore 存 map[int64]bool（房间踢人名单）。
type kickedStore struct{ store }

func newKickedStore() *kickedStore { return &kickedStore{store: store{m: make(map[int64]interface{})}} }

func (s *kickedStore) Get(k int64) (map[int64]bool, bool) {
	v, ok := s.getRaw(k)
	if !ok {
		return nil, false
	}
	return v.(map[int64]bool), true
}

func (s *kickedStore) Set(k int64, v map[int64]bool) { s.setRaw(k, v) }

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
