package database

import "testing"

func TestPlayerAddPoints(t *testing.T) {
	p := &Player{Amount: 2000}

	if got := p.AddPoints(-250); got != -250 {
		t.Fatalf("expect -250, got %d", got)
	}
	if p.Points() != 1750 {
		t.Fatalf("expect 1750, got %d", p.Points())
	}

	if got := p.AddPoints(600); got != 600 {
		t.Fatalf("expect +600, got %d", got)
	}
	if p.Points() != 2350 {
		t.Fatalf("expect 2350, got %d", p.Points())
	}

	// 扣分不会把积分扣成负数
	if got := p.AddPoints(-100000); got != -2350 {
		t.Fatalf("expect -2350, got %d", got)
	}
	if p.Points() != 0 {
		t.Fatalf("expect 0, got %d", p.Points())
	}
}
