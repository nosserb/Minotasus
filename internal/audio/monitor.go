package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Monitor capte du son via pw-record et émet, bloc après bloc, un niveau [0,1]
// qui suit le rythme.
//
// Deux modes :
//   - sink : capter le monitor d'une sortie (tout ce que joue le PC) ;
//   - app  : relier directement les ports de sortie d'une application (Spotify)
//     à notre capture, pour ne réagir qu'à elle.
type Monitor struct {
	sink     string   // node.name du sink (mode sink)
	appPorts []string // ports de sortie à relier (mode app)
	rate     int      // Hz
	block    int      // échantillons par bloc d'analyse
	gain     float64
}

// Options configure le Monitor. Renseigner AppPorts (mode app) OU Sink (mode
// sortie complète). Les zéros prennent des valeurs par défaut.
type Options struct {
	Sink     string   // node.name du sink à capter (sortie complète)
	AppPorts []string // ports "node:port" d'une app à relier (prioritaire sur Sink)
	Rate     int      // défaut 22050
	Block    int      // défaut 512 échantillons (~23 ms à 22050 Hz)
	Gain     float64  // sensibilité ; défaut 1
}

// NewMonitor prépare un Monitor à partir des options.
func NewMonitor(o Options) *Monitor {
	if o.Rate <= 0 {
		o.Rate = 48000 // par défaut la fréquence usuelle du graphe PipeWire
	}
	if o.Block <= 0 {
		o.Block = o.Rate / 50 // ~20 ms par bloc, quelle que soit la fréquence
	}
	if o.Gain <= 0 {
		o.Gain = 1
	}
	return &Monitor{sink: o.Sink, appPorts: o.AppPorts, rate: o.Rate, block: o.Block, gain: o.Gain}
}

// Start lance la capture et renvoie un canal de niveaux [0,1]. Le canal se ferme
// quand ctx est annulé ou que la capture s'arrête. La fonction renvoyée doit
// être appelée pour libérer le processus pw-record.
func (m *Monitor) Start(ctx context.Context) (<-chan float64, func(), error) {
	stdout, stop, err := m.spawn(ctx)
	if err != nil {
		return nil, nil, err
	}
	levels := make(chan float64, 8)
	go m.pump(stdout, levels)
	return levels, stop, nil
}

// Frames lance la capture et renvoie, bloc après bloc, la forme d'onde brute
// captée (mono, [-1,1]) — de quoi dessiner un oscilloscope. C'est le signal
// lui-même qui est émis, pas seulement son niveau. Comme Start, la capture se
// fait à la fréquence de sortie configurée (m.rate) et la fonction renvoyée
// libère pw-record.
func (m *Monitor) Frames(ctx context.Context) (<-chan []float64, func(), error) {
	stdout, stop, err := m.spawn(ctx)
	if err != nil {
		return nil, nil, err
	}
	frames := make(chan []float64, 4)
	go m.pumpFrames(stdout, frames)
	return frames, stop, nil
}

// spawn démarre pw-record (capture du monitor de sortie, ou des ports d'une
// application) et renvoie son flux PCM brut ainsi qu'une fonction d'arrêt. C'est
// la plomberie commune à Start (niveaux) et Frames (forme d'onde).
func (m *Monitor) spawn(ctx context.Context) (io.Reader, func(), error) {
	// Nom unique pour retrouver notre nœud de capture dans le graphe PipeWire.
	capName := fmt.Sprintf("kbdlight_capture_%d", os.Getpid())

	var args []string
	appMode := len(m.appPorts) > 0
	if appMode {
		// On coupe l'auto-connexion (sinon pw-record se relie au micro !) et on
		// reliera nous-mêmes les ports de l'application.
		args = []string{"-P", "node.autoconnect=false", "-P", "node.name=" + capName}
	} else {
		// Capter le monitor du sink = ce qui SORT, pas le micro.
		args = []string{"-P", "stream.capture.sink=true", "--target", m.sink}
	}
	args = append(args,
		"--rate", strconv.Itoa(m.rate),
		"--channels", "1",
		"--format", "s16",
		"--latency", "20ms",
		"--raw", "-",
	)

	cmd := pwCmd(ctx, "pw-record", args...)
	// Propre groupe de processus : sous « sudo -u … pw-record », tuer le seul
	// processus sudo laisserait pw-record orphelin. On tuera tout le groupe.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("démarrage de pw-record : %w", err)
	}

	if appMode {
		go linkAppPorts(ctx, capName, m.appPorts)
	}

	stop := func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // -pid = groupe
		}
		_ = cmd.Wait()
	}
	return stdout, stop, nil
}

// linkAppPorts attend que le port d'entrée de notre capture apparaisse, puis y
// relie les ports de sortie de l'application. Best-effort : en cas d'échec la
// capture reste muette (lumière éteinte) plutôt que de basculer sur le micro.
func linkAppPorts(ctx context.Context, capName string, appPorts []string) {
	inPort := waitInputPort(ctx, capName, 3*time.Second)
	if inPort == "" {
		return
	}
	for _, p := range appPorts {
		_ = pwCmd(ctx, "pw-link", p, inPort).Run()
	}
}

// waitInputPort scrute pw-link -i jusqu'à voir le port d'entrée de capName.
func waitInputPort(ctx context.Context, capName string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := pwCmd(ctx, "pw-link", "-i").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, capName+":") {
					return line
				}
			}
		}
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(100 * time.Millisecond):
		}
	}
	return ""
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

// pumpFrames lit le PCM s16 mono, le découpe en blocs et pousse chaque bloc de
// forme d'onde (mono, [-1,1]). Chaque bloc est une tranche neuve : le
// consommateur peut la garder le temps de la dessiner.
func (m *Monitor) pumpFrames(r io.Reader, out chan<- []float64) {
	defer close(out)

	raw := make([]byte, m.block*2) // 2 octets par échantillon s16
	for {
		if _, err := io.ReadFull(r, raw); err != nil {
			return
		}
		buf := make([]float64, m.block)
		for i := 0; i < m.block; i++ {
			s := int16(binary.LittleEndian.Uint16(raw[2*i:]))
			buf[i] = float64(s) / 32768.0
		}
		select {
		case out <- buf:
		default: // consommateur en retard : on saute ce bloc
		}
	}
}
