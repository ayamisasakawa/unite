package button

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/pidgy/unitehud/cursor"
	"github.com/pidgy/unitehud/gui/visual/decorate"
	"github.com/pidgy/unitehud/gui/visual/screen"
	"github.com/pidgy/unitehud/nrgba"
	"github.com/pidgy/unitehud/splash"
)

type ImageWidget struct {
	*screen.Widget
	Click     func(i *ImageWidget)
	Hint      string
	HintEvent func()
	Hide      bool

	hover bool
}

func (i *ImageWidget) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	defer i.HoverHint()

	tmp := i.Widget.Image

	if i.Widget.Image == nil {
		i.Widget.Image = splash.Default()
	}

	if i.Hide {
		i.Widget.Image = image.NewRGBA(i.Widget.Bounds())

		hidden := material.H5(th, "Hidden")
		hidden.Alignment = text.Middle
		hidden.TextSize = unit.Sp(12)

		decorate.Label(&hidden, hidden.Text)

		layout.Inset{
			Top:  unit.Dp(18),
			Left: unit.Dp(29),
		}.Layout(gtx, hidden.Layout)
	}

	dims := i.Widget.Layout(gtx)
	i.Widget.Image = tmp

	for _, e := range gtx.Events(i) {
		if e, ok := e.(pointer.Event); ok {
			switch e.Type {
			case pointer.Enter:
				i.hover = true
				i.Widget.BorderColor = nrgba.White
				i.Widget.Border = true

				cursor.Is(pointer.CursorPointer)
			case pointer.Move:
				cursor.Is(pointer.CursorPointer)
			case pointer.Leave:
				i.hover = false
				i.Widget.BorderColor = nrgba.Gray

				cursor.Is(pointer.CursorDefault)
			case pointer.Press:
				cursor.Is(pointer.CursorPointer)
			case pointer.Release:
				cursor.Is(pointer.CursorDefault)

				if i.hover && i.Click != nil {
					i.Click(i)
					i.Widget.BorderColor = nrgba.Gray
				}
			}
		}
	}

	i.Widget.BorderColor = nrgba.Gray.Alpha(15)
	if i.hover {
		i.Widget.BorderColor = nrgba.White
	}

	area := clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops)
	pointer.InputOp{
		Tag:   i,
		Types: pointer.Press | pointer.Release | pointer.Enter | pointer.Leave | pointer.Move,
	}.Add(gtx.Ops)
	area.Pop()

	return dims
}

func (i *ImageWidget) HoverHint() {
	if i.hover && i.HintEvent != nil {
		i.HintEvent()
	}
}

func (i *ImageWidget) SetImage(img image.Image) {
	if i.Hide {
		return
	}

	i.Image = img
}
