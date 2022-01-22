package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vova616/screenshot"
	"gocv.io/x/gocv"

	"github.com/pidgy/unitehud/config"
	"github.com/pidgy/unitehud/dev"
	"github.com/pidgy/unitehud/match"
	"github.com/pidgy/unitehud/pipe"
	"github.com/pidgy/unitehud/team"
	"github.com/pidgy/unitehud/template"
	"github.com/pidgy/unitehud/window/gui"
	"github.com/pidgy/unitehud/window/terminal"
)

// windows
// cls && go build && unitehud.exe
// go build -ldflags="-H windowsgui"
var (
	sigq = make(chan os.Signal, 1)
)

var (
	record = false
	addr   = ":17069"
	term   = false
)

func init() {
	flag.BoolVar(&record, "record", record, "record data such as images and logs for developer-specific debugging")
	flag.BoolVar(&term, "terminal", term, "use a custom terminal style window for debugging")
	flag.StringVar(&addr, "addr", addr, "http/websocket serve address")
	custom := flag.Bool("custom", false, "configure a customized screen capture or use the default 1920x1080 setting")
	avg := flag.Float64("match", 91, `0-100% certainty when processing score values`)
	level := flag.String("v", zerolog.LevelInfoValue, "log level (panic, fatal, error, warn, info, debug)")
	flag.Parse()

	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.Stamp,
		},
	).With().Timestamp().Logger()

	pipe.Socket = pipe.New(addr)

	go signals()

	lvl, err := zerolog.ParseLevel(*level)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Logger = log.Logger.Level(lvl)

	conf := "default"
	if *custom {
		conf = "custom"
	}

	err = config.Load(conf, float32(*avg)/100, 1, record)
	if err != nil {
		kill(err)
	}
}

func capture(name string, imgq chan *image.RGBA, stop *bool) {
	for {
		if !*stop {
			img, err := screenshot.CaptureRect(config.Current.Scores)
			if err != nil {
				kill(err)
			}

			imgq <- img
		}

		time.Sleep(team.Delay(name))
	}
}

func loop(t []template.Template, imgq chan *image.RGBA) {
	runtime.LockOSThread()

	for img := range imgq {
		matrix, err := gocv.ImageToMatRGB(img)
		if err != nil {
			kill(err)
		}

		m := match.Match{}

		m.Matches(matrix, img, t)

		matrix.Close()
	}
}

func kill(errs ...error) {
	for _, err := range errs {
		log.Error().Err(err).Send()
		time.Sleep(time.Millisecond)
	}

	sig := os.Kill
	if len(errs) == 0 {
		sig = os.Interrupt
	}

	sigq <- sig
}

func seconds(stop *bool) {
	m := match.Match{}

	for {
		if *stop {
			time.Sleep(time.Second)
			continue
		}

		img, err := screenshot.CaptureRect(config.Current.Time)
		if err != nil {
			kill(err)
		}

		matrix, err := gocv.ImageToMatRGB(img)
		if err != nil {
			kill(err)
		}

		rs, _ := m.Time(matrix, img)
		if rs == 0 {
			// Let's back off and not waste processing power.
			time.Sleep(time.Second * 5)
		}

		time.Sleep(team.Delay(team.Time.Name))
	}
}

func signals() {
	signal.Notify(sigq, syscall.SIGINT, syscall.SIGTERM)
	s := <-sigq

	terminal.Close()
	log.Info().Stringer("signal", s).Msg("closing...")
	os.Exit(1)
}

func main() {
	log.Info().
		Bool("record", record).
		Str("match", strconv.Itoa(int(config.Current.Acceptance*100))+"%").
		Str("addr", addr).Msg("unitehud")

	if record {
		err := dev.New()
		if err != nil {
			kill(err)
		}
	}

	imgq := map[string]chan *image.RGBA{
		team.None.Name:   make(chan *image.RGBA, 1),
		team.Self.Name:   make(chan *image.RGBA, 4),
		team.Purple.Name: make(chan *image.RGBA, 1),
		team.Orange.Name: make(chan *image.RGBA, 1),
		team.Balls.Name:  make(chan *image.RGBA, 1),
	}

	g := gui.New()
	defer g.Open()

	stop := true

	for category := range config.Current.Templates {
		if category == "points" || category == "time" {
			continue
		}

		for name := range config.Current.Templates[category] {
			for i := 0; i < cap(imgq[name]); i++ {
				go capture(name, imgq[name], &stop)
				go loop(config.Current.Templates[category][name], imgq[name])
			}
		}
	}

	go seconds(&stop)

	go func() {
		for {
			switch <-g.Actions {
			case gui.Start:
				log.Info().Bool("record", record).Str("match", strconv.Itoa(int(config.Current.Acceptance*100))+"%").Str("addr", addr).Msg("starting")
				g.Log("Started")
				stop = false
			case gui.Stop:
				log.Info().Bool("record", record).Str("match", strconv.Itoa(int(config.Current.Acceptance*100))+"%").Str("addr", addr).Msg("stopping")
				g.Log("Stopped")
				stop = true
			}
		}
	}()

	g.Log(fmt.Sprintf("Listening at %s", addr))

	if term {
		err := terminal.Init()
		if err != nil {
			kill(err)
		}

		terminal.Write(terminal.White, "Started Pokemon Unite HUD Server... listening on", addr)
		terminal.Show()
	}
}
