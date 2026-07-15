// Package scope dessine un oscilloscope ASCII d'un signal audio : chaque colonne
// du terminal représente l'amplitude d'un échantillon, ce qui fait apparaître la
// forme d'onde. Le rendu est une simple chaîne, ce qui le rend facile à tester
// et à imprimer telle quelle.
package scope

import "strings"

// home ramène le curseur en haut à gauche et efface l'écran, pour redessiner
// une image par-dessus la précédente sans faire défiler le terminal.
const home = "\033[H\033[2J"

// Frame construit une image de width colonnes sur height lignes représentant la
// forme d'onde de samples (valeurs attendues dans [-1, 1], les débordements sont
// bornés). La chaîne renvoyée commence par la séquence d'effacement puis les
// lignes séparées par des sauts de ligne : elle est prête à être imprimée.
func Frame(samples []float64, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	canvas := make([][]rune, height)
	for y := range canvas {
		canvas[y] = make([]rune, width)
		for x := range canvas[y] {
			canvas[y][x] = ' '
		}
	}

	// Une colonne par pas de largeur : on échantillonne le signal de gauche à
	// droite et on trace un point à la hauteur correspondant à son amplitude.
	if len(samples) > 0 {
		step := float64(len(samples)) / float64(width)
		for x := 0; x < width; x++ {
			i := int(step * float64(x))
			if i >= len(samples) {
				i = len(samples) - 1
			}
			// v ∈ [-1,1] → ligne 0 (haut) pour +1, ligne height-1 (bas) pour -1.
			v := clamp(samples[i], -1, 1)
			row := int((1 - (v+1)/2) * float64(height-1))
			canvas[row][x] = '*'
		}
	}

	var b strings.Builder
	b.Grow(len(home) + height*(width+1))
	b.WriteString(home)
	for _, line := range canvas {
		b.WriteString(string(line))
		b.WriteByte('\n')
	}
	return b.String()
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
