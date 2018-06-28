package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

const (
	ConfigureWhileStreamStarted  error = fmt.Errorf("cannot configure engine while started, must call Stop() first")
	ConfigureWhileNotInitialized error = fmt.Errorf("cannot configure engine while not initialized, must call Reinitialize() first")
)

// engine is a type which maintains structural information
// related to playback and device parameters
type Engine struct {
	// stream parameters keeps track of relevant
	// playback variables for a stream, namely SampleRate, FramesPerBuffer,
	// and the output device.
	streamParameters portaudio.StreamParameters
	// the returned "stream" object by portaudio which we can start/stop
	stream *portaudio.Stream
	// mapping from a slot number -> sample (or as we call tables)
	// this collates references to the loaded tables
	tables map[int]*Table
	// list of (currently) active table players
	// appending to this is done by Engine.Play(...)
	// and removal is done by StopActivePlayback() or when
	// a TablePlayer's reached a "done" state of playback
	activeTablePlayers []*TablePlayer
	// flag to check whether portaudio is initialized
	initialized bool
	// flag to check whether the portaudio stream started
	started bool
	// lock (for use when configuring the engine)
	sync.Mutex
}

// initialize portaudio, acquiring the default output device
// with a default low latency configuration
//
// this does *not* start an audio stream , it just configures one
//
// if you want to configure the engine with non-default options
// namely to change the sample rate, framesPerBuffer
// or the output device, you must Stop() the engine before
// calling the respective setter methods
func New() (*Engine, err) {

	var (
		err                     error
		defaultOutputDeviceInfo *portaudio.DeviceInfo
		streamParameters        portaudio.StreamParameters
	)

	// initialize portaudio (this must be done to use *any* of portaudio's API)
	if err = portaudio.Initialize(); err != nil {
		return nil, err
	}
	// get device info of the default output device
	if defaultOutputDeviceInfo, err = portaudio.DefaultOutputDevice(); err != nil {
		return nil, err
	}
	// get stream parameters for the default output device
	// we're requesting low latency parameters (gotta go fast)
	streamParameters := portaudio.LowLatencyParameters(nil, defaultOutputDeviceInfo)
	// also stereo is preferred/required
	// if it doesn't support stereo, well, you'll find out when Start() is called won't you
	streamParameters.Output.Channels = 2

	return &Engine{
		streamParameters:   streamParameters, // <--- default configuration
		stream:             nil,
		tables:             map[int]*Table{},
		activeTablePlayers: []*TablePlayer{},
		initialized:        true,
		started:            false,
	}, nil
}

// list audio devices
// this returns a listing of the portaudio device info objects
// which you can explore at your leisure (or use for a call to  SetDevice())
// who knows?
func (e *Engine) ListDevices() ([]*portaudio.DeviceInfo, err) {
	return portaudio.Devices()
}

// setters for sample rate, framesPerBuffer, and output device
// the engine must be 1. initialized and 2. stopped to configure
// these parameters, otherwise an error is returned
// Nota Bene, you might be able to set values which will incur
// an error once Start() is called afterwards (resuming playback)
// these setters *wont* show you whether the values set are acceptable
func (e *Engine) SetSampleRate(sr float64) error {
	// lock self temporarily as we update it
	b.Lock()
	defer b.Unlock()
	// check for errors first
	if e.started {
		return ConfigureWhileStreamStarted
	}
	if !e.initialized {
		return ConfigureWhileNotInitialized
	}
	// update the stream parameters (atomically)
	e.streamParameters.SampleRate = sr
	return nil
}
func (e *Engine) SetFramesPerBuffer(framesPerBuffer int) error {
	// lock self temporarily as we update it
	b.Lock()
	defer b.Unlock()
	// check for errors first
	if e.started {
		return ConfigureWhileStreamStarted
	}
	if !e.initialized {
		return ConfigureWhileNotInitialized
	}
	// update the stream parameters (atomically)
	e.streamParameters.FramesPerBuffer = framesPerBuffer
	return nil
}
func (e *Engine) SetDevice(deviceInfo *portaudio.DeviceInfo) error {
	// lock self temporarily as we update it
	b.Lock()
	defer b.Unlock()
	// check for errors first
	if e.started {
		return ConfigureWhileStreamStarted
	}
	if !e.initialized {
		return ConfigureWhileNotInitialized
	}
	// create a new (low latency) stream parameter configuration (for the new device)
	// hopefully you passed in an output device, otherwise Start() explodes an error
	streamParameters := portaudio.LowLatencyParameters(nil, deviceInfo)
	// force stereo
	// the output device *must* support stereo (otherwise this entire library will not work)
	// if it doesn't support stereo, well, you'll find out when Start() is called won't you
	streamParameters.Output.Channels = 2
	// update the stream parameters (atomically)
	e.streamParameters = streamParameters
	return nil
}

// start streaming audio to the output device
func (e *Engine) Start() error {
	//TODO
}

// stop streaming audio to the output device
func (e *Engine) Stop() error {
	//TODO
}

// should be called after you're done utilizing the Engine
// this is mainly here to clean up portaudio
// if you wish to resuse the Engine after Close(), call
// Reinitialize() first
func (e *Engine) Close() error {
	return portaudio.Terminate()
}

// used to (re)initialize the engine (should you have called Close() prior)
// New() automatically initializes portaudio
func (e *Engine) Reinitialize() error {
	if e.initialized {
		return fmt.Errorf("already initialized this stereophonic engine")
	}
	return portaudio.Initialize()
}

// loads a soundfile into a sample slot
// (which internally just loads a table with the soundfile frames,
// then saves a reference in the engine)
func (e *Engine) Load(slot int, sampleFile string) err {
	//TODO
}

// trigger playback of a table
// internally, creates a table player
// adds the tableplayer to the activeTablePlayers of engine
// (which begins ticking (computing) audio frames for output)
// the returned *TablePlayer can be used to control playback further in realtime
func (e *Engine) Play(slot int) *TablePlayer {
	//TODO
}

// the callback which portaudio uses to fill the output buffer
func (e *Engine) streamCallback(out []float32) {
	// for every output frame
	for i := 0; i < len(out); i += 2 {
		out[i], out[i+1] = e.tick()
	}
}

// compute 1 frame of audio in the engine's system
func (e *Engine) tick() (float32, float32) {
	//TODO
}
