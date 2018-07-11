package stereophonic

import (
	"errors"
	"fmt"
	"math"
)

// TablePlayer (obviously enough) keeps track of playback
// within 1 table (which is a contiguous array of (interleaved) samples)
//
// This is basically a wave table synthesizer for 1 (stereo output) voice,
// and represents 1 occurence of that voice (thereafter it's intended to be
// garbage collected)
//
// Currently, TablePlayer only plays Tables of mono or stereo audio.
// Mono (1 channel) audio will be automatically converted to stereo (2 channel)
// output (which is the only output available in the engine)
//
// The Table (from which audio frames are drawn) *cannot* be changed
// after construction of a TablePlayer, although a number of playback
// variables are modifiable in real-time, as was mentioned above, this
// struct represents *1* instance of playback for *1* table.
//
// Things which are modifiable in realtime:
// the speed (pitch), amplitude, dc-offset, start/end points,
// as well as loop-start/loop-end points, forwards and reverse playback
//

type TablePlayer struct {
	// sample rate of the original sound file
	sampleRate float64
	// used to determine mismatch between playback sample rate and original sound file sample rate
	srFactor float64
	// amplitude is multiplied by the result of reading the table
	amplitude float64
	// dc offset is added to the result of reading the table
	dcOffset float64
	// balance controls how much the left or right channel is dampened
	// NB. we don't need to store a "balance" variable as the setter
	// calculates the left/right channel multipliers
	balanceMultiplierLeft, balanceMultiplierRight float64
	// the frame data we read from (the Table)
	table *Table
	// current frame index in the table
	// which ranges from 0 to table.nFrames - 1
	phase float64
	// speed with which to increment phase (our index into the table)
	// we can adjust the playback rate changing this value
	// when
	//   phaseIncrement > 0 --> forwards playback
	//   phaseIncrement < 0 --> reverse playback
	phaseIncrement float64
	// this is a destination rate of playback (phase increment)
	// we want to acheive.  It's necessary for simulating pitch slides
	targetPhaseIncrement float64
	// determines how long a slide will take
	// glide factor =  1/(glideTimeInMilliseconds/1000)*sampleRate
	// this is always greater or equal to 0
	glideFactor float64
	// true  --> looping
	// false --> one shot
	isLooping bool
	// offset (and looping offset) frame indices
	start     int
	end       int
	loopStart int
	loopEnd   int
	// whether we are in reverse playback
	reversed bool
	// whether we are done playback (cannot be true if looping)
	donePlayback bool
}

func NewTablePlayer(t *Table, sampleRate float64) (*TablePlayer, error) {

	// check table is non-nil
	if t == nil {
		return nil, errors.New("Cannot create a table player with a nil table")
	}

	// check nFrames >= 1
	if t.nFrames < 1 {
		return nil, fmt.Errorf("Cannot create a table player from a table with %d frames", t.nFrames)
	}

	// check sample rate is valid
	if sampleRate < 1 {
		return nil, fmt.Errorf("Cannot create a table player with samplerate: %f", sampleRate)
	}

	// account for possible mismatch in the
	// table's sample rate and the table-player's sample rate
	srFactor := t.sampleRate / sampleRate

	// create the table player
	tp := &TablePlayer{
		sampleRate:             sampleRate,
		srFactor:               srFactor,
		amplitude:              1.0,
		dcOffset:               0.0,
		balanceMultiplierLeft:  1.0,
		balanceMultiplierRight: 1.0,
		table:                t,
		phase:                0.0,
		phaseIncrement:       srFactor, /* speed == 1.0 at *player's* sampleRate */
		targetPhaseIncrement: srFactor, /* where we want to eventually arrive    */
		glideFactor:          0.0,      /* how fast we arrive there              */
		isLooping:            false,
		reversed:             false,
		donePlayback:         false,
		start:                0,
		end:                  t.nFrames - 1,
		loopStart:            0,
		loopEnd:              t.nFrames - 1,
	}
	// correct possible sample rate mismatch between the table and the table player
	tp.SetSpeed(1.0)

	return tp, nil
}

// returns the current audio frame, "ticking" table playback 1 frame further
// and updating relevant playback related variables
// this function always returns a stereo audio frame (left and right), to clarify:
// for stereo channel tables, the returned frame is the processed left/right channels
// for mono channel tables, the returned frame is the processed mono channel duplicated
func (tp *TablePlayer) tick() (float64, float64) {

	var (
		left  float64
		right float64
	)

	// check that we aren't done playback already
	if tp.donePlayback {
		left = 0.0
		right = 0.0
		return left, right
	}

	// get current frame index of our table
	i := int(tp.phase)

	// read the samples in this frame
	switch tp.table.channels {
	// mono
	case 1:
		left = tp.table.samples[i]
		right = left
	// stereo
	case 2:
		left = tp.table.samples[2*i]
		right = tp.table.samples[2*i+1]
	//
	default:
		left = 0.0
		right = 0.0
	}

	// balance the signal
	left *= tp.balanceMultiplierLeft
	right *= tp.balanceMultiplierRight

	// multiply by amplitude
	left *= tp.amplitude
	right *= tp.amplitude

	// add dc offset
	left += tp.dcOffset
	right += tp.dcOffset

	// update phase
	tp.phase += tp.phaseIncrement

	// update phase increment
	// explanation:
	// phase increment determines our rate of playback or how often the
	// phase (table index) changes its current index in the table.  glide
	// factor determines how *quickly* we can alter our phase increment
	// until it reaches a target phase increment (this simulates pitch
	// slurring).  There are 3 cases to consider, the phase increment
	// and target phase increment being: equal, less than, or greater than
	// one another, in the latter 2 cases, we approach the target by
	// glide factor step amount (and correct for overshoot).
	switch {
	case tp.phaseIncrement == tp.targetPhaseIncrement:
		// do nothing if we're at the target
		break
	case tp.phaseIncrement < tp.targetPhaseIncrement:
		// ramp up
		tp.phaseIncrement += tp.glideFactor
		// correct for overshoot
		if tp.phaseIncrement > tp.targetPhaseIncrement {
			tp.phaseIncrement = tp.targetPhaseIncrement
		}
	case tp.phaseIncrement > tp.targetPhaseIncrement:
		// ramp down
		tp.phaseIncrement -= tp.glideFactor
		// correct for overshoot
		if tp.phaseIncrement < tp.targetPhaseIncrement {
			tp.phaseIncrement = tp.targetPhaseIncrement
		}
	}

	// compute next frame index (phase)
	// there are 4 main cases to consider:
	// forwards, forwards-looping, reverse, reverse-looping

	// check that our *next* frame index will be within bounds
	next := int(tp.phase)

	var (
		start            = tp.start
		end              = tp.end
		loopStart        = tp.loopStart
		loopEnd          = tp.loopEnd
		forwardsPlayback = tp.phaseIncrement >= 0.0
	)

	if tp.isLooping {

		// out of bounds detection
		switch {

		// forwards looping
		case forwardsPlayback && loopEnd < next:
			// reset phase to loop start
			tp.phase = float64(loopStart)

		// reverse looping
		case !forwardsPlayback && next < loopStart:
			// reset phase to loop end
			tp.phase = float64(loopEnd)
		}
	} else {

		// out of bounds detection
		switch {

		// forwards (no looping)
		case next > end:
			// reset phase to end
			tp.phase = float64(end)
			// flag that we are finished playback
			tp.donePlayback = true

		// reverse (no looping)
		case next < start:
			// reset phase to start
			tp.phase = float64(start)
			// flag that we are finished playback
			tp.donePlayback = true
		}
	}

	// return the stereo frame
	return left, right
}

// set looping mode, true => looping on, false => looping off
func (tp *TablePlayer) SetLooping(loopingOn bool) {
	if loopingOn {
		tp.donePlayback = false
	}
	tp.isLooping = loopingOn

}

// set start/end
// where start/end are in the range [0, 1)
// and start < end
func (tp *TablePlayer) SetSlice(start, end float64) {

	// clamp start/end in range [0, 1)
	start = math.Min(math.Max(0, start), 1.0)
	end = math.Min(math.Max(0, end), 1.0)

	// check that start < end
	if start < end {
		// compute the new table frame indices
		s := int(float64(tp.table.nFrames-1) * start)
		e := int(float64(tp.table.nFrames-1) * end)

		// check that the difference of the
		// frame indices is >= 1
		if d := e - s; d >= 1 {
			// save the new start/end indices
			tp.start = s
			tp.end = e
		}
	}
}

// set loop start/end
// where loop start/end are in the range [0, 1)
// this is conceptually the same as StartEnd above (copy-pasted it really)
//
// NB. this code allows one to create loop points that are larger than the
// start/end points which is kind of weird... but I'll allow it.
func (tp *TablePlayer) SetLoopSlice(loopStart, loopEnd float64) {

	// clamp start/end in range [0, 1)
	loopStart = math.Min(math.Max(0, loopStart), 1.0)
	loopEnd = math.Min(math.Max(0, loopEnd), 1.0)

	// check that start < end
	if loopStart < loopEnd {

		// compute the new table frame indices
		s := int(float64(tp.table.nFrames-1) * loopStart)
		e := int(float64(tp.table.nFrames-1) * loopEnd)

		// check that the difference of the
		// frame indices is >= 1
		if d := e - s; d >= 1 {

			// save the new start/end indices
			tp.loopStart = s
			tp.loopEnd = e
		}
	}
}

// (re)initialize table player for playback
// begin playback at "start" position if forwards playback
// begin playback at "end" position if reverse playback
//
// NB. if you call SetReverse(true) immediately after creation of the
// TablePlayer, the starting phase of the table will be at 0, subsequently
// *finishing* playback on the next tick() (as reverse playback will move
// backwards towards 0 (and hit it instantly)).  Hence, if you want to reverse
// right after TablePlayer creation, call SetReverse(true) *then* Trigger() to
// fix the phase to the end of the table
func (tp *TablePlayer) Trigger() {

	if tp.reversed {
		// reverse playback
		// begin playback at "end" position
		tp.phase = float64(tp.end)
	} else {
		// forwards playback
		// begin playback at "start" position
		tp.phase = float64(tp.start)
	}
}

func (tp *TablePlayer) SetDCOffset(dc float64) {
	tp.dcOffset = dc
}

// specify the gain of the TablePlayer using decibels (0dBFS)
// ex. tp.SetGain(6.0)  // => 6db increase in volume
// ex. tp.SetGain(-3.0) // => 3db decrease in volume
// ex. tp.SetGain(0.0)  // => 0db (no change in volume)
// awesome brief discussion here:
//   https://sound.stackexchange.com/a/25533
func (tp *TablePlayer) SetGain(db float64) {
	// db        = 20*log10(amplitude/1.0)
	// amplitude = 10^(db/20)
	tp.amplitude = math.Pow(10, db*0.05)
}

// adjust playback rate of the table
// only accepts values > 0
func (tp *TablePlayer) SetSpeed(speed float64) {
	// return on unacceptable speeds
	if speed <= 0 {
		return
	}

	// calculate the speed (correcting for SR mismatch with the srFactor)
	s := speed * tp.srFactor

	// assign the target phase increment (the phase increment will
	// approach this by glide factor increment/decrements)
	tp.targetPhaseIncrement = s
	// if there isn't a glide factor (glide factor <= 0)
	// immediately assign phaseIncrement to targetPhaseIncrement
	if tp.glideFactor <= 0.0 {
		tp.phaseIncrement = s
	}

	// on reversed playback
	// invert the direction of the phase increment & target phase increment
	if tp.reversed {
		tp.phaseIncrement *= -1.0
		tp.targetPhaseIncrement *= -1.0
	}

}

// set the duration of time a glide to another pitch should take
func (tp *TablePlayer) SetGlide(g float64) {
	if g > 0 {
		// glide factor is how much we increment/decrement the phase
		// increment until we approach the target phase increment
		// yes I realize that's confusing
		tp.glideFactor = g
		//FIXME: g needs to be very small to have any noticeable effect
		// ex: 0.00001
	}
}

// turn on reverse playback(if it's not already on)
// NB. if you just want a one-shot reverse playback of a table, call
// SetReverse(true); Trigger() (in that order).  You must call Trigger() to
// inform the tableplayer that you want the phase of the table to begin at the
// end (upon table player creation, its default phase is set at the start
// of the table).
func (tp *TablePlayer) SetReverse(reverseOn bool) {
	// save it
	tp.reversed = reverseOn
	// if forwards playback and reverseOn == true, set reverse playback
	// or
	// if reverse playback and reverseOn == false, set foward playback
	if (tp.phaseIncrement > 0 && reverseOn == true) ||
		(tp.phaseIncrement < 0 && reverseOn == false) {
		// invert all the phase increment variables
		tp.phaseIncrement *= -1.0
		tp.targetPhaseIncrement *= -1.0
	}
}

// set the balance of the signal
// -1: left (right fully muted)
//  0: center (nothing altered)
//  1: right (left fully muted)
func (tp *TablePlayer) SetBalance(balance float64) {
	// make sure balance is between -1 and 1 (inclusive)
	if balance < -1.0 || 1.0 < balance {
		return
	}
	// determine what to multiple the left/right channels by
	switch {
	case balance == 0.0:
		tp.balanceMultiplierLeft = 1.0
		tp.balanceMultiplierRight = 1.0
	case 0.0 < balance:
		tp.balanceMultiplierLeft = 1.0 - balance
		tp.balanceMultiplierRight = 1.0
	case balance < 0.0:
		tp.balanceMultiplierLeft = 1.0
		tp.balanceMultiplierRight = 1.0 + balance
	}
}
