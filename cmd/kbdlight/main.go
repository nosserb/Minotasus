// Commande kbdlight : petit programme interactif pour allumer et régler le
// rétroéclairage du clavier d'un portable ASUS Vivobook.
//
// Le matériel n'a que quelques crans (0..3). Pour obtenir des niveaux
// intermédiaires, le programme clignote très vite entre deux crans (PWM
// logiciel, paquet internal/effect) : l'œil perçoit une luminosité moyenne.
//
// Lance-le puis appuie sur une touche :
//
//	espace   allume / éteint (bascule)
//	+ / -    monte / descend d'un cran fin (niveaux intermédiaires)
//	0..3     règle directement un cran matériel plein
//	q        quitte
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"minotasus/internal/backlight"
	"minotasus/internal/effect"
)

// subdivisions découpe chaque cran matériel en sous-niveaux via PWM.
// Avec max=3 et 4 subdivisions, on passe de 4 à 13 niveaux perçus.
const subdivisions = 4

func main() {
	period := flag.Duration("period", 15*time.Millisecond, "durée d'un cycle PWM (plus court = moins de scintillement)")
	pulses := flag.Int("pulses", 0, "impulsions par cycle (0 = auto selon la latence du clavier)")
	flag.Parse()

	c := backlight.New()

	if !c.Available() {
		fmt.Fprintln(os.Stderr, "Rétroéclairage introuvable en", backlight.DefaultPath)
		fmt.Fprintln(os.Stderr, "Vérifie que le module noyau asus_nb_wmi est chargé (lsmod | grep asus).")
		os.Exit(1)
	}

	max, err := c.Max()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Lecture du niveau max impossible :", err)
		os.Exit(1)
	}

	// Vérifie tout de suite qu'on peut écrire : sinon le PWM tournerait dans
	// le vide sans jamais allumer le clavier.
	if cur, _ := c.Get(); c.Set(cur) != nil {
		fmt.Fprintln(os.Stderr, "Écriture refusée sur le rétroéclairage.")
		fmt.Fprintln(os.Stderr, "Relance avec sudo, ou installe la règle udev (voir README).")
		os.Exit(1)
	}

	// Choisit le nombre d'impulsions : soit imposé, soit déduit de la latence
	// d'écriture réelle du clavier pour ne pas saturer son contrôleur.
	n := *pulses
	if n <= 0 {
		lat := measureWriteLatency(c, max)
		n = autoPulses(*period, lat)
		fmt.Printf("Latence d'écriture ~%s → %d impulsion(s)/cycle (règle avec -pulses).\n", lat.Round(time.Microsecond), n)
	}

	if err := run(c, max, *period, n); err != nil {
		fmt.Fprintln(os.Stderr, "Erreur :", err)
		os.Exit(1)
	}
}

// measureWriteLatency estime le temps d'une écriture sur le rétroéclairage en
// alternant deux crans (l'EC ignore souvent une réécriture identique). Restaure
// le niveau de départ après mesure.
func measureWriteLatency(c *backlight.Controller, max int) time.Duration {
	cur, _ := c.Get()
	a, b := cur, cur
	if cur > 0 {
		b = cur - 1
	} else if max > 0 {
		b = cur + 1
	}
	const n = 8
	start := time.Now()
	for i := 0; i < n; i++ {
		_ = c.Set(a)
		_ = c.Set(b)
	}
	elapsed := time.Since(start)
	_ = c.Set(cur)
	return elapsed / (2 * n)
}

// autoPulses déduit un nombre d'impulsions tenant dans le cycle : chaque
// impulsion coûte deux écritures (allumé puis éteint), on garde donc un créneau
// d'au moins ~2× la latence, avec une marge.
func autoPulses(period, latency time.Duration) int {
	if latency <= 0 {
		return 4
	}
	minSlot := 2 * latency * 3 / 2 // 2 écritures + 50 % de marge
	n := int(period / minSlot)
	return clamp(n, 1, 8)
}

// run met le terminal en mode brut, pilote le PWM et réagit aux touches
// jusqu'à ce que l'utilisateur quitte.
func run(c *backlight.Controller, max int, period time.Duration, pulses int) error {
	restore, err := rawMode()
	if err != nil {
		return fmt.Errorf("passage en mode brut : %w", err)
	}

	pwm := effect.New(c, period, pulses)

	// Nettoyage commun (terminal + PWM), idempotent.
	var cleaned bool
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		pwm.Stop()
		restore()
	}
	defer cleanup()

	// Restaure le terminal même en cas de Ctrl-C brutal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cleanup()
		fmt.Print("\r\n")
		os.Exit(0)
	}()

	fineMax := max * subdivisions

	// Cran fin courant, déduit du niveau matériel de départ.
	cur, _ := c.Get()
	fine := cur * subdivisions
	last := fineMax // dernier niveau non nul, pour la bascule espace
	if fine > 0 {
		last = fine
	}
	pwm.SetLevel(float64(fine) / subdivisions)

	fmt.Print("Clavier lumineux — espace: on/off  +/-: niveau fin  0-3: cran plein  q: quitter\r\n")
	draw(fine, fineMax)

	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return nil
		}

		switch buf[0] {
		case 'q', 'Q', 3: // 3 = Ctrl-C
			fmt.Print("\r\n")
			return nil
		case ' ': // bascule on/off
			if fine > 0 {
				last = fine
				fine = 0
			} else {
				fine = last
			}
		case '+', '=':
			fine++
		case '-', '_':
			fine--
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			fine = int(buf[0]-'0') * subdivisions
		default:
			continue
		}

		fine = clamp(fine, 0, fineMax)
		if fine > 0 {
			last = fine
		}
		pwm.SetLevel(float64(fine) / subdivisions)
		draw(fine, fineMax)
	}
}

// draw réaffiche une jauge d'état sur la ligne courante.
func draw(fine, fineMax int) {
	bar := strings.Repeat("█", fine) + strings.Repeat("·", fineMax-fine)
	state := "éteint"
	if fine > 0 {
		state = "allumé"
	}
	// \r + effacement de ligne, pas de \n pour rester sur place.
	fmt.Printf("\r\033[K  [%s]  %d/%d — %s ", bar, fine, fineMax, state)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// rawMode configure le terminal pour lire les touches une par une, sans écho,
// et renvoie une fonction de restauration idempotente.
func rawMode() (func(), error) {
	if !isTerminal() {
		return nil, errors.New("l'entrée n'est pas un terminal")
	}
	saved, err := stty("-g")
	if err != nil {
		return nil, err
	}
	if _, err := stty("-echo", "-icanon", "min", "1", "time", "0"); err != nil {
		return nil, err
	}
	var done bool
	return func() {
		if done {
			return
		}
		done = true
		_, _ = stty(strings.Fields(strings.TrimSpace(saved))...)
	}, nil
}

// stty pilote le terminal via /dev/tty et renvoie sa sortie standard.
func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return "", err
	}
	defer tty.Close()
	cmd.Stdin = tty
	out, err := cmd.Output()
	return string(out), err
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
