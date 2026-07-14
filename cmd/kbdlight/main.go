// Commande kbdlight : petit programme interactif pour allumer et régler le
// rétroéclairage du clavier d'un portable ASUS Vivobook.
//
// Lance-le puis appuie sur une touche :
//
//	espace   allume / éteint (bascule)
//	+ / -    augmente / diminue le niveau
//	0..3     règle directement le niveau
//	q        quitte
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"minotasus/internal/backlight"
)

func main() {
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

	if !c.Writable() {
		fmt.Println("⚠  Écriture non autorisée : relance avec sudo, ou installe la règle udev (voir README).")
		fmt.Println()
	}

	if err := run(c, max); err != nil {
		fmt.Fprintln(os.Stderr, "Erreur :", err)
		os.Exit(1)
	}
}

// run met le terminal en mode brut, affiche l'état et réagit aux touches
// jusqu'à ce que l'utilisateur quitte.
func run(c *backlight.Controller, max int) error {
	restore, err := rawMode()
	if err != nil {
		return fmt.Errorf("passage en mode brut : %w", err)
	}
	defer restore()

	// Restaure le terminal même en cas de Ctrl-C brutal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		restore()
		fmt.Print("\r\n")
		os.Exit(0)
	}()

	// Dernier niveau non nul, mémorisé pour la bascule espace.
	last := max

	fmt.Print("Clavier lumineux — espace: on/off  +/-: niveau  0-3: direct  q: quitter\r\n")
	if lvl, _ := c.Get(); lvl > 0 {
		last = lvl
	}
	draw(c, max)

	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return nil
		}
		cur, _ := c.Get()
		target := cur
		quit := false

		switch buf[0] {
		case 'q', 'Q', 3: // 3 = Ctrl-C
			quit = true
		case ' ': // bascule
			if cur > 0 {
				last = cur
				target = 0
			} else {
				target = last
			}
		case '+', '=':
			target = cur + 1
		case '-', '_':
			target = cur - 1
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			target = int(buf[0] - '0')
		default:
			continue
		}

		if quit {
			fmt.Print("\r\n")
			return nil
		}

		if err := c.Set(target); err != nil {
			// Efface la ligne d'état et montre l'erreur sans casser l'affichage.
			fmt.Print("\r\033[K")
			return err
		}
		if target > 0 {
			last = target
		}
		draw(c, max)
	}
}

// draw réaffiche une jauge d'état sur la ligne courante.
func draw(c *backlight.Controller, max int) {
	lvl, _ := c.Get()
	bar := strings.Repeat("█", lvl) + strings.Repeat("·", max-lvl)
	state := "éteint"
	if lvl > 0 {
		state = "allumé"
	}
	// \r + effacement de ligne, pas de \n pour rester sur place.
	fmt.Printf("\r\033[K  [%s]  niveau %d/%d — %s ", bar, lvl, max, state)
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
