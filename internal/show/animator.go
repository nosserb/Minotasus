// Package show transforme la tension d'un match et les buts en animations de
// luminosité pour le rétroéclairage (clavier mono-zone : on joue sur le temps).
package show

import (
	"math"
	"sync"
	"time"
)

// Renderer reçoit un niveau fractionnaire (0..max matériel) à afficher.
type Renderer interface {
	SetLevel(level float64)
}

// goalDuration est la durée de l'animation de but.
const goalDuration = 6 * time.Second

// Animator pilote en continu un Renderer : au repos il joue un « battement de
// cœur » dont le rythme et l'ampleur montent avec la tension ; sur un but il
// déclenche une explosion lumineuse.
type Animator struct {
	r   Renderer
	max float64

	mu        sync.Mutex
	tension   float64
	goalStart time.Time // dernier but déclenché (zéro = aucun en cours)

	stop chan struct{}
	done chan struct{}
}

// New démarre un Animator qui écrit sur r, avec max le niveau matériel maximum.
func New(r Renderer, max int) *Animator {
	a := &Animator{
		r:    r,
		max:  float64(max),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go a.loop()
	return a
}

// SetTension fixe la tension courante (0..1).
func (a *Animator) SetTension(t float64) {
	a.mu.Lock()
	a.tension = clamp01(t)
	a.mu.Unlock()
}

// Goal déclenche l'animation de but.
func (a *Animator) Goal() {
	a.mu.Lock()
	a.goalStart = time.Now()
	a.mu.Unlock()
}

// Stop arrête l'animation et éteint la lumière.
func (a *Animator) Stop() {
	close(a.stop)
	<-a.done
	a.r.SetLevel(0)
}

func (a *Animator) loop() {
	defer close(a.done)
	tick := time.NewTicker(20 * time.Millisecond) // ~50 images/s
	defer tick.Stop()
	for {
		select {
		case <-a.stop:
			return
		case now := <-tick.C:
			a.mu.Lock()
			tension := a.tension
			goalStart := a.goalStart
			a.mu.Unlock()

			var level float64
			if !goalStart.IsZero() {
				if e := now.Sub(goalStart); e < goalDuration {
					level = goalLevel(e)
				} else {
					a.mu.Lock()
					a.goalStart = time.Time{} // but terminé
					a.mu.Unlock()
					level = heartbeat(now, tension)
				}
			} else {
				level = heartbeat(now, tension)
			}
			a.r.SetLevel(clamp01(level) * a.max)
		}
	}
}

// heartbeat renvoie un niveau [0,1] en forme de battement de cœur. Le rythme
// (BPM) et l'amplitude croissent avec la tension : calme et lent au repos,
// rapide et fort quand ça chauffe.
func heartbeat(now time.Time, tension float64) float64 {
	bpm := lerp(45, 150, tension)
	period := 60.0 / bpm
	// Position dans le cycle courant, en secondes puis en phase [0,1).
	sec := float64(now.UnixNano()) / 1e9
	phase := math.Mod(sec, period) / period

	// « poum-poum » : deux bosses, la seconde plus faible.
	shape := gauss(phase, 0.0, 0.06) + 0.55*gauss(phase, 0.18, 0.05)
	base := 0.12
	amp := lerp(0.25, 1.0, tension)
	return clamp01(base + amp*shape)
}

// goalLevel décrit l'explosion de but sur goalDuration : stroboscope éclatant,
// puis pulsations triomphales qui retombent.
func goalLevel(elapsed time.Duration) float64 {
	e := elapsed.Seconds()
	if e < 2.0 {
		// Stroboscope ~6 Hz à fond.
		if int(e*12)%2 == 0 {
			return 1.0
		}
		return 0.05
	}
	// 2s → 6s : grosses pulsations qui s'estompent vers le battement de fond.
	fade := 1.0 - (e-2.0)/4.0 // 1 → 0
	pulse := math.Abs(math.Sin(2 * math.Pi * (e - 2.0) * 2.0))
	return 0.15 + 0.85*pulse*fade
}

func gauss(x, center, width float64) float64 {
	d := (x - center) / width
	return math.Exp(-d * d)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*clamp01(t) }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
