package show

import (
	"testing"
	"time"
)

// avgHeartbeat échantillonne le battement sur 2 s pour une tension donnée.
func avgHeartbeat(tension float64) (avg, max float64) {
	const n = 400
	start := time.Unix(1_000_000, 0)
	for i := 0; i < n; i++ {
		now := start.Add(time.Duration(i) * 5 * time.Millisecond)
		v := heartbeat(now, tension)
		avg += v
		if v > max {
			max = v
		}
	}
	return avg / n, max
}

func TestHeartbeatIntensifiesWithTension(t *testing.T) {
	calmAvg, _ := avgHeartbeat(0.0)
	hotAvg, _ := avgHeartbeat(1.0)
	if hotAvg <= calmAvg {
		t.Fatalf("tension haute (%.3f) devrait être plus lumineuse en moyenne que basse (%.3f)", hotAvg, calmAvg)
	}
}

func TestHeartbeatStaysInRange(t *testing.T) {
	for _, ten := range []float64{0, 0.5, 1} {
		_, max := avgHeartbeat(ten)
		if max > 1.0001 {
			t.Errorf("battement dépasse 1 (tension %.1f) : %.3f", ten, max)
		}
	}
}

func TestGoalStrobeIsBrightAndBounded(t *testing.T) {
	var lit int
	const n = 200
	for i := 0; i < n; i++ {
		e := time.Duration(i) * 10 * time.Millisecond // 0..2s
		v := goalLevel(e)
		if v < 0 || v > 1.0001 {
			t.Fatalf("goalLevel hors [0,1] : %.3f", v)
		}
		if v > 0.9 {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("le stroboscope de but devrait atteindre le plein éclat")
	}
}
