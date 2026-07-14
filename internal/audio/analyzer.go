package audio

import "math"

// Analyzer transforme des blocs d'échantillons audio en un niveau [0,1] qui
// « pulse » sur la musique. La chaîne de traitement :
//
//  1. filtre passe-bas → on garde surtout les basses (grosse caisse, basse),
//     là où vivent les battements ;
//  2. RMS du bloc → l'énergie instantanée ;
//  3. normalisation adaptative (AGC) → un pic récent sert de référence, donc
//     musique douce ou forte donnent la même dynamique ;
//  4. suiveur d'enveloppe attaque rapide / relâche lent → la lumière saute sur
//     le beat puis redescend doucement.
type Analyzer struct {
	lpAlpha   float64 // coefficient du passe-bas (0..1)
	attack    float64 // vitesse de montée (proche de 1 = très réactif)
	release   float64 // vitesse de descente (petit = traîne)
	peakDecay float64 // décroissance de la référence AGC par bloc
	gate      float64 // en dessous : considéré comme silence
	gain      float64 // sensibilité appliquée au niveau normalisé

	lp   float64 // état du filtre passe-bas
	peak float64 // référence adaptative
	env  float64 // enveloppe de sortie
}

// Params règle le comportement de l'analyseur. Zéro = valeur par défaut.
type Params struct {
	Rate int     // fréquence d'échantillonnage (Hz)
	Gain float64 // sensibilité (1 = neutre)
}

// NewAnalyzer construit un analyseur pour une fréquence donnée.
func NewAnalyzer(p Params) *Analyzer {
	if p.Rate <= 0 {
		p.Rate = 48000
	}
	if p.Gain <= 0 {
		p.Gain = 1
	}
	// Passe-bas ~200 Hz : alpha = dt / (RC + dt).
	const cutoff = 200.0
	dt := 1.0 / float64(p.Rate)
	rc := 1.0 / (2 * math.Pi * cutoff)
	return &Analyzer{
		lpAlpha:   dt / (rc + dt),
		attack:    0.55,
		release:   0.12,
		peakDecay: 0.9985,
		gate:      0.0008,
		gain:      p.Gain,
		peak:      0.02, // référence de départ non nulle
	}
}

// Process consomme un bloc d'échantillons (mono, [-1,1]) et renvoie le niveau
// [0,1] correspondant.
func (a *Analyzer) Process(samples []float64) float64 {
	if len(samples) == 0 {
		return a.env
	}

	var sumSq float64
	for _, x := range samples {
		a.lp += a.lpAlpha * (x - a.lp) // passe-bas un pôle
		sumSq += a.lp * a.lp
	}
	rms := math.Sqrt(sumSq / float64(len(samples)))

	// Silence : on laisse l'enveloppe retomber vers 0.
	if rms < a.gate {
		a.env += a.release * (0 - a.env)
		a.decayPeak()
		return clamp01(a.env)
	}

	// AGC : la référence suit les pics et redescend lentement.
	if rms > a.peak {
		a.peak = rms
	} else {
		a.decayPeak()
	}

	norm := clamp01(rms / a.peak * a.gain)

	// Attaque rapide en montée, relâche lente en descente.
	if norm > a.env {
		a.env += a.attack * (norm - a.env)
	} else {
		a.env += a.release * (norm - a.env)
	}
	return clamp01(a.env)
}

func (a *Analyzer) decayPeak() {
	a.peak *= a.peakDecay
	if a.peak < 0.02 {
		a.peak = 0.02
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
