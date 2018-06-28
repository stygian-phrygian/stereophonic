package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
)

// engine creates an audio context
type Engine struct {
	// stream parameters keeps track of relevant
	// playback variables for a stream, namely SampleRate, FramesPerBuffer,
	// and the output device.
	streamParameters portaudio.StreamParameters
	// the returned "stream" object by portaudio which we can start/stop
	stream *portaudio.Stream
	// mapping from a slot number -> sample (or as we call tables)
	tables map[int]*Table
	// list of (currently) active table players
	// appending to this is done by Engine.Play(...)
	// and removal is done by StopActivePlayback() or when
	// a TablePlayer's reached a "done" state of playback
	activeTablePlayers []*TablePlayer
}

// create the audio output engine
// this internally initializes an (unbegun) instance of portaudio
// with the default output device selected
// if you want to change the sample rate, bufferSizeInFrames,
// or the output device, you must Stop() the engine before
// calling their respective setter methods
func New(sr float64, bufferSizeInFrames int) (*Engine, err) {
	// TODO

	return &Engine{
		stream:       stream,
		tables:       []*Table{},
		MonitoringOn: false,
	}, nil
}

// list audio devices
func (e *Engine) ListDevices() ([]*portaudio.DeviceInfo, err) {
	return portaudio.Devices()
}

// setters for sample rate, bufferSizeInFrames, and output device
// calling these will stop the engine (if it's playing)
// and create an new audio stream with the aforementioned set variables
// Nota Bene, you might be able to set values which will incur
// an error once Start() is called afterwards (resuming playback)
// for example, the sample rate / buffer size in frames
// might be impossible for the underlying output device
// these setters *wont* show you whether they're acceptable values
func (e *Engine) SetSampleRate(sr float64) err {
	//TODO
}
func (e *Engine) SetBufferSizeInFrames(bufferSizeInFrames int) err {
	//TODO
}
func (e *Engine) SetDevice() {
	//TODO
}

// start streaming audio to the output device
func (e *Engine) Start() error {
	//TODO
}

// stop streaming audio to the output device
func (e *Engine) Stop() error {
	//TODO
}

// turn off the audio backend (and terminate portaudio)
// the Engine cannot be used any further
func (e *Engine) Close() error {
	//TODO
}

// loads a soundfile into a sample slot
// (which internally just loads a table with the soundfile frames,
// then saves a reference in the engine)
func (e *Engine) Load(slot int, sampleFile string) err {
}

// trigger playback of a table
// internally, creates a table player, adds
// the tableplayer to the activeTablePlayers of engine
// and begins ticking (computing) audio frames
// the returned *TablePlayer can be used to control playback
// of the sound in realtime should it be desired
func (e *Engine) Play(slot int) *TablePlayer {
}
