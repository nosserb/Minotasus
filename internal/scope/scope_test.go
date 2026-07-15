package scope

import (
	"strings"
	"testing"
)

// lines renvoie les lignes du canevas, sans la séquence d'effacement initiale.
func lines(t *testing.T, frame string) []string {
	t.Helper()
	body := strings.TrimPrefix(frame, home)
	return strings.Split(strings.TrimRight(body, "\n"), "\n")
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
	// Largeur = nombre d'échantillons → une colonne par échantillon, pas de
	// lissage (radius 0). +1 se trace tout en haut, -1 tout en bas.
	f := Frame([]float64{1, -1}, 2, 5)
	ls := lines(t, f)
	if !strings.ContainsRune(ls[0], wave) {
		t.Errorf("+1 devrait marquer la ligne du haut : %q", ls[0])
	}
	if !strings.ContainsRune(ls[len(ls)-1], wave) {
		t.Errorf("-1 devrait marquer la ligne du bas : %q", ls[len(ls)-1])
	}
}

func TestFrameConnectsPoints(t *testing.T) {
	// Deux points opposés doivent être RELIÉS : la colonne du saut est remplie de
	// haut en bas (plus de « points » isolés).
	f := Frame([]float64{1, -1}, 2, 5)
	ls := lines(t, f)
	filled := 0
	for _, l := range ls {
		if []rune(l)[1] == wave {
			filled++
		}
	}
	if filled != len(ls) {
		t.Errorf("la colonne du saut devrait être pleine (%d/%d)", filled, len(ls))
	}
}

func TestFrameZeroSamplesDrawsBaseline(t *testing.T) {
	// Sans échantillon : une ligne de base au centre, pas un écran vide.
	f := Frame(nil, 3, 3)
	ls := lines(t, f)
	if len(ls) != 3 {
		t.Fatalf("%d lignes, veut 3", len(ls))
	}
	if strings.ContainsRune(ls[0], wave) || strings.ContainsRune(ls[2], wave) {
		t.Error("la ligne de base ne devrait toucher que le centre")
	}
	if !strings.ContainsRune(ls[1], wave) {
		t.Error("le centre devrait porter la ligne de base")
	}
}

func TestFrameClampsOutOfRange(t *testing.T) {
	// Des valeurs hors [-1,1] ne débordent pas (pas de panique) et restent bornées.
	f := Frame([]float64{9, -9}, 2, 4)
	ls := lines(t, f)
	if !strings.ContainsRune(ls[0], wave) {
		t.Errorf("valeur > 1 bornée en haut : %q", ls[0])
	}
	if !strings.ContainsRune(ls[len(ls)-1], wave) {
		t.Errorf("valeur < -1 bornée en bas : %q", ls[len(ls)-1])
	}
}
