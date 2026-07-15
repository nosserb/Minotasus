// Package scope dessine un oscilloscope ASCII d'un signal audio : la forme
// d'onde défile de gauche à droite, chaque colonne du terminal montrant
// l'amplitude du son à cet instant.
//
// Un rendu naïf (un point par colonne, pris tel quel) donne un « nuage de
// points » illisible sur de la musique riche en aigus : deux échantillons
// voisins sautent d'un extrême à l'autre. Pour rester lisible tout le temps, on
//
//  1. réduit chaque colonne à la moyenne des échantillons qu'elle couvre
//     (décimation = on gomme le hachis des aigus, la forme des basses/médiums
//     ressort) ;
//  2. lisse légèrement la courbe horizontalement ;
//  3. relie les points par des segments verticaux pleins → un tracé continu.
package scope

import "strings"

// home ramène le curseur en haut à gauche et efface l'écran, pour redessiner
// une image par-dessus la précédente sans faire défiler le terminal.
const home = "\033[H\033[2J"

// wave est le caractère de tracé (bloc plein : trait franc et bien visible).
const wave = '█'

// displayGain amplifie un peu l'affichage pour que les passages moyens
// remplissent l'écran ; les crêtes sont bornées à [-1, 1] (léger écrêtage, à la
// manière d'un vrai oscilloscope poussé).
const displayGain = 1.5

// Frame construit une image de width colonnes sur height lignes représentant la
// forme d'onde de samples (valeurs attendues dans [-1, 1]). La chaîne renvoyée
// commence par la séquence d'effacement puis les lignes : elle est prête à être
// imprimée telle quelle.
func Frame(samples []float64, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	// 1. Une valeur par colonne : moyenne des échantillons couverts. Sans
	//    échantillon, tout reste à 0 → une ligne de base bien nette au centre.
	cols := make([]float64, width)
	if n := len(samples); n > 0 {
		for x := 0; x < width; x++ {
			lo := x * n / width
			hi := (x + 1) * n / width
			if hi <= lo {
				hi = lo + 1
			}
			if hi > n {
				hi = n
			}
			var sum float64
			for i := lo; i < hi; i++ {
				sum += samples[i]
			}
			cols[x] = sum / float64(hi-lo)
		}
	}

	// 2. Lissage horizontal, proportionnel à la largeur (aucun sur les très
	//    petites largeurs, pour un rendu prévisible).
	cols = smooth(cols, width/100)

	// 3. Tracé : on relie chaque colonne à la précédente par un segment vertical
	//    plein, ce qui donne une ligne continue plutôt que des points isolés.
	canvas := make([][]rune, height)
	for y := range canvas {
		canvas[y] = make([]rune, width)
		for x := range canvas[y] {
			canvas[y][x] = ' '
		}
	}
	prev := -1
	for x := 0; x < width; x++ {
		r := row(cols[x], height)
		a, b := r, r
		if prev >= 0 {
			if a, b = prev, r; a > b {
				a, b = b, a
			}
		}
		for y := a; y <= b; y++ {
			canvas[y][x] = wave
		}
		prev = r
	}

	var sb strings.Builder
	sb.Grow(len(home) + height*(width+1))
	sb.WriteString(home)
	for _, line := range canvas {
		sb.WriteString(string(line))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// row convertit une amplitude [-1, 1] en numéro de ligne : +1 en haut (0), -1
// en bas (height-1), 0 au centre. Le gain d'affichage est appliqué puis borné.
func row(v float64, height int) int {
	v = clamp(v*displayGain, -1, 1)
	return int((1 - (v+1)/2) * float64(height-1))
}

// smooth applique une moyenne glissante de rayon radius (0 = pas de lissage).
func smooth(cols []float64, radius int) []float64 {
	if radius < 1 {
		return cols
	}
	out := make([]float64, len(cols))
	for i := range cols {
		lo, hi := i-radius, i+radius
		if lo < 0 {
			lo = 0
		}
		if hi >= len(cols) {
			hi = len(cols) - 1
		}
		var sum float64
		for j := lo; j <= hi; j++ {
			sum += cols[j]
		}
		out[i] = sum / float64(hi-lo+1)
	}
	return out
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
