package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

var (
	errorEngineAlreadyInitialized error = fmt.Errorf("engine is already initialized")
	errorEngineNotInitialized     error = fmt.Errorf("engine isn't initialized")
	errorEngineAlreadyStarted     error = fmt.Errorf("engine is already started")
	errorEngineNotStarted         error = fmt.Errorf("engine isn't started")
	errorTableDoesNotExist        error = fmt.Errorf("table does not exist")
	errorInvalidDuration          error = fmt.Errorf("invalid duration of time")
)

// engine is a struct which maintains structural information
// related to playback and device parameters
type Engine struct {
	// lock (only for configuring the engine pre-playback and load/delete
	// sample slots from the system)
	sync.Mutex
	// stream parameters keeps track of relevant playback variables for a
	// stream, namely SampleRate, FramesPerBuffer, and the output device.
	streamParameters portaudio.StreamParameters
	// the returned "stream" object by portaudio which we can start/stop
	stream *portaudio.Stream
	// the sample rate of the *stream* (not necessarily what you set it as)
	// this is a necessary variable for many audio computations
	streamSampleRate float64
	// mapping from a slot number -> sample (or as we call tables)
	// this collates references to the loaded tables
	tables map[int]*Table
	// set (really a map, cuz golang has no set datatype) of (currently)
	// active sources of audio.  the stream callback is constantly
	// iterating the active playbackEvents calling tick() on each
	activePlaybackEvents map[*playbackEvent]bool
	// (buffered) channel to receive new playback events (for appending to
	// activePlaybackEvents, avoiding a concurrent map failure of Play()
	// directly accessing activePlaybackEvents while the stream is active
	newPlaybackEvents chan *playbackEvent
	// flag to check whether portaudio is initialized
	initialized bool
	// flag to check whether the portaudio stream started
	started bool
}

// prepare an engine
// internally this:
// initializes portaudio
// acquires the default output device stream parameters with low latency configuration
//
// this does *not* start an audio stream, it just configures one
func New() (*Engine, error) {

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
	streamParameters = portaudio.LowLatencyParameters(nil, defaultOutputDeviceInfo)
	// also stereo is preferred/required
	// if it doesn't support stereo, well, you'll find out when Start() is called won't you
	streamParameters.Output.Channels = 2

	return &Engine{
		streamParameters:     streamParameters, // <--- default configuration
		stream:               nil,
		tables:               map[int]*Table{},
		activePlaybackEvents: map[*playbackEvent]bool{},
		newPlaybackEvents:    make(chan *playbackEvent, 128), // <--- magic number
		initialized:          true,
		started:              false,
	}, nil
}

// list audio devices this returns a listing of the portaudio device info
// objects which you can explore at your leisure, maybe for use in SetDevice(),
// who knows?
func (e *Engine) ListDevices() ([]*portaudio.DeviceInfo, error) {
	if !e.initialized {
		return nil, errorEngineNotInitialized
	}
	return portaudio.Devices()
}

// setters for sample rate, framesPerBuffer, and output device these methods
// only work *before* you call Start() if you call them while the engine is
// started, they won't have any effect, you must Stop() the engine.  if you
// call them after Close(), they will return an error and you must Reopen()
// Nota Bene:
// these setters *wont* show you whether the values set are acceptable ;)
func (e *Engine) SetSampleRate(sr float64) error {
	if !e.initialized {
		return errorEngineNotInitialized
	}
	// update the stream parameters
	e.streamParameters.SampleRate = sr
	return nil
}
func (e *Engine) SetFramesPerBuffer(framesPerBuffer int) error {
	if !e.initialized {
		return errorEngineNotInitialized
	}
	// update the stream parameters
	e.streamParameters.FramesPerBuffer = framesPerBuffer
	return nil
}
func (e *Engine) SetDevice(deviceInfo *portaudio.DeviceInfo) error {
	if !e.initialized {
		return errorEngineNotInitialized
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
		return errorEngineNotInitialized
	}

	// check that we aren't already started
	if e.started {
		return errorEngineAlreadyStarted
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
	// save the stream's current sample rate
	streamInfo := stream.Info()
	e.streamSampleRate = streamInfo.SampleRate
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
		return errorEngineNotStarted
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
	if err := e.stream.Close(); err != nil {
		// if it failed, return the error
		return err
	}
	// return without error
	return nil
}

// close the engine, should be called after you're done utilizing it as this
// terminates the underlying portaudio instance.  if you wish to resuse the
// Engine after Close(), call Reopen()
func (e *Engine) Close() error {
	var (
		err error
	)

	// update atomically
	e.Lock()
	defer e.Unlock()

	// first, check if the stream exists
	// edge case call sequence of: New() -> [stream: nil], Close()
	if e.stream != nil {
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
	e.activePlaybackEvents = nil
	e.activePlaybackEvents = map[*playbackEvent]bool{}

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
		return errorEngineAlreadyInitialized
	}

	// now, try to initialize
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	// assuming we successfully initialized portaudio
	// flag that we did so
	e.initialized = true

	return nil
}

// loads a soundfile into a sample slot
// (which internally just loads a table with the soundfile frames,
// then saves a reference in the engine)
func (e *Engine) Load(slot int, soundFileName string) error {
	e.Lock()
	defer e.Unlock()

	table, err := NewTable(soundFileName)
	if err != nil {
		return err
	}
	e.tables[slot] = table

	return nil
}

// deletes a soundfile from a sample slot
func (e *Engine) Delete(slot int) error {
	e.Lock()
	defer e.Unlock()

	// check that the slot exists
	if _, exists := e.tables[slot]; !exists {
		return errorTableDoesNotExist
	}
	// otherwise safely delete the table at this slot
	delete(e.tables, slot)

	return nil
}

// returns a callback which "deactivates" an active event, that is to say,
// removes the event from the active events set
//
// Yea it's a pretty wonky name.  But it's not a "finalizer" or "deconstructor"
// albeit similar to a disposer pattern.  It doesn't technically garbage
// collect the event, it just removes it from activity (or deactivates it).  The
// event still remains however if you have reference(s) to it, losing the
// reference should implicitly garbage collect it.
//
// apparently you *can* delete keys from a map during range iteration (which is
// when this callback would be called (after the event is "released")
// https://stackoverflow.com/questions/23229975/is-it-safe-to-remove-selected-keys-from-golang-map-within-a-range-loop
func (e *Engine) newPlaybackEventDeactivator(p *playbackEvent) func() {
	return func() {
		delete(e.activePlaybackEvents, p)
	}
}

// triggers playback of a table player at startime for duration
// multiple triggers of the *exact* same event (object) will have no additional
// effect. If you want a polyphonic simulation of playing a single table, you
// must call Prepare() for each voice
func (e *Engine) Play(playbackEvents ...*playbackEvent) {
	e.Lock()
	defer e.Unlock()

	// check that there are playback events first
	if playbackEvents == nil {
		return
	}

	// add the events to the internal active event "set"
	for _, playbackEvent := range playbackEvents {
		// queue the playback event (shouldn't block, because the
		// channel is buffered with a large (magic) number unlikely
		// to be surpassed for audio applications...)
		e.newPlaybackEvents <- playbackEvent
	}
}

// the callback which portaudio uses to fill the output buffer
// the output buffer is assumed to be interleaved stereo format
func (e *Engine) streamCallback(out []float32) {

	var (
		left, right float64
	)

	// for each (stereo interleaved) frame
	for n := 0; n < len(out); n += 2 {

		// check if there are new playback events recently encountered
		// append them to the active playback events should they exist
		for i, count := 0, len(e.newPlaybackEvents); i < count; i++ {
			e.activePlaybackEvents[<-e.newPlaybackEvents] = true
		}

		// clear the current frame (to avoid explosive accumulation)
		out[n] = 0.0
		out[n+1] = 0.0

		// for each event in the active playback event
		// accumulate a frame of audio at this frame in the output buffer
		for playbackEvent, _ := range e.activePlaybackEvents {
			left, right = playbackEvent.tick()
			out[n] += float32(left)
			out[n+1] += float32(right)
		}
	}
}
