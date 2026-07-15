package scope

import (
	"strings"
	"testing"
)

// lines renvoie les lignes du canevas, sans la séquence d'effacement initiale.
func lines(t *testing.T, frame string) []string {
	t.Helper()
	body := strings.TrimPrefix(frame, home)
	ls := strings.Split(strings.TrimRight(body, "\n"), "\n")
	return ls
}

func TestFrameDimensions(t *testing.T) {
	f := Frame([]float64{0, 0, 0}, 10, 4)
	if !strings.HasPrefix(f, home) {
		t.Error("la frame devrait commencer par l'effacement d'écran")
	}
	ls := lines(t, f)
	if len(ls) != 4 {
		t.Fatalf("%d lignes, veut 4", len(ls))
	}
	for i, l := range ls {
		if len([]rune(l)) != 10 {
			t.Errorf("ligne %d : largeur %d, veut 10", i, len([]rune(l)))
		}
	}
}

func TestFramePlacesPeaks(t *testing.T) {
	// +1 doit se tracer tout en haut (ligne 0), -1 tout en bas (dernière ligne).
	f := Frame([]float64{1, -1}, 2, 5)
	ls := lines(t, f)
	if ls[0][0] != '*' {
		t.Errorf("+1 devrait être sur la ligne du haut : %q", ls[0])
	}
	if ls[len(ls)-1][1] != '*' {
		t.Errorf("-1 devrait être sur la ligne du bas : %q", ls[len(ls)-1])
	}
}

func TestFrameZeroSamples(t *testing.T) {
	// Aucun échantillon : canevas vide, mais dimensions correctes (pas de panique).
	f := Frame(nil, 3, 2)
	ls := lines(t, f)
	if len(ls) != 2 {
		t.Fatalf("%d lignes, veut 2", len(ls))
	}
	if strings.ContainsRune(f, '*') {
		t.Error("un canevas sans échantillon ne devrait rien tracer")
	}
}

func TestFrameClampsOutOfRange(t *testing.T) {
	// Des valeurs hors [-1,1] ne doivent pas déborder du canevas (pas de panique).
	f := Frame([]float64{9, -9}, 2, 4)
	ls := lines(t, f)
	if ls[0][0] != '*' {
		t.Errorf("valeur > 1 devrait être bornée en haut : %q", ls[0])
	}
	if ls[len(ls)-1][1] != '*' {
		t.Errorf("valeur < -1 devrait être bornée en bas : %q", ls[len(ls)-1])
	}
}
