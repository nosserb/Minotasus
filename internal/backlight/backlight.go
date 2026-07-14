// Package backlight pilote le rétroéclairage du clavier via le sysfs Linux.
//
// Sur les portables ASUS (Vivobook, ZenBook…), le module noyau asus_nb_wmi
// expose le rétroéclairage sous /sys/class/leds/asus::kbd_backlight. Le niveau
// se lit et s'écrit dans le fichier "brightness" (0 = éteint jusqu'à
// max_brightness, en général 3).
package backlight

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// DefaultPath est l'emplacement du LED sur les portables ASUS.
const DefaultPath = "/sys/class/leds/asus::kbd_backlight"

// Controller pilote un LED de rétroéclairage repéré par son dossier sysfs.
type Controller struct {
	dir string

	mu       sync.Mutex
	maxVal   int  // max_brightness mis en cache (invariant)
	maxOK    bool // max_brightness déjà lu ?
	last     int  // dernier niveau écrit (pour éviter les écritures redondantes)
	haveLast bool
}

// New renvoie un Controller pour le chemin ASUS par défaut.
func New() *Controller {
	return &Controller{dir: DefaultPath}
}

// NewAt renvoie un Controller pour un dossier LED sysfs arbitraire (tests,
// autres marques).
func NewAt(dir string) *Controller {
	return &Controller{dir: dir}
}

// Available indique si le LED est présent (matériel + module noyau chargé).
func (c *Controller) Available() bool {
	_, err := os.Stat(filepath.Join(c.dir, "brightness"))
	return err == nil
}

// Max renvoie le niveau maximal accepté par le rétroéclairage. La valeur est
// invariante : on ne lit le fichier qu'une fois.
func (c *Controller) Max() (int, error) {
	c.mu.Lock()
	if c.maxOK {
		v := c.maxVal
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	v, err := c.readInt("max_brightness")
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	c.maxVal, c.maxOK = v, true
	c.mu.Unlock()
	return v, nil
}

// Get renvoie le niveau courant.
func (c *Controller) Get() (int, error) {
	return c.readInt("brightness")
}

// Set écrit un niveau, borné entre 0 et max_brightness. Une écriture identique à
// la précédente est ignorée : inutile de solliciter le contrôleur (ACPI/WMI)
// pour rien, ce qui allège fortement la charge en mode musique.
func (c *Controller) Set(level int) error {
	max, err := c.Max()
	if err != nil {
		return err
	}
	if level < 0 {
		level = 0
	}
	if level > max {
		level = max
	}

	c.mu.Lock()
	if c.haveLast && c.last == level {
		c.mu.Unlock()
		return nil // rien à faire : déjà à ce niveau
	}
	c.mu.Unlock()

	path := filepath.Join(c.dir, "brightness")
	if err := os.WriteFile(path, []byte(strconv.Itoa(level)), 0o644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("écriture refusée sur %s : lance le programme avec sudo, ou installe la règle udev (voir README)", path)
		}
		return err
	}

	c.mu.Lock()
	c.last, c.haveLast = level, true
	c.mu.Unlock()
	return nil
}

// Writable teste si le niveau peut être modifié sans droits root.
func (c *Controller) Writable() bool {
	f, err := os.OpenFile(filepath.Join(c.dir, "brightness"), os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func (c *Controller) readInt(name string) (int, error) {
	raw, err := os.ReadFile(filepath.Join(c.dir, name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(raw)))
}
