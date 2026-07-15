package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"minotasus/internal/audio"
	"minotasus/internal/backlight"
	"minotasus/internal/effect"
	"minotasus/internal/scope"
)

// scopeColors : la forme d'onde change de couleur de temps en temps en tournant
// sur cette palette (codes ANSI, teintes vives).
var scopeColors = []string{
	"\033[96m", // cyan
	"\033[92m", // vert
	"\033[93m", // jaune
	"\033[95m", // magenta
	"\033[91m", // rouge
	"\033[94m", // bleu
}

// scopeColorEvery : durée pendant laquelle une couleur reste affichée avant de
// passer à la suivante.
const scopeColorEvery = 4 * time.Second

// runMusic capte le son qui sort du PC — à la fréquence de sortie, sans fichier
// à fournir — et en dessine la forme d'onde en plein écran (oscilloscope ASCII
// qui change de couleur de temps en temps), tout en faisant suivre le
// rétroéclairage du clavier. La touche q (ou Ctrl-C) quitte.
func runMusic(c *backlight.Controller, max int, period time.Duration, pulses int, app, sink string, gain float64, width, height, fps int) error {
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

	// Comme avant : une seule impulsion PWM suffit (l'image bouge trop vite pour
	// percevoir le grain), ce qui limite les écritures au contrôleur.
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
		fmt.Print("\033[0m\033[H\033[2J") // couleur par défaut + écran propre en sortant
	}
	defer cleanup()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	start := time.Now()
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

			// Plein écran : on prend toute la taille du terminal (moins une ligne
			// pour l'état), recalculée à chaque image au cas où on redimensionne.
			w, h := scopeSize(width, height)
			color := scopeColors[int(time.Since(start)/scopeColorEvery)%len(scopeColors)]
			fmt.Print(color, scope.Frame(block, w, h))
			fmt.Printf("\033[0m  ♪ %s @ %d Hz — q pour quitter", desc, opts.Rate)
		}
	}
}

// scopeSize choisit les dimensions de l'oscilloscope : les valeurs > 0 forcent
// une taille (flags -width/-height) ; sinon on remplit tout le terminal (une
// ligne réservée à l'état en bas), avec un repli si la taille est illisible.
func scopeSize(width, height int) (w, h int) {
	tw, th := termSize()
	w, h = width, height
	if w <= 0 {
		w = tw
	}
	if h <= 0 {
		h = th - 1 // garde une ligne pour l'état
	}
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 28
	}
	return w, h
}

// termSize renvoie la taille du terminal (colonnes, lignes) via « stty size »,
// ou (0, 0) si elle n'est pas lisible.
func termSize() (cols, rows int) {
	out, err := stty("size") // format "lignes colonnes"
	if err != nil {
		return 0, 0
	}
	if _, err := fmt.Sscan(strings.TrimSpace(out), &rows, &cols); err != nil {
		return 0, 0
	}
	return cols, rows
}
