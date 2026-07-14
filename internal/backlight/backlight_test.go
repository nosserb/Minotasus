package backlight

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeLED crée un faux dossier LED sysfs dans un répertoire temporaire.
func fakeLED(t *testing.T, brightness, max string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "brightness"), []byte(brightness), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "max_brightness"), []byte(max), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGetMax(t *testing.T) {
	c := NewAt(fakeLED(t, "2\n", "3\n"))
	if got, _ := c.Get(); got != 2 {
		t.Errorf("Get() = %d, want 2", got)
	}
	if got, _ := c.Max(); got != 3 {
		t.Errorf("Max() = %d, want 3", got)
	}
}

func TestSetClamps(t *testing.T) {
	c := NewAt(fakeLED(t, "0", "3"))
	for _, tc := range []struct{ in, want int }{{-5, 0}, {1, 1}, {9, 3}} {
		if err := c.Set(tc.in); err != nil {
			t.Fatal(err)
		}
		if got, _ := c.Get(); got != tc.want {
			t.Errorf("Set(%d) -> Get() = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSetDeduplicates(t *testing.T) {
	dir := fakeLED(t, "0", "3")
	c := NewAt(dir)
	if err := c.Set(2); err != nil {
		t.Fatal(err)
	}
	// Rend le fichier non inscriptible : une écriture réelle échouerait.
	bf := filepath.Join(dir, "brightness")
	if err := os.Chmod(bf, 0o444); err != nil {
		t.Fatal(err)
	}
	// Même niveau → ignoré → pas d'erreur (aucune écriture tentée).
	if err := c.Set(2); err != nil {
		t.Errorf("Set(2) répété devrait être ignoré, got %v", err)
	}
	// Niveau différent → tentative d'écriture → doit échouer (lecture seule).
	if err := c.Set(1); err == nil {
		t.Errorf("Set(1) sur fichier lecture seule devrait échouer")
	}
}

func TestAvailable(t *testing.T) {
	if NewAt(fakeLED(t, "0", "3")).Available() != true {
		t.Error("Available() = false, want true")
	}
	if NewAt(t.TempDir()).Available() != false {
		t.Error("Available() = true sur dossier vide, want false")
	}
}
