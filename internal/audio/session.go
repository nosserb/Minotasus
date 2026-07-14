package audio

import (
	"context"
	"os"
	"os/exec"
)

// pwCmd construit une commande PipeWire (pw-record, pw-dump, wpctl) qui marche
// aussi quand kbdlight tourne sous sudo.
//
// PipeWire vit dans la session de l'utilisateur : son socket est à
// /run/user/<uid>/pipewire-0. Sous sudo, le programme est root et
// XDG_RUNTIME_DIR est réinitialisé, donc les outils PipeWire ne trouvent plus
// le serveur. On redescend alors vers l'utilisateur d'origine (SUDO_USER) en
// repointant XDG_RUNTIME_DIR vers sa session.
func pwCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	if os.Geteuid() == 0 {
		if uid := os.Getenv("SUDO_UID"); uid != "" {
			user := os.Getenv("SUDO_USER")
			wrapped := append([]string{
				"-u", user,
				"env", "XDG_RUNTIME_DIR=/run/user/" + uid,
				name,
			}, args...)
			return exec.CommandContext(ctx, "sudo", wrapped...)
		}
	}
	return exec.CommandContext(ctx, name, args...)
}
