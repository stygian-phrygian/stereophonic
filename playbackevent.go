package stereophonic

/*

how about this is a finite state machine.  only a couple of states
off, delay, hold
off        -> off
delay == 0 -> hold
hold  == 0 -> off

func tick() {

	switch currentState {
	case off:
		return 0.0, 0.0
	case delay:
		delay--
		if delay <= 0 {
			// state change [delay -> hold]
			currentState = hold
		}
		return 0.0, 0.0
	case hold:
		if hold != -1 {
		}

	}
	if !off && states[currentState] != 0 {
		states[currentState]--
		return tick()
	} else {
		return 0.0, 0.0
	}
}

func changeState() {
}

*/

// a playback event represents a definite/indefinite duration of time
// to pull frames of audio from a tick source (TablePlayer)

// a source of audio (the TablePlayer)
// the remaining number of audio frames left to compute/tick-off
// when to begin computing them after Play() is called
// a done action function to run (only once) after we've exceeded our duration
type playbackEvent struct {
	// delayInFrames is the number of frames to delay before we begin
	// ticking from our *TablePlayer
	// durationInFrames is how many times we tick() on the *TablePlayer
	// therefore, total frames = delayInFrames + durationInFrames
	delayInFrames, durationInFrames int
	// the *TablePlayer is what generates frames of audio for us...
	// this could be abstracted perhaps into an interface with a tick()
	*TablePlayer
	// function to run once durationInFrames <= 0 (ie. we're done playback)
	// or Done() is called
	doneAction func()
	// flag to determine if we entered a done state
	// and ran the doneAction already
	donePlayback bool
	// flag to determine if we're an indefinite event (meaning that the
	// doneAction will only be called by Done() and not automatically once
	// we've broached a durationInFrames number of tick())
	// this flag (should be) set to true (only) if durationInSeconds <= 0
	indefinitePlayback bool
}

// redefine tick() to handle delayInFrames/durationInFrames
// and call a done action (only once) after we tick() past durationInFrames
//FIXME: handle indefinite events
func (p *playbackEvent) tick() (float64, float64) {

	// check that we haven't entered a "done" state
	if p.donePlayback {
		return 0.0, 0.0
	}

	if p.delayInFrames > 0 {
		// decrement delayInFrames, returning nothing
		p.delayInFrames -= 1
		return 0.0, 0.0
	}

	if p.durationInFrames > 0 {
		// decrement delayInFrames, returning a tick()
		p.durationInFrames -= 1
		return p.TablePlayer.tick()
	}

	// if we've made it here, run the doneAction() (only once)
	p.donePlayback = true
	p.doneAction()

	// and return nothing if we keep getting called past our durationInFrames
	return 0.0, 0.0
}

// immediately put the playbackEvent into a "done" state
//FIXME: handle indefinite events
func (p *playbackEvent) Done() {

	// this is admittedly kind of a hack as it's a race condition I think
	// but won't panic at runtime as it's not modifying any maps/slices
	p.delayInFrames = 0
	p.durationInFrames = 0
}
