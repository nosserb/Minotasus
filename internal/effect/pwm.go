// Package effect crée des niveaux de luminosité intermédiaires que le matériel
// ne propose pas, en clignotant très vite entre deux crans (PWM logiciel).
//
// Le clavier ASUS n'accepte que des niveaux entiers (0..max, souvent 0..3).
// Pour obtenir par exemple « 1,5 », on alterne rapidement entre 1 et 2 : si on
// passe 50 % du temps à 2 et 50 % à 1, l'œil perçoit une luminosité moyenne.
// En jouant sur ce rapport (duty cycle) on obtient un continuum de niveaux.
package effect

import (
	"sync"
	"time"
)

// Setter est ce dont le PWM a besoin pour piloter le matériel.
type Setter interface {
	Set(level int) error
}

// PWM alterne en tâche de fond entre deux crans matériels pour simuler un
// niveau fractionnaire. Un seul PWM doit piloter un LED donné à la fois.
type PWM struct {
	set    Setter
	period time.Duration
	pulses int // nombre d'impulsions par cycle (répartit le temps « allumé »)

	mu    sync.Mutex
	level float64 // niveau fractionnaire visé, dans [0, hardware max]

	stop chan struct{}
	done chan struct{}
}

// New démarre un moteur PWM. period est la durée d'un cycle complet ; pulses
// découpe ce cycle en autant d'impulsions courtes, ce qui monte la fréquence
// perçue et réduit le scintillement (au prix de plus d'écritures). pulses < 1
// est ramené à 1.
func New(set Setter, period time.Duration, pulses int) *PWM {
	if pulses < 1 {
		pulses = 1
	}
	p := &PWM{
		set:    set,
		period: period,
		pulses: pulses,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go p.loop()
	return p
}

// SetLevel change le niveau fractionnaire visé. La valeur entière (ex. 2.0)
// n'entraîne aucun clignotement : le cran est écrit tel quel.
func (p *PWM) SetLevel(level float64) {
	p.mu.Lock()
	p.level = level
	p.mu.Unlock()
}

// Level renvoie le niveau fractionnaire courant.
func (p *PWM) Level() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.level
}

// Stop arrête le clignotement et fige le matériel sur le cran entier le plus
// proche du dernier niveau visé.
func (p *PWM) Stop() {
	close(p.stop)
	<-p.done
}

func (p *PWM) loop() {
	defer close(p.done)
	for {
		p.mu.Lock()
		level := p.level
		p.mu.Unlock()

		lo := int(level)            // cran bas
		frac := level - float64(lo) // part passée sur le cran haut

		if frac <= 0.001 {
			// Niveau entier : rien à clignoter, on écrit et on attend un cycle.
			_ = p.set.Set(lo)
			if p.sleep(p.period) {
				_ = p.set.Set(lo)
				return
			}
			continue
		}

		// Un cycle = p.pulses impulsions courtes. Chaque impulsion passe la
		// fraction « frac » de son temps sur le cran haut. Répartir ainsi le
		// temps allumé multiplie la fréquence perçue par p.pulses.
		slot := p.period / time.Duration(p.pulses)
		high := slot * time.Duration(frac*1000) / 1000
		low := slot - high
		for i := 0; i < p.pulses; i++ {
			_ = p.set.Set(lo + 1)
			if p.sleep(high) {
				_ = p.set.Set(round(level))
				return
			}
			_ = p.set.Set(lo)
			if p.sleep(low) {
				_ = p.set.Set(round(level))
				return
			}
		}
	}
}

// sleep attend d gérée par le canal d'arrêt. Renvoie true si on doit s'arrêter.
func (p *PWM) sleep(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-p.stop:
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.stop:
		return true
	case <-t.C:
		return false
	}
}

func round(f float64) int {
	return int(f + 0.5)
}
