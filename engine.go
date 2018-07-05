package stereophonic

import (
	"fmt"
	"github.com/gordonklaus/portaudio"
	"sync"
)

var (
	EngineAlreadyInitialized error = fmt.Errorf("engine is already initialized")
	EngineNotInitialized     error = fmt.Errorf("engine isn't initialized")
	EngineAlreadyStarted     error = fmt.Errorf("engine is already started")
	EngineNotStarted         error = fmt.Errorf("engine isn't started")
	TableDoesNotExist        error = fmt.Errorf("table does not exist")
	InvalidDuration          error = fmt.Errorf("invalid duration of time")
)

// a playback event is
// a source of audio (the TablePlayer)
// the remaining number of audio frames left to compute/tick-off
// when to begin computing them after Play() is called
// a done action function to run (only once) after we've exceeded our duration
type playbackEvent struct {
	startTimeInFrames, durationInFrames int
	*TablePlayer                               // <--- could abstract this into an interface... tick()?
	doneAction                          func() // function to run once duration <= 0 (we're done)
	ranDoneAction                       bool   // flag to determine we ran the done action already
}

// redefine tick event to handle startTimeInFrames/durationInFrames
// and call a done action callback (only once) after it's... uhm done.
func (p *playbackEvent) tick() (float64, float64) {
	// decrement startTimeInFrames, returning nothing
	if p.startTimeInFrames > 0 {
		p.startTimeInFrames -= 1
		return 0.0, 0.0
	}
	// decrement startTimeInFrames, returning a tick()
	if p.durationInFrames > 0 {
		p.durationInFrames -= 1
		return p.TablePlayer.tick()
	}
	// run the done action (once)
	if !p.ranDoneAction {
		p.ranDoneAction = true
		p.doneAction()
	}
	// and return nothing
	return 0.0, 0.0
}

// engine is a type which maintains structural information
// related to playback and device parameters
type Engine struct {
	// lock (only for configuring the engine pre-playback)
	sync.Mutex
	// stream parameters keeps track of relevant
	// playback variables for a stream, namely SampleRate, FramesPerBuffer,
	// and the output device.
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
	// active sources of audio
	// appending to this set is done by Play()
	// removal is done automatically after the events expire through
	// a goroutine which listens for event expiration
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
// this does *not* start an audio stream , it just configures one
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
		newPlaybackEvents:    make(chan *playbackEvent, 128),
		initialized:          true,
		started:              false,
	}, nil
}

// list audio devices
// this returns a listing of the portaudio device info objects
// which you can explore at your leisure,
// maybe for use in SetDevice(), who knows?
func (e *Engine) ListDevices() ([]*portaudio.DeviceInfo, error) {
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
	if err := e.stream.Close(); err != nil {
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
		return EngineAlreadyInitialized
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
		return TableDoesNotExist
	}
	// otherwise safely delete the table at this slot
	delete(e.tables, slot)

	return nil
}

// func which returns a func which is called when our playback event is done
// apparently you *can* delete keys from a map during iteration
// which is when this callback would be called, iterating the active players
// see:
// https://stackoverflow.com/questions/23229975/is-it-safe-to-remove-selected-keys-from-golang-map-within-a-range-loop
func (e *Engine) newDoneAction(p *playbackEvent) func() {
	return func() {
		delete(e.activePlaybackEvents, p)
	}
}

// prepare/create a playback event
//
// slot determines which sound file will be played back
//
// startTimeInMilliseconds specifies how long to wait *after* Play() *and*
// before actual playback commences
//
// durationInMilliseconds specifies how long to continue playing *after*
// actual playback commences (after startTimeInMilliseconds duration)
//
// NB. this does *not* start playback immediately, but allows you to configure
// the playback before it begins (variables like speed, offset, volume, etc)
func (e *Engine) Prepare(slot int, startTimeInMilliseconds, durationInMilliseconds float64) (*playbackEvent, error) {
	e.Lock()
	defer e.Unlock()

	// check if stream started (which is necessary
	// to get the correct stream sample rate)
	if !e.started {
		return nil, EngineNotStarted
	}

	// check that the duration makes sense
	if durationInMilliseconds <= 0.0 || startTimeInMilliseconds < 0.0 {
		return nil, InvalidDuration
	}

	// check that we have this slot
	table, exists := e.tables[slot]
	if !exists {
		return nil, TableDoesNotExist
	}

	// (try to) create a new tableplayer (with the recently acquired table)
	tablePlayer, err := NewTablePlayer(table, e.streamSampleRate)
	if err != nil {
		return nil, err
	}

	// return a playback event
	startTimeInFrames := int(startTimeInMilliseconds * 0.001 * e.streamSampleRate)
	durationInFrames := int(durationInMilliseconds * 0.001 * e.streamSampleRate)
	//
	p := &playbackEvent{}
	p.startTimeInFrames = startTimeInFrames
	p.durationInFrames = durationInFrames
	p.TablePlayer = tablePlayer
	p.ranDoneAction = false
	// attach a callback which removes p from activePlaybackEvents after its done
	p.doneAction = e.newDoneAction(p)
	return p, nil
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
func (e *Engine) streamCallback(out []float32) {

	var (
		left, right float64
	)

	// clear the buffer before proceding (if we don't, the accumulation
	// of prior samples creates explosive dc-offset)
	for i, _ := range out {
		out[i] = 0.0
	}

	// get how many new playback events there are, then append them
	// to the active playback events set
	q := len(e.newPlaybackEvents)
	for i := 0; i < q; i++ {
		e.activePlaybackEvents[<-e.newPlaybackEvents] = true
	}

	// compute each frame from each active playback event
	// remember our map of playbackEvents is being treated like a set
	// hence we're iterating the *keys* of the map
	for playbackEvent, _ := range e.activePlaybackEvents {
		for i := 0; i < len(out); i += 2 {
			left, right = playbackEvent.tick()
			out[i] += float32(left)
			out[i+1] += float32(right)
		}
	}
}

/*
ISSUES


should we have the duration and startTimeOffset in the TablePlayer itself
or the event...?  I feel like these things should belong to the event
and not the TablePlayer

"fatal error: concurrent map writes"

how to remove playback of "done" tableplayers from the activeTablePlayers
set in Engine?

what happens if we add 2 playback events with different timeToLive

We have to rename "active" set

Can you Prepare() a playbackEvent without the engine having started?  Answer:
no, because we need to calculate the sample rate to determine the duration of
time the event lasts, and the sample rate is only VALID after calling Start().
Ex. if we didn't check if engine was started already during Prepare() we'd have
an edge case where we called Start() (got a sample rate), then Stop(), changed
SampleRate, THEN Prepare(), then Start() again, which would (possibly) have an
invalid sample rate.  UNLESS we don't store the ticksRemaining in playbackEvent
but the milliseconds, and only decrement this during playback... This would
require more computation though, with each iteration of tick().  We could have
a DoneAction thread running concurrently, which receives on a channel,
decrement signals.  The DoneAction thread watches the activePlaybackEvents and
removes them accordingly.  Maybe after each iteration of the playback callback,
we can signal through a channel how much time has progressed, removing from
activePlaybackEvents accordingly.  Could even do k-rate style updates,
so like 10 frames, instead of a full FramesPerBuffer frames

what is our TablePlayer tracks the passage of time.  Afterall, it needs
sample rate to compute ticks, it therefore can track how long it's been playing

the event type coul dhave a doneAction callback which is triggered
when a doneEvent occurs somehow?  The DoneAction could just be
a simple removal from the active players set


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
