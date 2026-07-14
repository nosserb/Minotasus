package match

import (
	"context"
	"time"
)

// Kind est le type d'un évènement de match.
type Kind int

const (
	Update    Kind = iota // simple rafraîchissement d'état (porte la tension)
	GoalHome              // but de l'équipe à domicile
	GoalAway              // but de l'équipe à l'extérieur
	KickOff               // coup d'envoi
	HalfTimeK             // mi-temps
	FullTime              // fin du match
)

// Event est émis par le Watcher à chaque changement notable.
type Event struct {
	Kind    Kind
	State   State
	Tension float64 // 0..1, niveau de tension courant
}

// Watcher interroge périodiquement un match et émet des évènements.
type Watcher struct {
	client   *Client
	interval time.Duration
}

// NewWatcher construit un Watcher. interval < 5s est ramené à 5s pour respecter
// l'API gratuite.
func NewWatcher(c *Client, interval time.Duration) *Watcher {
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return &Watcher{client: c, interval: interval}
}

// Run interroge le match eventID jusqu'à l'annulation du contexte et pousse les
// évènements sur out. Les erreurs réseau ponctuelles sont ignorées (on retente
// au tour suivant).
func (w *Watcher) Run(ctx context.Context, eventID string, out chan<- Event) {
	var prev State
	have := false
	for {
		if st, err := w.client.Lookup(ctx, eventID); err == nil {
			w.emit(prev, st, have, out)
			prev, have = st, true
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.interval):
		}
	}
}

// emit compare l'ancien et le nouvel état et envoie les évènements adéquats,
// suivis d'un Update portant la tension courante.
func (w *Watcher) emit(prev, cur State, have bool, out chan<- Event) {
	send := func(k Kind) {
		select {
		case out <- Event{Kind: k, State: cur, Tension: Tension(cur)}:
		default:
		}
	}
	if have {
		if cur.Home > prev.Home {
			send(GoalHome)
		}
		if cur.Away > prev.Away {
			send(GoalAway)
		}
		if cur.Phase == HalfTime && prev.Phase != HalfTime {
			send(HalfTimeK)
		}
		if cur.Phase == Live && prev.Phase == Scheduled {
			send(KickOff)
		}
		if cur.Phase == Finished && prev.Phase != Finished {
			send(FullTime)
		}
	}
	send(Update)
}

// Tension estime la tension d'un match dans [0,1] : d'autant plus forte que le
// score est serré et que la fin approche.
func Tension(s State) float64 {
	switch s.Phase {
	case Scheduled, Finished:
		return 0
	case HalfTime:
		return 0.2
	}

	diff := s.Home - s.Away
	if diff < 0 {
		diff = -diff
	}
	var closeness float64
	switch diff {
	case 0:
		closeness = 1.0
	case 1:
		closeness = 0.7
	case 2:
		closeness = 0.35
	default:
		closeness = 0.15
	}

	var timePressure float64
	switch m := s.Minute; {
	case m <= 45:
		timePressure = 0.25
	case m < 60:
		timePressure = 0.35
	case m < 75:
		timePressure = 0.5
	case m < 85:
		timePressure = 0.7
	case m < 90:
		timePressure = 0.85
	default:
		timePressure = 1.0 // 90e+, prolongations, tirs au but
	}

	t := 0.15 + closeness*timePressure
	if t > 1 {
		t = 1
	}
	return t
}
