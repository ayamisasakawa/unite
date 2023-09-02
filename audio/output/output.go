package output

import (
	"fmt"
	"io"

	"github.com/gen2brain/malgo"
	"github.com/pkg/errors"

	"github.com/pidgy/unitehud/audio/device"
	"github.com/pidgy/unitehud/notify"
)

type Device struct {
	ID      string
	Formats []malgo.DataFormat

	name      string
	isDefault bool

	config malgo.DeviceConfig

	active            bool
	closingq, closedq chan bool

	reconnects int
}

func (d *Device) Is(name string) bool { return device.Is(d, name) }
func (d *Device) IsDefault() bool     { return d.isDefault }
func (d *Device) IsDisabled() bool    { return d.name == device.Disabled }
func (d *Device) Name() string        { return d.name }

func New(ctx *malgo.AllocatedContext, name string) (*Device, error) {
	if name == device.Disabled {
		return &Device{name: device.Disabled}, nil
	}

	for _, d := range Devices(ctx) {
		if !device.Is(d, name) {
			continue
		}

		d.config = malgo.DefaultDeviceConfig(malgo.Playback)
		d.config.Capture.Format = malgo.FormatS16
		d.config.Capture.Channels = 1
		d.config.Playback.Format = malgo.FormatS16
		d.config.Playback.Channels = 1
		d.config.SampleRate = 44100
		d.config.Alsa.NoMMap = 1

		return d, nil
	}

	return nil, fmt.Errorf("failed to find playback device with term: %s", name)
}

func (d *Device) Active() bool {
	return d.active
}

func (d *Device) Close() {
	defer notify.Debug("Closed %s", d.name)

	if !d.Active() {
		return
	}

	close(d.closingq)
	<-d.closedq
}

// Playback streams samples from a reader to the sound device.
// The function initializes a playback device in the default context using
// provide stream configuration.
// Playback will commence playing the samples provided from the reader until either the
// reader returns an error, or the context signals done.
func (d *Device) Start(mctx malgo.Context, r io.ReadWriter, errq chan error) {
	if d.Active() {
		errq <- errors.Wrap(fmt.Errorf("already active"), d.name)
		return
	}

	go func() {
		d.closingq = make(chan bool)
		d.closedq = make(chan bool)
		d.active = true

		defer func() { d.active = false }()
		defer close(d.closedq)

		callbacks := malgo.DeviceCallbacks{
			Data: func(outputSamples, inputSamples []byte, frameCount uint32) {
				if !d.Active() {
					return
				}

				if frameCount == 0 {
					return
				}

				_, err := io.ReadFull(r, outputSamples)
				if err != nil {
					if err == io.EOF || err == io.ErrUnexpectedEOF {
						d.reconnects++
						return
					}

					errq <- errors.Wrap(err, d.name)
				}
			},
		}

		device, err := malgo.InitDevice(mctx, d.config, callbacks)
		if err != nil {
			errq <- errors.Wrap(err, d.name)
			return
		}
		defer device.Uninit()

		err = device.Start()
		if err != nil {
			errq <- errors.Wrap(err, d.name)
			return
		}
		defer func() {
			err = device.Stop()
			if err != nil {
				errq <- errors.Wrap(err, d.name)
				return
			}
		}()

		<-d.closingq
	}()
}

func (d *Device) String() string {
	return device.String(d)
}

func (d *Device) Type() device.Type {
	return device.Output
}

func Devices(ctx *malgo.AllocatedContext) (playbacks []*Device) {
	d, err := ctx.Devices(malgo.Playback)
	if err != nil {
		notify.Error("Failed to discover audio playback devices (%v)", err)
		return nil
	}

	for _, info := range d {
		full, err := ctx.DeviceInfo(malgo.Playback, info.ID, malgo.Shared)
		if err != nil {
			notify.Warn("Failed to poll audio playback device \"%s\" info (%v)", info.ID, err)
		}

		playbacks = append(playbacks, &Device{
			ID:      info.ID.String(),
			Formats: full.Formats,

			name:      info.Name(),
			isDefault: info.IsDefault != 0,
		})
	}

	return playbacks
}
