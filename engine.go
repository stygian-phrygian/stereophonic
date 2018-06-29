package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

/*
ISSUES

trigger playback with playback parameters
ex. e.Play(1, {speed: 1.5, sampleOffset: 0.4})
iunno.  Instead of just a default table playback everytime,
we need a method to trigger specific events


SOLUTIONS

create an event object? which e.Play() can utilize
for preconfiguring a TablePlayer, before tick() is computing
sample frames from it


*/

const (
	EngineAlreadyInitialized error = fmt.Errorf("engine is already initialized")
	EngineNotInitialized     error = fmt.Errorf("engine isn't initialized")
	EngineAlreadyStarted     error = fmt.Errorf("engine is already started")
	EngineNotStarted         error = fmt.Errorf("engine isn't started")
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

// prepare an engine
// interally this:
// initializes portaudio
// acquires the default output device stream parameters with low latency configuration
//
// this does *not* start an audio stream , it just configures one
//
// if you want to configure the engine with non-default options
// namely to change the sample rate, framesPerBuffer
// or the output device, you must Stop() the engine before
// calling the respective setter methods.  The change will occur after
// calling Start()
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
// which you can explore at your leisure,
// maybe for use in SetDevice(), who knows?
func (e *Engine) ListDevices() ([]*portaudio.DeviceInfo, err) {
	return portaudio.Devices()
}

// setters for sample rate, framesPerBuffer, and output device
// the engine must be:
// Nota Bene, these setters *wont* show you whether
// the values set are acceptable ;)
func (e *Engine) SetSampleRate(sr float64) error {
	if !e.initialized {
		return EngineNotInitialized
	}
	// update the stream parameters
	e.streamParameters.SampleRate = sr
	return nil
}
func (e *Engine) SetFramesPerBuffer(framesPerBuffer int) error {
	if !e.initialized {
		return EngineNotInitialized
	}
	// update the stream parameters
	e.streamParameters.FramesPerBuffer = framesPerBuffer
	return nil
}
func (e *Engine) SetDevice(deviceInfo *portaudio.DeviceInfo) error {
	if !e.initialized {
		return EngineNotInitialized
	}
	// create a new (low latency) stream parameter configuration (for the new device)
	// hopefully you passed in an output device, otherwise Start() explodes later)
	streamParameters := portaudio.LowLatencyParameters(nil, deviceInfo)
	// copy the relevant old stream parameter values into the new stream parameter values
	streamParameters.SampleRate = e.streamParameters.SampleRate
	streamParameters.FramesPerBuffer = e.streamParameters.FramesPerBuffer
	// force stereo
	// the output device *must* support stereo (otherwise this entire library will not work)
	// if it doesn't support stereo, well, you'll find out when Start() is called won't you
	streamParameters.Output.Channels = 2
	// update the stream parameters
	e.streamParameters = streamParameters
	return nil
}

// open *and* start an audio stream with existing stream parameters
func (e *Engine) Start() error {
	// update atomically
	e.Lock()
	defer e.Unlock()

	// check that we are initialized
	if !e.initialized {
		return EngineNotInitialized
	}

	// check that we aren't already started
	if e.started {
		return EngineAlreadyStarted
	}

	// open an (output only) stream
	// with prior specified stream parameters & our callback
	stream, err := portaudio.OpenStream(e.streamParameters, e.streamCallback)
	if err != nil {
		return err
	}
	// the stream *opened* successfully
	// now we can *start* it
	if err = stream.Start(); err != nil {
		return err
	}
	// flag that we are started
	e.started = true
	// save a reference to the newly created stream
	e.stream = stream
	// return without error
	return nil
}

// stop *and* close an audio stream
func (e *Engine) Stop() error {
	// update atomically
	e.Lock()
	defer e.Unlock()

	// check that we aren't already stopped
	if !e.started {
		return EngineNotStarted
	}

	// try to stop the stream
	if err := e.stream.Stop(); err != nil {
		// if it failed, return the error
		return err
	}
	// the stream stopped successfully
	// flag that we aren't started anymore
	e.started = false

	// try to close the stream
	if err = e.stream.Close(); err != nil {
		// if it failed, return the error
		return err
	}
	// return without error
	return nil
}

// should be called after you're done utilizing the Engine
// terminates underlying portaudio
// if you wish to resuse the Engine after Close(), call Reopen
func (e *Engine) Close() error {
	var (
		err error
	)

	// update atomically
	e.Lock()
	defer e.Unlock()

	// first, check if the stream exists
	// edge case call sequence: New(), [stream: nil], Close()
	if stream != nil {
		// now, check if we are started (stream is playing currently)
		if e.started {
			// and stop the stream
			if err = e.stream.Stop(); err != nil {
				return err
			}
			// close the stream
			if err = e.stream.Close(); err != nil {
				// if it failed, return the error
				return err
			}
			// the stream was closed successfully
			// flag that we aren't started anymore
			e.started = false
		}
		// if we're not started (stopped)
		// there's nothing to do, Stop() automatically
		// stops/closes the stream after each call
	}

	// remove the active playing tables
	e.activeTablePlayers = nil
	e.activeTablePlayers = []*TablePlayer{}

	// now try to turn off portaudio
	if err := portaudio.Terminate(); err != nil {
		// if it failed, return the error
		return err
	}
	// otherwise termination of portaudio was successful
	// flag that we aren't initialized anymore
	e.initialized = false

	return nil
}

// used to (re)initialize the engine (should you have called Close() prior)
// This provides the option not needing to reload all the tables
func (e *Engine) Reopen() error {
	// update atomically
	e.Lock()
	defer e.Unlock()

	// firstly, check that the intialized flag is false
	if e.initialized {
		return EngineAlreadyInitialized
	}

	// now, try to initialize
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	// assuming we successfully initialized portaudio
	// flag that we did so
	e.initialized = true
}

// loads a soundfile into a sample slot
// (which internally just loads a table with the soundfile frames,
// then saves a reference in the engine)
func (e *Engine) Load(slot int, soundFileName string) err {
	e.Lock()
	defer e.Unlock()

	if table, err := NewTable(soundFileName); err != nil {
		return err
	}

	e.tables[slot] = table
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
