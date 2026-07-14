// Package match suit un match de foot en direct via l'API publique gratuite
// TheSportsDB, et en déduit les moments forts (buts, tension) pour piloter le
// rétroéclairage.
package match

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// base est l'API v1 de TheSportsDB ; la clé "3" est la clé de test publique.
const base = "https://www.thesportsdb.com/api/v1/json/3"

// Phase décrit l'état d'avancement d'un match.
type Phase int

const (
	Scheduled Phase = iota // pas encore commencé
	Live                   // en jeu
	HalfTime               // mi-temps
	Finished               // terminé
	Unknown
)

// State est l'instantané d'un match.
type State struct {
	Home, Away         int
	HomeTeam, AwayTeam string
	Minute             int // minute de jeu (0 si inconnue)
	Phase              Phase
	Status             string // statut brut renvoyé par l'API
}

// Client interroge TheSportsDB.
type Client struct {
	http *http.Client
}

// NewClient construit un client avec un timeout raisonnable.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TheSportsDB a répondu %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// TeamID cherche l'identifiant d'une équipe de foot par son nom.
func (c *Client) TeamID(ctx context.Context, name string) (string, error) {
	var r struct {
		Teams []struct {
			ID    string `json:"idTeam"`
			Name  string `json:"strTeam"`
			Sport string `json:"strSport"`
		} `json:"teams"`
	}
	if err := c.get(ctx, "/searchteams.php?t="+url.QueryEscape(name), &r); err != nil {
		return "", err
	}
	for _, t := range r.Teams {
		if t.Sport == "Soccer" {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("équipe de foot introuvable : %q", name)
}

// NextEventID renvoie le prochain (ou actuel) match d'une équipe.
func (c *Client) NextEventID(ctx context.Context, teamID string) (string, error) {
	var r struct {
		Events []struct {
			ID    string `json:"idEvent"`
			Event string `json:"strEvent"`
		} `json:"events"`
	}
	if err := c.get(ctx, "/eventsnext.php?id="+url.QueryEscape(teamID), &r); err != nil {
		return "", err
	}
	if len(r.Events) == 0 {
		return "", fmt.Errorf("aucun match à venir pour l'équipe %s", teamID)
	}
	return r.Events[0].ID, nil
}

// Lookup renvoie l'état courant d'un match par son idEvent.
func (c *Client) Lookup(ctx context.Context, eventID string) (State, error) {
	var r struct {
		Events []struct {
			Event    string  `json:"strEvent"`
			Status   string  `json:"strStatus"`
			Progress string  `json:"strProgress"`
			Home     *string `json:"intHomeScore"`
			Away     *string `json:"intAwayScore"`
			HomeTeam string  `json:"strHomeTeam"`
			AwayTeam string  `json:"strAwayTeam"`
		} `json:"events"`
	}
	if err := c.get(ctx, "/lookupevent.php?id="+url.QueryEscape(eventID), &r); err != nil {
		return State{}, err
	}
	if len(r.Events) == 0 {
		return State{}, fmt.Errorf("match %s introuvable", eventID)
	}
	e := r.Events[0]
	st := State{
		Home:     atoiSafe(e.Home),
		Away:     atoiSafe(e.Away),
		HomeTeam: e.HomeTeam,
		AwayTeam: e.AwayTeam,
		Status:   e.Status,
	}
	st.Phase, st.Minute = parseStatus(e.Status, e.Progress)
	return st, nil
}

// atoiSafe convertit une chaîne de score éventuellement nulle en entier.
func atoiSafe(s *string) int {
	if s == nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(*s))
	return n
}

// parseStatus interprète strStatus/strProgress en phase + minute. Le statut
// peut être un code ("NS", "HT", "FT", "AET", "PEN") ou une minute ("67").
func parseStatus(status, progress string) (Phase, int) {
	s := strings.ToUpper(strings.TrimSpace(status))
	minute := parseMinute(progress)
	if minute == 0 {
		minute = parseMinute(status)
	}
	switch s {
	case "", "NS", "NOT STARTED":
		return Scheduled, 0
	case "HT", "HALF TIME":
		return HalfTime, 45
	case "FT", "AET", "MATCH FINISHED", "FINISHED":
		return Finished, minute
	}
	// Codes de jeu (1H, 2H, ET, PEN, LIVE) ou minute numérique → en jeu.
	return Live, minute
}

// parseMinute extrait un nombre de minutes d'une chaîne comme "67" ou "67'".
func parseMinute(s string) int {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "'"))
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 0
}
