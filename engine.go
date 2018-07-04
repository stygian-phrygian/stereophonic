package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

/*
ISSUES

how to remove playback of "done" tableplayers from the activeTablePlayers
set in Engine?

what happens if we add 2 playback events with different timeToLive


We have to rename "active" set

SOLUTIONS

could specify a duration in Play()?
maybe Play() takes a tableplayer, starttime, and a duration (eerily
similar to csound no...) and builds an Event object which is what
is actually tracked in Engine.  An Event = a TablePlayer & a timeToLive.
timeToLive is initially some number of frames that the playback event should
persist (which can be decrementede every tick() or maybe every FramesPerBuffer
ticks() of the engine)


What is an event?  It's a starttime, duration, and source of audio.
Instead of Prepare() we can actually just create a new Event?

*/

const (
	EngineAlreadyInitialized error = fmt.Errorf("engine is already initialized")
	EngineNotInitialized     error = fmt.Errorf("engine isn't initialized")
	EngineAlreadyStarted     error = fmt.Errorf("engine is already started")
	EngineNotStarted         error = fmt.Errorf("engine isn't started")
	TableDoesNotExist        error = fmt.Errorf("table does not exist")
)

// a playback event is
// a source of audio (the TablePlayer)
// and the remaining number of audio frames left to compute/tick-off
type playbackEvent struct {
	ticksRemaining int
	TablePlayer
}

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
	// set (really a map, cuz golang has no set datatype) of (currently)
	// active sources of audio
	// appending to this set is done by Play()
	// removal is done automatically after the events expire through
	// a goroutine which listens for event expiration
	activePlaybackEvents map[playbackEvent]bool
	// flag to check whether portaudio is initialized
	initialized bool
	// flag to check whether the portaudio stream started
	started bool
	// lock (only for configuring the engine pre-playback)
	sync.Mutex
}

// prepare an engine
// internally this:
// initializes portaudio
// acquires the default output device stream parameters with low latency configuration
//
// this does *not* start an audio stream , it just configures one
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
	if !e.initialized {
		return nil, EngineNotInitialized
	}
	return portaudio.Devices()
}

// setters for sample rate, framesPerBuffer, and output device
// these methods only work *before* you call Start()
// if you call them while the engine is started, they won't have
// any effect, you must Stop() the engine.
// if you call them after Close(), they will return an error
// and you must Reopen()
// Nota Bene:
// these setters *wont* show you whether the values set are acceptable ;)
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
	// edge case call sequence of: New() -> [stream: nil], Close()
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

// prepare/create a playback event NB. this does *not* start playback
// immediately, but allows you to configure the playback of the audio file
// before it begins (variables like speed, offset, volume, etc)
func (e *Engine) Prepare(slot int, durationInMilliseconds int) (*playbackEvent, error) {

	// check that the duration makes sense
	//FIXME

	// check that we have this slot
	if table, exists := e.tables[slot]; !exists {
		return nil, TableDoesNotExist
	}
	// create a new tableplayer (with the recently acquired table)
	if tablePlayer, err := NewTablePlayer(table); err != nil {
		return err
	}

	return &playbackEvent{
		convertMillisecondsToFrames(durationInMS),
		tablePlayers,
	}

}

func (e *Engine) convertMillisecondsToFrames(durationInMilliseconds int) {
}

// triggers playback of a table player at startime for duration
// starttime is the number of ms from now to being playback (therefore > 0)
// duration is the time in number of ms the TablePlayer should be active
// multiple triggers of the *exact* same TablePlayer will have no additional
// effect. If you want a polyphonic simulation of playing a single table, you
// must call Prepare() for each voice
func (e *Engine) Play(starttime, duration float64, tablePlayers ...*TablePlayer) {
	// check that there are table players firstly
	if tablePlayers != nil {
		return
	}

	// add the tableplayers to the internal active players "set"
	for _, tablePlayer := range tablePlayers {

	}

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
