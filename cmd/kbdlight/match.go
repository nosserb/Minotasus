package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"minotasus/internal/backlight"
	"minotasus/internal/effect"
	"minotasus/internal/match"
	"minotasus/internal/show"
)

// runMatch suit un match en direct : la lumière « respire » au rythme de la
// tension et explose sur chaque but de l'équipe suivie. Touches : g (ou espace)
// = but manuel, +/- = ajuste la tension, q = quitter.
func runMatch(c *backlight.Controller, max int, period time.Duration, pulses int, team, event string, poll time.Duration, demo bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if pulses > musicMaxPulses {
		pulses = musicMaxPulses
	}
	pwm := effect.New(c, period, pulses)
	anim := show.New(pwm, max)

	restore, _ := rawMode()
	var cleaned bool
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		anim.Stop()
		pwm.Stop()
		if restore != nil {
			restore()
		}
	}
	defer cleanup()

	// Tension pilotée au clavier, mêlée à celle de l'API.
	manualTension := 0.0
	if restore != nil {
		go func() {
			buf := make([]byte, 4)
			for {
				nr, err := os.Stdin.Read(buf)
				if err != nil || nr == 0 {
					return
				}
				switch buf[0] {
				case 'q', 'Q', 3:
					cancel()
					return
				case 'g', 'G', ' ':
					anim.Goal()
				case '+', '=':
					manualTension = clampf(manualTension+0.15, 0, 1)
					anim.SetTension(manualTension)
				case '-', '_':
					manualTension = clampf(manualTension-0.15, 0, 1)
					anim.SetTension(manualTension)
				}
			}
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	if demo {
		fmt.Print("Mode match — DÉMO (aucune API). g: but  +/-: tension  q: quitter\r\n")
		runMatchDemo(ctx, anim)
		return nil
	}

	// Résolution du match.
	cl := match.NewClient()
	eid := event
	if eid == "" {
		fmt.Printf("Recherche du match de %s…\r\n", team)
		id, err := cl.TeamID(ctx, team)
		if err != nil {
			return err
		}
		if eid, err = cl.NextEventID(ctx, id); err != nil {
			return err
		}
	}
	st, err := cl.Lookup(ctx, eid)
	if err != nil {
		return err
	}
	ourHome := strings.Contains(strings.ToLower(st.HomeTeam), strings.ToLower(team))
	fmt.Printf("⚽ %s %d–%d %s  (%s)\r\n", st.HomeTeam, st.Home, st.Away, st.AwayTeam, st.Status)
	fmt.Print("g: but manuel  +/-: tension  q: quitter\r\n")

	events := make(chan match.Event, 8)
	go match.NewWatcher(cl, poll).Run(ctx, eid, events)

	for {
		select {
		case <-ctx.Done():
			cleanup()
			fmt.Print("\r\n")
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			switch ev.Kind {
			case match.GoalHome:
				onGoal(anim, ev.State, ev.State.HomeTeam, ourHome)
			case match.GoalAway:
				onGoal(anim, ev.State, ev.State.AwayTeam, !ourHome)
			case match.HalfTimeK:
				fmt.Print("\r\033[K— Mi-temps —\r\n")
			case match.FullTime:
				fmt.Print("\r\033[K— Fin du match —\r\n")
			case match.KickOff:
				fmt.Print("\r\033[K— Coup d'envoi ! —\r\n")
			}
			anim.SetTension(ev.Tension)
			drawMatch(ev)
		}
	}
}

// onGoal réagit à un but : célébration si c'est notre équipe, simple info sinon.
func onGoal(anim *show.Animator, st match.State, scorer string, ours bool) {
	if ours {
		anim.Goal()
		fmt.Printf("\r\033[K🎉 BUUUT %s ! %d–%d\r\n", scorer, st.Home, st.Away)
	} else {
		fmt.Printf("\r\033[K😳 But de %s… %d–%d\r\n", scorer, st.Home, st.Away)
	}
}

// drawMatch affiche une ligne d'état avec la jauge de tension.
func drawMatch(ev match.Event) {
	const w = 16
	on := int(ev.Tension*float64(w) + 0.5)
	bar := strings.Repeat("█", on) + strings.Repeat("·", w-on)
	min := ""
	if ev.State.Minute > 0 {
		min = fmt.Sprintf("%d'", ev.State.Minute)
	}
	fmt.Printf("\r\033[K  %s %d–%d %s  %-4s tension [%s] %3.0f%% ",
		ev.State.HomeTeam, ev.State.Home, ev.State.Away, ev.State.AwayTeam, min, bar, ev.Tension*100)
}

// runMatchDemo joue un scénario pour visualiser les animations sans match réel.
func runMatchDemo(ctx context.Context, anim *show.Animator) {
	steps := []struct {
		tension float64
		goal    bool
		hold    time.Duration
		label   string
	}{
		{0.15, false, 3 * time.Second, "début de match, calme"},
		{0.45, false, 3 * time.Second, "ça s'anime"},
		{0.55, true, 6 * time.Second, "BUT !"},
		{0.7, false, 3 * time.Second, "fin de première période tendue"},
		{0.9, false, 3 * time.Second, "money time"},
		{0.9, true, 6 * time.Second, "BUT décisif !"},
		{1.0, false, 4 * time.Second, "prolongations / tirs au but"},
	}
	for _, s := range steps {
		anim.SetTension(s.tension)
		if s.goal {
			anim.Goal()
		}
		fmt.Printf("\r\033[K  démo : %-32s tension %3.0f%% ", s.label, s.tension*100)
		if sleepCtx(ctx, s.hold) {
			return
		}
	}
	fmt.Print("\r\033[K  démo terminée\r\n")
}

// sleepCtx attend d ou l'annulation ; renvoie true si annulé.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-t.C:
		return false
	}
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
