// Package audio capte le son qui sort du PC (le « monitor » de la sortie par
// défaut) sous PipeWire, et le transforme en un niveau qui suit le rythme.
package audio

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
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

// pwNode est la vue minimale d'un objet PipeWire renvoyé par `pw-dump`.
type pwNode struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Info struct {
		State string                     `json:"state"`
		Props map[string]json.RawMessage `json:"props"`
	} `json:"info"`
}

func (n pwNode) prop(key string) string {
	raw, ok := n.Info.Props[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// AppStream cherche le flux de lecture d'une application (ex. "spotify") et
// renvoie l'identifiant de nœud à capter. La comparaison est insensible à la
// casse sur application.name et node.name. Un flux « running » est préféré.
func AppStream(match string) (target string, ok bool) {
	out, err := exec.Command("pw-dump").Output()
	if err != nil {
		return "", false
	}
	var nodes []pwNode
	if json.Unmarshal(out, &nodes) != nil {
		return "", false
	}
	want := strings.ToLower(match)
	best := -1
	running := false
	for _, n := range nodes {
		if n.prop("media.class") != "Stream/Output/Audio" {
			continue
		}
		app := strings.ToLower(n.prop("application.name"))
		name := strings.ToLower(n.prop("node.name"))
		if !strings.Contains(app, want) && !strings.Contains(name, want) {
			continue
		}
		// Garde le premier trouvé, mais un flux actif l'emporte.
		if best == -1 || (!running && n.Info.State == "running") {
			best = n.ID
			running = n.Info.State == "running"
		}
	}
	if best == -1 {
		return "", false
	}
	return strconv.Itoa(best), true
}
