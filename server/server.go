package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"nhooyr.io/websocket"

	"github.com/pidgy/unitehud/config"
	"github.com/pidgy/unitehud/global"
	"github.com/pidgy/unitehud/notify"
	"github.com/pidgy/unitehud/rgba"
	"github.com/pidgy/unitehud/state"
	"github.com/pidgy/unitehud/team"
)

const Address = "127.0.0.1:17069"

type game struct {
	Bottom    []objective `json:"bottom"`
	Config    bool        `json:"config"`
	Defeated  []int       `json:"defeated"`
	Energy    int         `json:"balls"`
	Events    []string    `json:"events"`
	Match     bool        `json:"match"`
	Orange    score       `json:"orange"`
	Purple    score       `json:"purple"`
	Profile   string      `json:"profile"`
	Rayquaza  string      `json:"rayquaza"`
	Regilekis []string    `json:"regis"`
	Seconds   int         `json:"seconds"`
	Self      score       `json:"self"`
	Stacks    int         `json:"stacks"`
	Started   bool        `json:"started"`
	Version   string      `json:"version"`
}

type info struct {
	game

	tx       int
	requests int

	clients map[string]time.Time

	mutex *sync.Mutex
}

type objective struct {
	Name string `json:"name"`
	Team string `json:"team"`
	Time int64  `json:"time"`
}

type score struct {
	Team  string `json:"team"`
	Value int    `json:"value"`
	KOs   int    `json:"kos"`
}

var current = &info{
	game:    reset(),
	clients: map[string]time.Time{},
	mutex:   &sync.Mutex{},
}

func Bottom() []objective {
	return current.game.Bottom
}

func Clear() {
	log.Debug().Object("game", current.game).Msg("clearing")

	started := current.game.Started
	current.game = reset()
	current.game.Started = started
}

func Clock() string {
	return fmt.Sprintf("%02d:%02d", current.game.Seconds/60, current.game.Seconds%60)
}

func Clients() int {
	current.mutex.Lock()
	defer current.mutex.Unlock()

	for c := range current.clients {
		if time.Since(current.clients[c]) > time.Second*5 {
			notify.Feed(rgba.Slate, "Client %s has disconnected", c)
			delete(current.clients, c)
		}
	}

	return len(current.clients)
}

func Holding() int {
	return current.game.Energy
}

func IsFinalStretch() bool {
	return current.game.Seconds != 0 && current.game.Seconds <= 120
}

func KOs(t *team.Team) int {
	switch t.Name {
	case team.Purple.Name:
		return current.game.Purple.KOs
	case team.Orange.Name:
		return current.game.Orange.KOs
	default:
		return 0
	}
}

func Listen() error {
	http.Handle("/ws", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns:     []string{"127.0.0.1", "localhost", "0.0.0.0"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			notify.Error("Server failed to accept websocket connection (%v)", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "cross origin WebSocket accepted")

		current.game.Profile = config.Current.Profile
		current.game.Events = state.Strings(time.Second * 5)

		raw, err := json.Marshal(current.game)
		if err != nil {
			notify.Error("Server failed to create server response (%v)", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = c.Write(context.Background(), websocket.MessageText, raw)
		if err != nil {
			notify.Error("Server failed to send server response (%v)", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		current.client(r, "/ws", raw)

		current.tx += len(raw)
		current.requests++
	}))

	http.HandleFunc("/http", func(w http.ResponseWriter, r *http.Request) {
		current.game.Profile = config.Current.Profile
		current.game.Events = state.Strings(time.Second * 5)

		raw, err := json.Marshal(current.game)
		if err != nil {
			notify.Error("Server failed to create server response (%v)", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Debug().RawJSON("response", raw).Str("client", r.RemoteAddr).Msg("http response")

		_, err = w.Write(raw)
		if err != nil {
			notify.Error("Server failed to send server response (%v)", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		current.client(r, "/http", raw)
		current.tx += len(raw)
		current.requests++
	})

	go func() {
		last := 0

		for range time.NewTicker(time.Minute).C {
			if current.requests < 1 {
				continue
			}

			diff := float64(last - (current.tx / current.requests))
			if math.Abs(diff) < 10 {
				continue
			}
			last = current.tx / current.requests

			notify.System("Server is sending an average of %d bytes per request", last)
		}
	}()

	state.Add(state.ServerStarted, Clock(), -1)

	errq := make(chan error)
	go func() {
		errq <- http.ListenAndServe(Address, nil)
	}()

	time.AfterFunc(time.Second, func() { errq <- nil })

	return <-errq
}

func Match() bool {
	return current.game.Match
}

func Objectives(t *team.Team) (regielekis, regices, regirocks, registeels int) {
	return RegielekisSecured(t), RegicesSecured(t), RegirocksSecured(t), RegisteelsSecured(t)
}

func RegielekiAdv() *team.Team {
	p := 0
	o := 0

	for _, t := range current.game.Regilekis {
		switch t {
		case team.Purple.Name:
			p++
		case team.Orange.Name:
			o++
		}
	}

	switch {
	case p > o:
		return team.Purple
	case o > p:
		return team.Orange
	default:
		return team.None
	}
}

func RegicesSecured(t *team.Team) int {
	n := 0
	for _, b := range current.game.Bottom {
		if b.Name == "regice" && b.Team == t.Name {
			n++
		}
	}
	return n
}

func Regielekis() []string {
	return current.game.Regilekis
}

func RegielekisSecured(t *team.Team) int {
	n := 0
	for _, r := range current.game.Regilekis {
		if r == t.Name {
			n++
		}
	}
	return n
}

func RegirocksSecured(t *team.Team) int {
	n := 0
	for _, b := range current.game.Bottom {
		if b.Name == "regirock" && b.Team == t.Name {
			n++
		}
	}
	return n
}

func RegisteelsSecured(t *team.Team) int {
	n := 0
	for _, b := range current.game.Bottom {
		if b.Name == "registeel" && b.Team == t.Name {
			n++
		}
	}
	return n
}

func Score(t *team.Team) int {
	switch t {
	case team.Purple:
		return current.game.Purple.Value
	case team.Orange:
		return current.game.Orange.Value
	case team.Self:
		return current.game.Self.Value
	default:
		return -1
	}
}

func Scores() (orange, purple, self int) {
	return current.game.Orange.Value, current.game.Purple.Value, current.game.Self.Value
}

func Seconds() int {
	return current.game.Seconds
}

func SetBottomObjective(t *team.Team, name string, n int) {
	o := objective{
		Team: t.Name,
		Name: name,
		Time: time.Now().Unix(),
	}

	op := fmt.Sprintf("[%s] %s #%d", strings.Title(t.Name), strings.Title(o.Name), n+1)

	switch {
	// Illegal.
	case len(current.Bottom) < n:
		notify.Warn("[Control] %s illegal operation (no index)", op)

	// Remove.
	case len(current.Bottom) == n+1 && current.Bottom[n].Team == t.Name && current.Bottom[n].Name == o.Name:
		// Remove last objective.
		current.Bottom = current.Bottom[:n]
		notify.Unique(t.RGBA, "[Control] %s removed", op)

	// Add.
	case len(current.Bottom) == n:
		current.Bottom = append(current.Bottom, o)
		notify.Unique(t.RGBA, "[Control] %s secured", op)
	case len(current.Bottom) > n+1 && current.Bottom[n].Team != t.Name:
		current.Bottom[n] = o
		notify.Unique(t.RGBA, "[Control] %s secure replaced", op)

		// Overwrite.
	case len(current.Bottom) == n+1 && current.Bottom[n].Team == t.Name && current.Bottom[n].Name != o.Name:
		// Replace between first and last.
		fallthrough
	case len(current.Bottom) > n+1 && current.Bottom[n].Team == t.Name:
		fallthrough
	case len(current.Bottom) == n+1 && current.Bottom[n].Team != t.Name:
		// Overwrite last objective.
		current.Bottom[n] = o
		notify.Unique(t.RGBA, "[Control] %s secure replaced", op)
	}
}

func SetConfig(c bool) {
	current.game.Config = c
}

func SetDefeated() {
	current.game.Defeated = append(current.game.Defeated, current.game.Seconds)
}

func SetEnergy(b int) {
	current.game.Energy = b
}

func SetKO(t *team.Team) {
	switch t.Name {
	case team.Purple.Name:
		current.game.Purple.KOs++
	case team.Orange.Name:
		current.game.Orange.KOs++
	}
}

func SetMatchStarted() {
	current.game.Match = true
}

func SetMatchStopped() {
	current.game.Match = false
}

func SetRayquaza(t *team.Team) {
	current.game.Rayquaza = t.Name
}

func SetRegice(t *team.Team) {
	current.Bottom = append(current.Bottom, objective{
		Team: t.Name,
		Name: "regice",
		Time: time.Now().Unix(),
	})
}

func SetRegieleki(t *team.Team) {
	for i, t2 := range current.Regilekis {
		if t2 == team.None.Name {
			current.game.Regilekis[i] = t.Name
			return
		}
	}

	current.game.Regilekis[0] = t.Name
	current.game.Regilekis[1] = team.None.Name
	current.game.Regilekis[2] = team.None.Name
}

// SetRegielekiAt assumes n to be an index starting at 0.
func SetRegielekiAt(t *team.Team, n int) {
	op := fmt.Sprintf("[%s] Regieleki #%d", strings.Title(t.Name), n+1)

	switch {
	case n != 0 && current.game.Regilekis[n-1] == team.None.Name:
		notify.Warn("[Control] %s illegal operation (missing previous)", op)
	case current.game.Regilekis[n] != t.Name:
		notify.Unique(t.RGBA, "[Control] %s secure replaced", op)
		current.game.Regilekis[n] = t.Name
	case n+1 == len(current.game.Regilekis) || current.game.Regilekis[n+1] == team.None.Name:
		notify.Unique(t.RGBA, "[Control] %s reset", op)
		current.game.Regilekis[n] = team.None.Name
	default:
		notify.Warn("[Control] %s illegal operation", op)
	}
}

func SetRegirock(t *team.Team) {
	current.Bottom = append(current.Bottom, objective{
		Team: t.Name,
		Name: "regirock",
		Time: time.Now().Unix(),
	})
}

func SetRegisteel(t *team.Team) {
	current.Bottom = append(current.Bottom, objective{
		Team: t.Name,
		Name: "registeel",
		Time: time.Now().Unix(),
	})
}

func SetScore(t *team.Team, value int) {
	s := score{
		Team:  t.Name,
		Value: value,
	}

	log.Debug().Object("score", s).Object("game", current.game).Msg("publishing")

	switch t.Name {
	case team.Purple.Name:
		current.game.Purple.Value += s.Value
	case team.Orange.Name:
		current.game.Orange.Value += s.Value
	case team.Self.Name:
		current.game.Purple.Value += s.Value
		current.game.Self.Value += s.Value
		current.game.Stacks++
	case team.First.Name:
		switch team.First.Alias {
		case team.Purple.Name:
			current.game.Purple.Value += s.Value
		case team.Orange.Name:
			current.game.Orange.Value += s.Value
		default:
			notify.Error("Server received first goal from an unknown team")
		}
	}
}

func SetStarted() {
	current.game.Started = true
}

func SetStopped() {
	current.game.Started = false
}

func SetTime(minutes, seconds int) {
	if minutes+seconds == 0 {
		current.game.Match = false
		return
	}

	current.game.Match = true

	current.game.Seconds = minutes*60 + seconds
}

func Started() bool {
	return current.game.Started
}

func (i *info) client(r *http.Request, route string, raw []byte) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	ip := strings.Split(r.RemoteAddr, ":")[0]
	key := fmt.Sprintf("%s -> %s", ip, r.URL)

	_, ok := i.clients[key]
	if !ok {
		notify.System("Server accepted a new %s connection from %s", route, key)
		log.Debug().RawJSON("response", raw).Str("client", key).Msg("first json response")
	}

	i.clients[key] = time.Now()
}

func reset() game {
	return game{
		Purple: score{
			Team:  team.Purple.Name,
			Value: 0,
		},
		Orange: score{
			Team:  team.Orange.Name,
			Value: 0,
		},
		Self: score{
			Team:  team.Self.Name,
			Value: 0,
		},
		Seconds:   0,
		Energy:    0,
		Regilekis: []string{team.None.Name, team.None.Name, team.None.Name},
		Bottom:    []objective{},
		Version:   global.Version,
		Defeated:  []int{},
	}
}

// Zerolog.

func (g game) MarshalZerologObject(e *zerolog.Event) {
	e.Object("purple", g.Purple).Object("orange", g.Orange).Int("seconds", g.Seconds).Bool("config", g.Config)
}

func (s score) MarshalZerologObject(e *zerolog.Event) {
	e.Str("team", s.Team).Int("value", s.Value)
}
