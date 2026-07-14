// Package audio capte le son qui sort du PC (le « monitor » de la sortie par
// défaut) sous PipeWire, et le transforme en un niveau qui suit le rythme.
package audio

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var nodeNameRe = regexp.MustCompile(`node\.name\s*=\s*"([^"]+)"`)

// DefaultSink renvoie le node.name de la sortie audio par défaut (haut-parleurs
// ou casque). On capte le monitor de ce node pour « entendre » ce que joue le
// PC.
func DefaultSink() (string, error) {
	out, err := exec.Command("wpctl", "inspect", "@DEFAULT_AUDIO_SINK@").Output()
	if err != nil {
		return "", fmt.Errorf("wpctl inspect (PipeWire présent ?) : %w", err)
	}
	m := nodeNameRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("node.name introuvable dans la sortie de wpctl")
	}
	return strings.TrimSpace(m[1]), nil
}
