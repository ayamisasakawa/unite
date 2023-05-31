package area

import (
	"fmt"
	"image"
	"os"
	"syscall"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gocv.io/x/gocv"

	"github.com/pidgy/unitehud/config"
	"github.com/pidgy/unitehud/gui/visual/button"
	"github.com/pidgy/unitehud/nrgba"
	"github.com/pidgy/unitehud/video"
	"github.com/pidgy/unitehud/video/device"
	"github.com/pidgy/unitehud/video/proc"
	"github.com/pidgy/unitehud/video/screen"
	"github.com/pidgy/unitehud/video/window"
)

const alpha = 150

var (
	Locked = nrgba.Black
	Match  = nrgba.Green
	Miss   = nrgba.Red
)

type Area struct {
	Text          string
	TextSize      unit.Sp
	TextAlignLeft bool
	Subtext       string
	Hidden        bool
	Theme         *material.Theme

	*Capture

	Match    func(*Area) bool
	Cooldown time.Duration
	readyq   chan bool

	*button.Button

	Min, Max         image.Point
	baseMin, baseMax image.Point

	nrgba.NRGBA

	Drag, Focus bool

	lastDimsSize image.Point
	lastRelease  time.Time
	lastScale    float64
}

type Capture struct {
	Option string
	File   string
	Base   image.Rectangle

	Matched *Area
}

func (a *Area) Layout(gtx layout.Context, dims layout.Dimensions, img image.Image) error {
	if img == nil || dims.Size.X == 0 || a.Base.Max.X == 0 {
		return nil
	}
	defer a.match()

	if a.Button == nil {
		a.Button = &button.Button{Active: false}
	}

	if a.Theme == nil {
		a.Theme = material.NewTheme(gofont.Collection())
	}

	// Scale
	a.TextSize = unit.Sp(24) * unit.Sp(float32(dims.Size.X)/float32(img.Bounds().Max.X))

	if a.Hidden {
		layout.UniformInset(unit.Dp(0)).Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				area := clip.Rect{
					Min: a.Min,
					Max: a.Max,
				}.Push(gtx.Ops)
				defer area.Pop()

				paint.ColorOp{Color: a.Alpha(alpha).Color()}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)

				return layout.Dimensions{Size: a.Max.Sub(a.Min)}
			},
		)

		layout.Inset{
			Left: unit.Dp(float32(a.Min.X)),
			Top:  unit.Dp(float32(a.Min.Y)),
		}.Layout(
			gtx,
			func(gtx layout.Context) layout.Dimensions {
				title := material.Body1(a.Theme, a.Text)
				title.TextSize = a.TextSize
				title.Font.Weight = 500
				title.Color = nrgba.White.Color()
				layout.Inset{
					Left: unit.Dp(2),
					Top:  unit.Dp(1),
				}.Layout(gtx, title.Layout)

				sub := material.Body2(a.Theme, a.Subtext)
				sub.TextSize = a.TextSize * unit.Sp(.75) // Scale.
				sub.Font.Weight = 1000
				sub.Color = nrgba.White.Alpha(175).Color()

				layout.Inset{
					Left: unit.Dp(2),
					Top:  unit.Dp(unit.Sp(a.Max.Sub(a.Min).Y) - a.TextSize),
				}.Layout(gtx, sub.Layout)

				return layout.Dimensions{Size: a.Max.Sub(a.Min)}
			},
		)

		return nil
	}

	if !a.lastDimsSize.Eq(dims.Size) {
		minXScale := float32(a.Base.Min.X) / float32(img.Bounds().Max.X)
		maxXScale := float32(a.Base.Max.X) / float32(img.Bounds().Max.X)
		minYScale := float32(a.Base.Min.Y) / float32(img.Bounds().Max.Y)
		maxYScale := float32(a.Base.Max.Y) / float32(img.Bounds().Max.Y)

		a.Min.X = int(float32(dims.Size.X) * minXScale)
		a.Max.X = int(float32(dims.Size.X) * maxXScale)
		a.Min.Y = int(float32(dims.Size.Y) * minYScale)
		a.Max.Y = int(float32(dims.Size.Y) * maxYScale)

		a.lastDimsSize = dims.Size

		if a.lastScale == 0 {
			a.baseMin, a.baseMax = a.Min, a.Max
		}
	}

	if config.Current.Scale != a.lastScale {
		a.lastScale = config.Current.Scale

		a.Min = image.Pt(int((float64(a.baseMin.X) * config.Current.Scale)), int((float64(a.baseMin.Y) * config.Current.Scale)))
		a.Max = image.Pt(int((float64(a.baseMax.X) * config.Current.Scale)), int((float64(a.baseMax.Y) * config.Current.Scale)))
	}

	for _, ev := range gtx.Events(a) {
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}

		switch e.Type {
		case pointer.Enter:
			a.Focus = true
			a.NRGBA = Locked
			a.NRGBA.A = 0
		case pointer.Leave:
			a.Focus = false
			a.NRGBA.A = alpha
		case pointer.Cancel:
		case pointer.Press:
			if !a.Active {
				break
			}
		case pointer.Release:
			if a.Drag {
				a.Drag = false

				baseMinXScale := float32(a.Min.X) * float32(img.Bounds().Max.X)
				baseMaxXScale := float32(a.Max.X) * float32(img.Bounds().Max.X)
				baseMinYScale := float32(a.Min.Y) * float32(img.Bounds().Max.Y)
				baseMaxYScale := float32(a.Max.Y) * float32(img.Bounds().Max.Y)

				a.Base.Min.X = int(baseMinXScale / float32(dims.Size.X))
				a.Base.Max.X = int(baseMaxXScale / float32(dims.Size.X))
				a.Base.Min.Y = int(baseMinYScale / float32(dims.Size.Y))
				a.Base.Max.Y = int(baseMaxYScale / float32(dims.Size.Y))
			} else {
				s := time.Since(a.lastRelease)
				if s > time.Millisecond*100 && s < time.Millisecond*500 {
					err := a.Capture.Open()
					if err != nil {
						return err
					}
				}
				a.lastRelease = time.Now()
			}
		case pointer.Move:
			if !a.Drag {
				break
			}
			fallthrough
		case pointer.Drag:
			a.Drag = true

			half := a.Max.Sub(a.Min).Div(2)
			a.Min = image.Pt(int(e.Position.X)-half.X, int(e.Position.Y)-half.Y)
			a.Max = image.Pt(int(e.Position.X)+half.X, int(e.Position.Y)+half.Y)
		}
	}

	layout.UniformInset(unit.Dp(0)).Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			area := clip.Rect{
				Min: a.Min,
				Max: a.Max,
			}.Push(gtx.Ops)
			defer area.Pop()

			paint.ColorOp{Color: a.Alpha(alpha).Color()}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)

			return layout.Dimensions{Size: a.Max.Sub(a.Min)}
		},
	)

	area := clip.Rect{
		Min: a.Min,
		Max: a.Max,
	}.Push(gtx.Ops)
	pointer.InputOp{
		Tag:   a,
		Types: pointer.Press | pointer.Drag | pointer.Release | pointer.Leave | pointer.Enter | pointer.Move,
		Grab:  a.Drag,
	}.Add(gtx.Ops)
	area.Pop()

	layout.Inset{
		Left: unit.Dp(float32(a.Min.X)),
		Top:  unit.Dp(float32(a.Min.Y)),
	}.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			return widget.Border{
				Color: a.Alpha(255).Color(),
				Width: unit.Dp(2),
			}.Layout(
				gtx,
				func(gtx layout.Context) layout.Dimensions {
					defer clip.Rect{Min: a.Min, Max: a.Max}.Push(gtx.Ops).Pop()
					return layout.Dimensions{Size: a.Max.Sub(a.Min)}
				})
		})

	layout.Inset{
		Left: unit.Dp(float32(a.Min.X)),
		Top:  unit.Dp(float32(a.Min.Y)),
	}.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			title := material.Body1(a.Theme, a.Text)
			title.TextSize = a.TextSize
			title.Font.Weight = 500
			title.Color = nrgba.White.Color()
			layout.Inset{
				Left: unit.Dp(2),
				Top:  unit.Dp(1),
			}.Layout(gtx, title.Layout)

			sub := material.Body2(a.Theme, a.Subtext)
			// Scale.
			sub.TextSize = a.TextSize * unit.Sp(.75)
			sub.Font.Weight = 1000
			sub.Color = nrgba.White.Alpha(175).Color()

			layout.Inset{
				Left: unit.Dp(2),
				Top:  unit.Dp(unit.Sp(a.Max.Sub(a.Min).Y) - a.TextSize),
			}.Layout(gtx, sub.Layout)

			return layout.Dimensions{Size: a.Max.Sub(a.Min)}
		},
	)

	return nil
}

func (c *Capture) Rectangle() image.Rectangle {
	return c.Base
}

func (a *Area) Reset() {
	a.lastDimsSize = image.Pt(0, 0)
}

func (a *Area) match() {
	if a.Drag || a.Focus {
		return
	}

	if a.readyq == nil {
		a.readyq = make(chan bool)
		go func() { a.readyq <- true }()
	}

	if !device.IsActive() && !screen.IsDisplay() && !window.IsWindow() {
		return
	}

	select {
	case <-a.readyq:
		go func() {
			if a.Match(a) {
				a.Capture.Matched = a
			} else {
				a.Capture.Matched = nil
			}
			time.Sleep(a.Cooldown)
			a.readyq <- true
		}()
	default:
	}
}

func (c *Capture) Open() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Failed to find current directory (%v)", err)
	}

	img, err := video.CaptureRect(c.Base)
	if err != nil {
		return fmt.Errorf("Failed to capture %s (%v)", c.File, err)
	}

	matrix, err := gocv.ImageToMatRGB(img)
	if err != nil {
		return fmt.Errorf("Failed to create %s (%v)", c.File, err)
	}
	defer matrix.Close()

	if !gocv.IMWrite(c.File, matrix) {
		return fmt.Errorf("Failed to save %s (%v)", c.File, err)
	}

	argv, err := syscall.UTF16PtrFromString(os.Getenv("windir") + "\\system32\\cmd.exe /C " + fmt.Sprintf("\"%s\\%s\"", dir, c.File))
	if err != nil {
		return fmt.Errorf("Failed to open %s (%v)", c.File, err)
	}

	var sI syscall.StartupInfo
	var pI syscall.ProcessInformation

	err = syscall.CreateProcess(nil, argv, nil, nil, true, proc.CreateNoWindow, nil, nil, &sI, &pI)
	if err != nil {
		return fmt.Errorf("Failed to open %s (%v)", c.File, err)
	}

	return nil
}
