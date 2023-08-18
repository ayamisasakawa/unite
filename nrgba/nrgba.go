package nrgba

import (
	"image/color"

	"github.com/pidgy/unitehud/rgba"
)

type NRGBA color.NRGBA

var (
	Announce       = NRGBA(rgba.Announce)
	Background     = NRGBA(rgba.Background)
	BackgroundAlt  = NRGBA(rgba.BackgroundAlt)
	Black          = NRGBA(rgba.Black)
	BloodOrange    = NRGBA(rgba.BloodOrange)
	CoolBlue       = NRGBA(rgba.CoolBlue)
	DarkRed        = NRGBA(rgba.DarkRed)
	DarkSeafoam    = NRGBA(rgba.DarkSeafoam)
	DarkYellow     = NRGBA(rgba.DarkYellow)
	DarkBlue       = NRGBA(rgba.DarkBlue)
	DarkGray       = NRGBA(rgba.DarkGray)
	DarkerYellow   = NRGBA(rgba.DarkerYellow)
	DarkerRed      = NRGBA(rgba.DarkerRed)
	Denounce       = NRGBA(rgba.Denounce)
	Disabled       = NRGBA(rgba.Disabled)
	DreamyBlue     = NRGBA(rgba.DreamyBlue)
	DreamyPurple   = NRGBA(rgba.DreamyPurple)
	ForestGreen    = NRGBA(rgba.ForestGreen)
	Gray           = NRGBA(rgba.Gray)
	Green          = NRGBA(rgba.Green)
	Highlight      = NRGBA(rgba.Highlight)
	LightGray      = NRGBA(rgba.LightGray)
	LightPurple    = NRGBA(rgba.LightPurple)
	Night          = NRGBA(rgba.Night)
	Orange         = NRGBA(rgba.Orange)
	Purple         = NRGBA(rgba.Purple)
	PurpleBlue     = NRGBA(rgba.PurpleBlue)
	PaleRed        = NRGBA(rgba.PaleRed)
	PastelBabyBlue = NRGBA(rgba.PastelBabyBlue)
	PastelBlue     = NRGBA(rgba.PastelBlue)
	PastelGreen    = NRGBA(rgba.PastelGreen)
	PastelRed      = NRGBA(rgba.PastelRed)
	Pinkity        = NRGBA(rgba.Pinkity)
	Red            = NRGBA(rgba.Red)
	Regice         = SeaBlue
	Regieleki      = Yellow
	Regirock       = NRGBA(rgba.Regirock)
	Registeel      = PaleRed
	SeaBlue        = NRGBA(rgba.SeaBlue)
	Seafoam        = NRGBA(rgba.Seafoam)
	Slate          = NRGBA(rgba.Slate)
	Splash         = NRGBA(rgba.Splash)
	System         = NRGBA(rgba.System)
	Transparent30  = NRGBA(rgba.Transparent30)
	Transparent    = NRGBA(rgba.Transparent)
	User           = NRGBA(rgba.User)
	White          = NRGBA(rgba.White)
	Yellow         = NRGBA(rgba.Yellow)
)

func (n NRGBA) Alpha(a uint8) NRGBA {
	n.A = a
	return n
}

func Bool(b bool) NRGBA {
	if b {
		return System
	}

	return System.Alpha(255 / 2)
}

func (n NRGBA) Color() color.NRGBA {
	return color.NRGBA(n)
}

func Objective(name string) NRGBA {
	return NRGBA(rgba.Objective(name))
}
