package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

// Monitor capte le son en sortie du PC via pw-record et émet, bloc après bloc,
// un niveau [0,1] qui suit le rythme.
type Monitor struct {
	sink  string // node.name à capter ; vide = sortie par défaut
	rate  int    // Hz
	block int    // échantillons par bloc d'analyse
	gain  float64
}

// Options configure le Monitor. Les zéros prennent des valeurs par défaut.
type Options struct {
	Sink  string  // node.name ; vide = auto-détection
	Rate  int     // défaut 22050
	Block int     // défaut 512 échantillons (~23 ms à 22050 Hz)
	Gain  float64 // sensibilité ; défaut 1
}

// NewMonitor prépare un Monitor. Si Sink est vide, la sortie par défaut est
// détectée au démarrage.
func NewMonitor(o Options) *Monitor {
	if o.Rate <= 0 {
		o.Rate = 22050
	}
	if o.Block <= 0 {
		o.Block = 512
	}
	if o.Gain <= 0 {
		o.Gain = 1
	}
	return &Monitor{sink: o.Sink, rate: o.Rate, block: o.Block, gain: o.Gain}
}

// Start lance la capture et renvoie un canal de niveaux [0,1]. Le canal se ferme
// quand ctx est annulé ou que la capture s'arrête. La fonction renvoyée doit
// être appelée pour libérer le processus pw-record.
func (m *Monitor) Start(ctx context.Context) (<-chan float64, func(), error) {
	sink := m.sink
	if sink == "" {
		var err error
		if sink, err = DefaultSink(); err != nil {
			return nil, nil, err
		}
	}

	cmd := exec.CommandContext(ctx, "pw-record",
		// Capture ce que JOUE le sink (son monitor), pas le micro.
		"-P", "stream.capture.sink=true",
		"--target", sink,
		"--rate", strconv.Itoa(m.rate),
		"--channels", "1",
		"--format", "s16",
		"--latency", "20ms",
		"--raw", "-",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("démarrage de pw-record : %w", err)
	}

	levels := make(chan float64, 8)
	go m.pump(stdout, levels)

	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return levels, stop, nil
}

// pump lit le PCM s16 mono, le découpe en blocs et pousse les niveaux.
func (m *Monitor) pump(r io.Reader, out chan<- float64) {
	defer close(out)
	an := NewAnalyzer(Params{Rate: m.rate, Gain: m.gain})

	raw := make([]byte, m.block*2) // 2 octets par échantillon s16
	buf := make([]float64, m.block)
	for {
		if _, err := io.ReadFull(r, raw); err != nil {
			return
		}
		for i := 0; i < m.block; i++ {
			s := int16(binary.LittleEndian.Uint16(raw[2*i:]))
			buf[i] = float64(s) / 32768.0
		}
		level := an.Process(buf)
		select {
		case out <- level:
		default: // consommateur en retard : on saute ce niveau
		}
	}
}
