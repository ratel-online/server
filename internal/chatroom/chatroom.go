// Package chatroom provides word-pair generation for "Who Is Undercover".
//
// 从 github.com/lipp12138/chatroom vendor 进来（原仓库 go.mod 声明 go 1.24，
// 且使用了 Go 1.18 的 strings.Cut，为了兼容 Go 1.17 部署环境改为内置实现）。
package chatroom

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Pair is one civilian/undercover word pair.
type Pair struct {
	Civilian   string `json:"civilian"`
	Undercover string `json:"undercover"`
}

var (
	errNoPairs = errors.New("chatroom: no word pairs")

	defaultPicker = mustNewPicker(defaultPairs)
	activePicker  = defaultPicker
	activeMu      sync.RWMutex
)

type picker struct {
	pairs []Pair
	rng   *rand.Rand
	mu    sync.Mutex
}

// Update loads a local word library into memory.
//
// Supported formats:
//   - .txt: one pair per line, for example: 苹果,梨
//   - .json: [{"civilian":"苹果","undercover":"梨"}]
//
// If loading fails, Update switches back to the built-in default library and
// returns the error. Pick never reads files; it only uses the in-memory library.
func Update(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		setActive(defaultPicker)
		return nil
	}

	p, err := loadFile(path)
	if err != nil {
		setActive(defaultPicker)
		return err
	}

	setActive(p)
	return nil
}

// Pick returns a random word pair from the in-memory word library.
func Pick() (Pair, error) {
	activeMu.RLock()
	p := activePicker
	activeMu.RUnlock()

	return p.pick()
}

func setActive(p *picker) {
	activeMu.Lock()
	activePicker = p
	activeMu.Unlock()
}

func loadFile(path string) (*picker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(filepath.Ext(path), ".json") {
		return loadJSON(data)
	}
	return loadText(data)
}

func loadJSON(data []byte) (*picker, error) {
	var pairs []Pair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return nil, err
	}
	return newPicker(pairs)
}

func loadText(data []byte) (*picker, error) {
	lines := strings.Split(string(data), "\n")
	pairs := make([]Pair, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pair, ok := parseTextLine(line)
		if ok {
			pairs = append(pairs, pair)
		}
	}

	return newPicker(pairs)
}

// cut 是 strings.Cut 的 Go 1.17 兼容实现。
func cut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

func parseTextLine(line string) (Pair, bool) {
	for _, sep := range []string{",", "，", "|", "\t"} {
		left, right, ok := cut(line, sep)
		if !ok {
			continue
		}

		pair := Pair{
			Civilian:   strings.TrimSpace(left),
			Undercover: strings.TrimSpace(right),
		}
		return pair, pair.Civilian != "" && pair.Undercover != ""
	}

	return Pair{}, false
}

func newPicker(pairs []Pair) (*picker, error) {
	cleaned := make([]Pair, 0, len(pairs))
	seen := make(map[string]struct{}, len(pairs))

	for _, pair := range pairs {
		pair.Civilian = strings.TrimSpace(pair.Civilian)
		pair.Undercover = strings.TrimSpace(pair.Undercover)
		if pair.Civilian == "" || pair.Undercover == "" || pair.Civilian == pair.Undercover {
			continue
		}
		key := pair.Civilian + "\x00" + pair.Undercover
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, pair)
	}

	if len(cleaned) == 0 {
		return nil, errNoPairs
	}

	return &picker{
		pairs: cleaned,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (p *picker) pick() (Pair, error) {
	if p == nil || len(p.pairs) == 0 {
		return Pair{}, errNoPairs
	}

	p.mu.Lock()
	pair := p.pairs[p.rng.Intn(len(p.pairs))]
	p.mu.Unlock()

	return pair, nil
}

func mustNewPicker(pairs []Pair) *picker {
	picker, err := newPicker(pairs)
	if err != nil {
		panic(err)
	}
	return picker
}
