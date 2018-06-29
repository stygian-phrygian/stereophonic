package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

/* ISSUES
reconsidering API choices, particularly how to start/stop/open/close streams
should we have 4 methods 1-1 for the above functionality or
2 methods, open/close or start/stop which each do 2 of the functions above
this library isn't meant to be streaming to multple devices, so we don't need multiple
stream control really


SOLUTIONS
	all the setters can swiftly write to the streamParameters object *even* when not
	stopped.  We could just say, "to update a configuration of stream parameters,
	the effect will only occur *after* you've stopped the stream."  our Start()
	will then always open/start a portaudio stream, and our Stop() will always
	stop/close a portaudio stream using the recently altered stream parameters.
	This way we can avoid getting weird StreamDoesNotExist errors and CannotConfigure***
	errors.  In summary, Setters only apply after a stream is stopped/started.
	Close() calls portaudio.Terminate() and you can't call it multiple times (we'll flag
	for this).  If you want to Reinitialize() portaudio, we'll provide a method to do so,
	which you  (again) cannot call multiple times (we'll flag for this too).

*/

const (
	CannotConfigureWhileStreamStarted  error = fmt.Errorf("cannot configure engine while started, must call Stop() first")
	CannotConfigureWhileNotInitialized error = fmt.Errorf("cannot configure engine while not initialized, must call Reinitialize() first")
	AlreadyInitialized                 error = fmt.Errorf("already initialized engine")
	AlreadyStarted                     error = fmt.Errorf("already started engine")
	StreamDoesNotExist                 error = fmt.Errorf("stream does not exist")
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
	//
	// side note:
	// portaudio allows multiple initialization (of which an equivalent number of
	// terminations are subsequently demanded).  I chose to forgo this behavior
	// Why?
	// 1. disallow strange situations
	// 2. I want the engine API to be minimal, just New & Close (not New, Close, Initialize, Terminate, etc)
	//
	// If I didn't put initialization into New(), then I couldn't acquire
	// default stream parameters (a query to portaudio which requires initialization prior)
	// hence New() would be doing nothing but building a struct.  You'd need a subsequent Initialize(), then
	// SetDefaults() (strictly in that order).  Similarly, Close() should close the underlying stream & terminate
	// portaudio (instead of a separate Close() and Terminate()).  That said, in the event that you Close()
	// but happen to want to boot up portaudio again, there's a provided Reinitialize() method for you.
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
// the engine must be:
// 1. initialized
// 2. stopped
// otherwise an error is returned
// Nota Bene, these setters *wont* show you whether
// the values set are acceptable
func (e *Engine) SetSampleRate(sr float64) error {
	// lock self temporarily as we update it
	b.Lock()
	defer b.Unlock()
	// check for errors first
	if e.started {
		return CannotConfigureWhileStreamStarted
	}
	if !e.initialized {
		return CannotConfigureWhileNotInitialized
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
		return CannotConfigureWhileStreamStarted
	}
	if !e.initialized {
		return CannotConfigureWhileNotInitialized
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
		return CannotConfigureWhileStreamStarted
	}
	if !e.initialized {
		return CannotConfigureWhileNotInitialized
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
	// update the stream parameters (atomically)
	e.streamParameters = streamParameters
	return nil
}

// open *and* start an audio stream
func (e *Engine) Start() error {
	// update atomically
	e.Lock()
	defer e.Unlock()

	// check that we aren't already started
	if e.started {
		return AlreadyStarted
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

	// check stream exists
	if e.stream != nil {
		return StreamDoesNotExist
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
	e.stream = nil
	// return without error
	return nil
}

// should be called after you're done utilizing the Engine
// this is mainly here to clean up portaudio
// if you wish to resuse the Engine after Close(), call
// Reinitialize() first
func (e *Engine) Close() error {
	var (
		err error
	)

	// update atomically
	e.Lock()
	defer e.Unlock()

	// try to close the stream (if it exists ofc)
	if stream != nil {
		if err = e.stream.Close(); err != nil {
			// if it failed, return the error
			return err
		}
	}
	// otherwise the stream closed successfully
	// flag that we aren't started anymore
	e.started = false

	// remove the stream
	e.stream = nil

	// remove the active playing tables
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
// This provides the option of simply restarting the engine (after a call Close())
// without reloading all the sample files into tables
func (e *Engine) Reinitialize() error {
	// update atomically
	e.Lock()
	defer e.Unlock()

	// firstly, check that the intialized flag is false
	if e.initialized {
		// return error if it's true, we don't want to initialize twice
		//
		//
		return AlreadyInitialized
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
