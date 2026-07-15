package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"minotasus/internal/audio"
	"minotasus/internal/backlight"
	"minotasus/internal/effect"
	"minotasus/internal/scope"
)

// runScope capte le son qui sort du PC — à la fréquence de sortie, sans fichier
// à fournir — et en dessine la forme d'onde (oscilloscope ASCII) tout en faisant
// suivre le rétroéclairage du clavier. La touche q (ou Ctrl-C) quitte.
func runScope(c *backlight.Controller, max int, period time.Duration, pulses int, app, sink string, gain float64, width, height, fps int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts, desc, err := resolveTarget(app, sink)
	if err != nil {
		return err
	}
	opts.Gain = gain
	// On capte à la fréquence du graphe PipeWire : aucun rééchantillonnage, et
	// l'oscilloscope suit exactement la sortie quelle que soit sa fréquence.
	opts.Rate = audio.DefaultRate()
	if fps < 1 {
		fps = 1
	}
	opts.Block = opts.Rate / fps // un bloc par image → cadence pilotée par la capture

	mon := audio.NewMonitor(opts)
	frames, stopAudio, err := mon.Frames(ctx)
	if err != nil {
		return fmt.Errorf("capture audio : %w", err)
	}
	defer stopAudio()

	// Comme en mode musique : une seule impulsion PWM suffit (l'image bouge trop
	// vite pour percevoir le grain), ce qui limite les écritures au contrôleur.
	if pulses > musicMaxPulses {
		pulses = musicMaxPulses
	}
	pwm := effect.New(c, period, pulses)

	// L'analyseur transforme chaque bloc en un niveau [0,1] pour la lumière.
	an := audio.NewAnalyzer(audio.Params{Rate: opts.Rate, Gain: gain})

	// Lecture clavier en mode brut pour pouvoir quitter avec q.
	restore, err := rawMode()
	if err == nil {
		defer restore()
		go func() {
			buf := make([]byte, 4)
			for {
				n, e := os.Stdin.Read(buf)
				if e != nil || n == 0 {
					return
				}
				if b := buf[0]; b == 'q' || b == 'Q' || b == 3 {
					cancel()
					return
				}
			}
		}()
	}

	var cleaned bool
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		pwm.Stop()
		if restore != nil {
			restore()
		}
		fmt.Print("\033[H\033[2J") // laisse un écran propre en sortant
	}
	defer cleanup()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
			cleanup()
			return nil
		case block, ok := <-frames:
			if !ok {
				cleanup()
				return errors.New("flux audio interrompu (pw-record arrêté ?)")
			}
			pwm.SetLevel(an.Process(block) * float64(max))
			fmt.Print(scope.Frame(block, width, height))
			fmt.Printf("  oscilloscope — %s @ %d Hz — q pour quitter", desc, opts.Rate)
		}
	}
}
