// Package audio capte le son qui sort du PC (le « monitor » de la sortie par
// défaut) sous PipeWire, et le transforme en un niveau qui suit le rythme.
package audio

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var nodeNameRe = regexp.MustCompile(`node\.name\s*=\s*"([^"]+)"`)
var clockRateRe = regexp.MustCompile(`clock\.rate'\s+value:'(\d+)'`)

// DefaultRate renvoie la fréquence d'échantillonnage du graphe PipeWire. Capter
// à cette fréquence évite tout rééchantillonnage — important quand on se branche
// sur le flux d'une application, sinon le conflit d'horloge peut dégrader (voire
// « saturer ») le son joué. En cas d'échec, 48000 Hz (valeur quasi universelle).
func DefaultRate() int {
	out, err := pwCmd(context.Background(), "pw-metadata", "-n", "settings").Output()
	if err == nil {
		if m := clockRateRe.FindSubmatch(out); m != nil {
			if r, err := strconv.Atoi(string(m[1])); err == nil && r > 0 {
				return r
			}
		}
	}
	return 48000
}

// DefaultSink renvoie le node.name de la sortie audio par défaut (haut-parleurs
// ou casque). On capte le monitor de ce node pour « entendre » ce que joue le
// PC.
func DefaultSink() (string, error) {
	out, err := pwCmd(context.Background(), "wpctl", "inspect", "@DEFAULT_AUDIO_SINK@").Output()
	if err != nil {
		return "", fmt.Errorf("wpctl inspect (PipeWire présent ?) : %w", err)
	}
	m := nodeNameRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("node.name introuvable dans la sortie de wpctl")
	}
	return strings.TrimSpace(m[1]), nil
}

// AppPorts renvoie les ports de sortie audio d'une application (ex. "spotify"),
// sous la forme "node:port" (p. ex. "spotify:output_FL"). Ces ports serviront à
// relier directement le flux de l'app à notre capture. La comparaison sur le
// nom de nœud est insensible à la casse.
func AppPorts(match string) (ports []string, ok bool) {
	out, err := pwCmd(context.Background(), "pw-link", "-o").Output()
	if err != nil {
		return nil, false
	}
	want := strings.ToLower(match)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Format "node:port" ; on ne garde que les ports audio (output_*).
		i := strings.LastIndex(line, ":")
		if i < 0 {
			continue
		}
		node, port := line[:i], line[i+1:]
		if !strings.HasPrefix(port, "output_") {
			continue
		}
		if strings.Contains(strings.ToLower(node), want) {
			ports = append(ports, line)
		}
	}
	return ports, len(ports) > 0
}
