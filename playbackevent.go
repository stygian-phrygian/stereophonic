package stereophonic

import (
	"math"
)

const (
	playbackLimitedDuration int = iota
	playbackUnlimitedDuration
	playbackDelay
)

// a playback event represents a limited/unlimited duration of time to pull
// frames of audio from a tick source (tablePlayer) a playback event can only
// be used *once*, you *cannot* send it to Play() multiple times (it's only
// added once to the engine's active playback events set).
//
// Once its duration is finished, you cannot run it again.  We avoid an edge
// case of a dangling reference having strange side effects by banning the
// reuse of the object after it's finished its duration (be it limited duration
// or unlimited (and released))

type playbackEvent struct {
	// delayInFrames is the number of frames to delay before we begin
	// ticking from our *tablePlayer durationInFrames is how many times we
	// tick() on the *tablePlayer therefore, total frames = delayInFrames +
	// durationInFrames
	//
	// if durationInFrames <= 0, then the event is of *unlimited* duration
	// and Release() must be called to end it.  Release() will defer to the
	// underlying tablePlayer's adsr (calling its Release()) and awaiting
	// until its release stage is completely finished before running the
	// doneAction
	delayInFrames, durationInFrames int
	// the *tablePlayer is what generates frames of audio for us...  this
	// could be abstracted perhaps into an interface with a tick()
	*tablePlayer
	// which state playback is in (on (limited duration), on (unlimited
	// duration), or delayed).  NB. there's no Off stage, as the the adsr
	// envelope should remove the event via the done action
	currentState int
	// this flag handles an edge case where we have a delayInSeconds and a
	// durationSeconds which are both greater than 0, BUT round down into
	// 0.  The edge case is, we start in state=delay, which immediately has
	// no further ticks.  To determine which is the next state to
	// transition into, we could look at the durationInFrames and see if
	// it's greater than 0, but alas, it might == 0 from rounding error
	// (even though we specified an actual limited duration).  Hence, we
	// have an event which we specified as a having a (very small) limited
	// duration and a (very small) delay, but get an unlimited duration
	// event accidentally, which we won't know to Release().  This flag
	// preserves the relevant transition state information.
	isLimitedDuration bool
}

// create/prepare a playback event.
//
// The slot determines which sound file will be played back, delayInSeconds
// specifies how long to wait *after* Play() is called *and* before actual playback
// commences, durationSeconds specifies how long to continue playing *after*
// actual playback commences (after delayInSeconds duration) NB. this does
// *not* start playback immediately, but allows you to configure the playback
// before it begins (variables like speed, offset, volume, etc)
//
// delayInSeconds <= 0 are ignored
// durationInSeconds <= 0 results in an *unlimited* duration playback event,
// (ie. you MUST call Release() if you want it to end)
//
func (e *Engine) Prepare(slot int, delayInSeconds, durationInSeconds float64) (*playbackEvent, error) {
	e.Lock()
	defer e.Unlock()

	// check if stream started (which is necessary
	// to get the correct stream sample rate for table player creation)
	if !e.started {
		return nil, errorEngineNotStarted
	}

	// check that we have this slot
	table, exists := e.tables[slot]
	if !exists {
		return nil, errorTableDoesNotExist
	}

	// (try to) create a new tableplayer (with the recently acquired table)
	tablePlayer, err := newTablePlayer(table, e.streamSampleRate)
	if err != nil {
		return nil, err
	}

	// ignore delayInSeconds <= 0
	delayInSeconds = math.Max(delayInSeconds, 0.0)

	// calculate the delay/duration in frames of the playback event
	delayInFrames := int(delayInSeconds * e.streamSampleRate)
	durationInFrames := int(durationInSeconds * e.streamSampleRate)

	// create the playback event struct
	p := &playbackEvent{
		delayInFrames:     delayInFrames,
		durationInFrames:  durationInFrames,
		tablePlayer:       tablePlayer,
		currentState:      playbackLimitedDuration,
		isLimitedDuration: durationInSeconds > 0.0, // <--- edge case
	}

	// determine what our initial state is (that is, playbackDelay,
	// playbackUnlimitedDuration, or playbackLimitedDuration)
	if delayInSeconds > 0 {
		p.currentState = playbackDelay
	} else {
		// else we have either limited or unlimited duration playback
		if durationInSeconds > 0 {
			p.currentState = playbackLimitedDuration
		} else {
			p.currentState = playbackUnlimitedDuration
		}
	}

	// attach a callback which removes this playback event from the
	// engine's active playback events once it's "done" (finished duration
	// or released)
	p.amplitudeADSREnvelope.setDoneAction(e.newPlaybackEventDeactivator(p))

	// return a playback event
	return p, nil
}

// compute another tick of the event
func (p *playbackEvent) tick() (float64, float64) {
	var left, right float64

retry:
	switch p.currentState {

	// on (limited duration)
	case playbackLimitedDuration:
		// if there are frames to tick
		if p.durationInFrames > 0 {
			// tick them (and decrement remaining ticks)
			p.durationInFrames--
			left, right = p.tablePlayer.tick()
		} else {
			// change playback to unlimited duration (to allow the
			// release envelope to complete and call its doneAction
			// successfully)
			p.currentState = playbackUnlimitedDuration
			// enter release stage of the amplitude adsr
			p.Release()
			//
			goto retry
		}

	// on (unlimited duration)
	case playbackUnlimitedDuration:
		left, right = p.tablePlayer.tick()

	// on delayed playback
	case playbackDelay:
		// if there are (delay) frames to tick
		if p.delayInFrames > 0 {
			// decrement remaining ticks
			p.delayInFrames--
			left, right = 0.0, 0.0
		} else {
			// else there are no more (delay) frames to tick
			// change the playback state to unlimited/limited duration
			if p.isLimitedDuration {
				p.currentState = playbackLimitedDuration
			} else {
				p.currentState = playbackUnlimitedDuration
			}
			//
			goto retry
		}
	}

	return left, right
}
