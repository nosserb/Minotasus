package audio

import (
	"math"
	"testing"
)

// tone génère un bloc de sinusoïde à freqHz et amplitude amp.
func tone(rate, n int, freqHz, amp float64) []float64 {
	b := make([]float64, n)
	for i := range b {
		b[i] = amp * math.Sin(2*math.Pi*freqHz*float64(i)/float64(rate))
	}
	return b
}

// feed applique un même bloc plusieurs fois et renvoie le dernier niveau.
func feed(a *Analyzer, block []float64, times int) float64 {
	var lvl float64
	for i := 0; i < times; i++ {
		lvl = a.Process(block)
	}
	return lvl
}

func TestSilenceStaysLow(t *testing.T) {
	a := NewAnalyzer(Params{Rate: 22050})
	lvl := feed(a, make([]float64, 512), 50)
	if lvl > 0.05 {
		t.Fatalf("silence devrait rester bas, got %.3f", lvl)
	}
}

func TestLoudBassRaisesLevel(t *testing.T) {
	a := NewAnalyzer(Params{Rate: 22050})
	// 60 Hz (grosse caisse) fort : doit faire monter la lumière.
	lvl := feed(a, tone(22050, 512, 60, 0.8), 30)
	if lvl < 0.4 {
		t.Fatalf("basse forte devrait allumer, got %.3f", lvl)
	}
}

func TestDropToSilenceDecays(t *testing.T) {
	a := NewAnalyzer(Params{Rate: 22050})
	feed(a, tone(22050, 512, 60, 0.8), 30)   // musique
	lvl := feed(a, make([]float64, 512), 40) // puis silence
	if lvl > 0.1 {
		t.Fatalf("après coupure la lumière devrait retomber, got %.3f", lvl)
	}
}

// L'AGC doit donner une dynamique comparable, que la musique soit douce ou
// forte : deux volumes différents convergent vers un niveau proche.
func TestAGCIndependentOfVolume(t *testing.T) {
	loud := feed(NewAnalyzer(Params{Rate: 22050}), tone(22050, 512, 60, 0.9), 40)
	soft := feed(NewAnalyzer(Params{Rate: 22050}), tone(22050, 512, 60, 0.15), 40)
	if math.Abs(loud-soft) > 0.2 {
		t.Fatalf("AGC : fort=%.3f et doux=%.3f trop éloignés", loud, soft)
	}
}

func TestHighFrequencyAttenuated(t *testing.T) {
	// Le passe-bas doit atténuer une aiguë : moins d'effet qu'une basse égale.
	bass := feed(NewAnalyzer(Params{Rate: 22050}), tone(22050, 512, 60, 0.5), 30)
	treble := feed(NewAnalyzer(Params{Rate: 22050}), tone(22050, 512, 8000, 0.5), 30)
	if treble >= bass {
		t.Fatalf("aiguë (%.3f) devrait être atténuée sous basse (%.3f)", treble, bass)
	}
}
