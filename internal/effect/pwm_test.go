package effect

import (
	"sync"
	"testing"
	"time"
)

// recorder enregistre chaque niveau écrit, de façon sûre entre goroutines.
type recorder struct {
	mu   sync.Mutex
	vals []int
}

func (r *recorder) Set(level int) error {
	r.mu.Lock()
	r.vals = append(r.vals, level)
	r.mu.Unlock()
	return nil
}

func (r *recorder) seen() map[int]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := map[int]int{}
	for _, v := range r.vals {
		m[v]++
	}
	return m
}

func TestIntegerLevelDoesNotBlink(t *testing.T) {
	r := &recorder{}
	p := New(r, 5*time.Millisecond)
	p.SetLevel(2)
	time.Sleep(30 * time.Millisecond)
	p.Stop()

	for v := range r.seen() {
		if v != 2 {
			t.Fatalf("niveau entier 2 : écriture inattendue de %d", v)
		}
	}
}

func TestFractionalLevelBlinksBetweenCrans(t *testing.T) {
	r := &recorder{}
	p := New(r, 5*time.Millisecond)
	p.SetLevel(1.5) // doit alterner entre 1 et 2
	time.Sleep(40 * time.Millisecond)
	p.Stop()

	seen := r.seen()
	if seen[1] == 0 || seen[2] == 0 {
		t.Fatalf("1,5 devrait alterner entre 1 et 2, vu : %v", seen)
	}
	for v := range seen {
		if v != 1 && v != 2 {
			t.Fatalf("1,5 ne devrait produire que 1 et 2, vu %d", v)
		}
	}
}

func TestStopFreezesOnNearestCran(t *testing.T) {
	r := &recorder{}
	p := New(r, 5*time.Millisecond)
	p.SetLevel(1.8)
	time.Sleep(20 * time.Millisecond)
	p.Stop()

	r.mu.Lock()
	last := r.vals[len(r.vals)-1]
	r.mu.Unlock()
	if last != 2 {
		t.Fatalf("après Stop à 1,8, dernier cran = %d, attendu 2", last)
	}
}
