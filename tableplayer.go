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
	// amplitude envelope (when release occurs on this adsr, the doneAction
	// of the playback event is run (removing it from the active playback
	// events of the engine)
	amplitudeADSREnvelope *adsrEnvelope
	// filters (we need 2 of them for the stereo playback, as the filters
	// only operate on a single channel themselves)
	filterLeft, filterRight *filter
	// flag whether the filter envelope is on (avoids unnecessary computation)
	filterEnvelopeOn bool
	// filter cutoff & depth (used *only* when the filter envelope is on)
	// set filter depth to negative 1 to invert the envelope
	filterCutoff, filterEnvelopeDepth float64
	// filter cutoff envelope
	// (look inside tick() to see how the above filter variables are used)
	filterADSREnvelope *adsrEnvelope
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
	// slide factor is calculated by SetSpeed (which has an optional slide
	// time duration argument)
	// slideFactor is always greater or equal to 0
	slideFactor float64
	// true  --> looping
	// false --> one shot
	isLooping bool
	// offset (and looping offset) frame indices
	start     int
	end       int
	loopStart int
	loopEnd   int
	// whether we are in reverse playback
	isReversed bool
	// whether we are finished playback (that is reached the start or end)
	// in a particular direction (forwards or reverse).  NB. this cannot be
	// true if we're looping
	isFinished bool
	// "kontrol rate" (a csound inspired naming idiosyncracy).  These are
	// used to limit expensive computation by limiting their occurence.
	// The tradeoff is what is known as "zipper noise" but acceptable.
	// Thus far, kRate is only used in the filter cutoff envelope (of which
	// setting the cutoff induces costly division each call).  kRate could
	// theoretically be runtime available as a setter (altering kMaxTicks).
	kRate                   float64
	kCurrentTick, kMaxTicks int
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

	// create filters
	filterLeft := newFilter()
	filterRight := newFilter()

	// create amplitude ADSR envelope (with default values)
	defaultAmplitudeADSRAttack := 0.0
	defaultAmplitudeADSRDecay := 1.0
	defaultAmplitudeADSRSustainLevel := 1.0
	defaultAmplitudeADSRRelease := 0.001
	amplitudeADSREnvelope, err := newADSREnvelope(
		defaultAmplitudeADSRAttack,
		defaultAmplitudeADSRDecay,
		defaultAmplitudeADSRSustainLevel,
		defaultAmplitudeADSRRelease,
		sampleRate)
	if err != nil {
		return nil, err
	}

	// create filter cutoff ADSR envelope (with default values)
	defaultFilterEnvelopeDepth := 0.5
	defaultFilterADSRAttack := 0.0
	defaultFilterADSRDecay := 1.0
	defaultFilterADSRSustainLevel := 1.0
	defaultFilterADSRRelease := 0.001
	filterADSREnvelope, err := newADSREnvelope(
		defaultFilterADSRAttack,
		defaultFilterADSRDecay,
		defaultFilterADSRSustainLevel,
		defaultFilterADSRRelease,
		sampleRate)
	if err != nil {
		return nil, err
	}

	// k-rate (default 100hz)
	kRate := 100.0

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
		filterLeft:             filterLeft,
		filterRight:            filterRight,
		amplitudeADSREnvelope:  amplitudeADSREnvelope,
		filterADSREnvelope:     filterADSREnvelope,
		filterEnvelopeOn:       false,
		filterEnvelopeDepth:    defaultFilterEnvelopeDepth,
		table:                  t,
		phase:                  0.0,
		phaseIncrement:         srFactor, /* speed == 1.0 at *player's* sampleRate */
		targetPhaseIncrement:   srFactor, /* where we want to eventually arrive    */
		slideFactor:            0.0,      /* how fast we arrive there              */
		isLooping:              false,
		isReversed:             false,
		isFinished:             false,
		start:                  0,
		end:                    t.nFrames - 1,
		loopStart:              0,
		loopEnd:                t.nFrames - 1,
		kRate:                  kRate,
		kCurrentTick:           0,
		kMaxTicks:              int(sampleRate/kRate + 1),
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

	// check if we are finished progression (forwards or backwards)
	// if looping is on, this will be false (necessarily)
	if tp.isFinished {
		left = 0.0
		right = 0.0
		// edge case: we must still tick the amplitude adsr envelope.
		// the doneAction (which deactivates the playback event) is
		// only run when the amplitude adsr envelope finishes its
		// release stage (which can only happen if tick() is called to
		// progress it).  We can't run this critical callback otherwise.
		tp.amplitudeADSREnvelope.tick()
		// tick the filter cutoff envelope too for symmetry (if it's on)
		if tp.filterEnvelopeOn {
			tp.filterADSREnvelope.tick()
			// and update the kRate variables
			tp.kCurrentTick++
			tp.kCurrentTick %= tp.kMaxTicks
		}
		//
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

	// filter
	//
	// if the filter cutoff envelope is on
	// update the filter cutoff with an adsr envelope
	if tp.filterEnvelopeOn {
		// only update filter cutoff every tp.kMaxTicks (which is
		// dependent on kRate).  This creates some zipper noise, but
		// it's computationally cheaper (and hopefully acceptable).
		if tp.kCurrentTick == 0 {
			cutoff := tp.filterCutoff +
				tp.filterADSREnvelope.tick()*tp.filterEnvelopeDepth
			tp.filterLeft.setCutoff(cutoff)
			tp.filterRight.setCutoff(cutoff)
		} else {
			// else progress time in the filter cutoff adsr
			// envelope (with a tick but ignore result), skipping
			// the expensive filter coefficient recalculation
			tp.filterADSREnvelope.tick()
		}
		tp.kCurrentTick++
		tp.kCurrentTick %= tp.kMaxTicks
	}
	left = tp.filterLeft.tick(left)
	right = tp.filterRight.tick(right)

	// add dc offset
	left += tp.dcOffset
	right += tp.dcOffset

	// multiply by amplitude, adsr amplitude envelope, and the balance
	a := tp.amplitude * tp.amplitudeADSREnvelope.tick()
	left *= a * tp.balanceMultiplierLeft
	right *= a * tp.balanceMultiplierRight

	// update phase
	tp.phase += tp.phaseIncrement

	// update phase increment
	// explanation:
	// phase increment determines our rate of playback or how often the
	// phase (table index) changes its current index in the table.  slide
	// factor determines how *quickly* we can alter our phase increment
	// until it reaches a target phase increment (this simulates pitch
	// slurring).  There are 3 cases to consider, the phase increment
	// and target phase increment being: equal, less than, or greater than
	// one another, in the latter 2 cases, we approach the target by
	// slide factor (and correct for overshoot).
	switch {
	case tp.phaseIncrement == tp.targetPhaseIncrement:
		// do nothing if we're at the target
		break
	case tp.phaseIncrement < tp.targetPhaseIncrement:
		// ramp up
		tp.phaseIncrement += tp.slideFactor
		// correct for overshoot
		if tp.phaseIncrement > tp.targetPhaseIncrement {
			tp.phaseIncrement = tp.targetPhaseIncrement
		}
	case tp.phaseIncrement > tp.targetPhaseIncrement:
		// ramp down
		tp.phaseIncrement -= tp.slideFactor
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
			tp.isFinished = true

		// reverse (no looping)
		case next < start:
			// reset phase to start
			tp.phase = float64(start)
			// flag that we are finished playback
			tp.isFinished = true
		}
	}

	// return the stereo frame
	return left, right
}

// set looping mode, true => looping on, false => looping off
func (tp *TablePlayer) SetLooping(loopingOn bool) {
	if loopingOn {
		tp.isFinished = false
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

// reset playback position
// begin playback at "start" position if forwards playback
// begin playback at "end" position if reverse playback
// This doesn't restart the amplitude ADSR envelope, just the playback position
//
// NB. if you call SetReverse(true) immediately after creation of the
// TablePlayer, the starting phase of the table will be at 0, subsequently
// *finishing* playback on the next tick() (as reverse playback will move
// backwards towards 0 (and hit it instantly)).  Hence, if you want to reverse
// right after TablePlayer creation, call SetReverse(true) *then* Trigger() to
// fix the phase to the end of the table
func (tp *TablePlayer) Trigger() {

	if tp.isReversed {
		// reverse playback
		// begin playback at "end" position
		tp.phase = float64(tp.end)
	} else {
		// forwards playback
		// begin playback at "start" position
		tp.phase = float64(tp.start)
	}
	tp.isFinished = false
}

// (re)sets the envelopes to their attack stage, regardless of current stage
func (tp *TablePlayer) Attack() {
	tp.amplitudeADSREnvelope.attack()
	tp.filterADSREnvelope.attack()
}

// (re)sets the envelopes to their release stage, regardless of current stage
// NB, this will (possibly) remove the tableplayer from the active players if
// it fully releases, as the amplitude adsr (specifically) has a doneAction callback
// which removes the playback event from the active events in the engine
// (assuming it fully releases, that is enters an off stage)
func (tp *TablePlayer) Release() {
	tp.amplitudeADSREnvelope.release()
	tp.filterADSREnvelope.release()
}

// set the DC offset (obviously)
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
// only accepts arguments > 0
// an optional slide time (specified in seconds) is allowed
func (tp *TablePlayer) SetSpeed(speed float64, slideTime ...float64) {
	// return on unacceptable speeds
	if speed <= 0 {
		return
	}

	// save the current speed
	currentSpeed := tp.phaseIncrement

	// calculate the target speed
	// (correcting for SR mismatch with the srFactor)
	targetSpeed := speed * tp.srFactor

	// handle if we're in reverse playback
	if currentSpeed < 0 {
		targetSpeed *= -1.0
	}

	// assign to target speed
	tp.targetPhaseIncrement = targetSpeed

	// assign the target speed *also* to the current phase increment
	// NB. if there is a (valid) slide time given below
	// we'll immediately update the current phase increment
	tp.phaseIncrement = targetSpeed

	// if we're given a slide time
	if slideTime != nil {
		// if the slide time is valid
		if slideTimeInSeconds := slideTime[0]; slideTimeInSeconds > 0.0 {
			// calculate how fast we should change to the target
			// speed given our current speed, the slide time
			// specified, and the sample rate

			// first, reset phase increment to its original speed
			tp.phaseIncrement = currentSpeed

			// it'll take n ticks to approach our target speed
			n := slideTimeInSeconds * tp.sampleRate

			// this is how much we alter the phase increment each
			// of the n ticks
			tp.slideFactor = math.Abs(targetSpeed-currentSpeed) / n
		}
	}

}

// like SetSpeed, but integer note values which represent chromatic pitch offset
func (tp *TablePlayer) SetNote(n int, slideTime ...float64) {
	tp.SetSpeed(math.Pow(2, float64(n)/12.0), slideTime...)
}

// turn on reverse playback(if it's not already on)
// NB. if you just want a one-shot reverse playback of a table, call
// SetReverse(true); Trigger() (in that order).  You must call Trigger() to
// inform the tableplayer that you want the phase of the table to begin at the
// end (upon table player creation, its default phase is set at the start
// of the table).
func (tp *TablePlayer) SetReverse(isReversed bool) {
	// save it
	tp.isReversed = isReversed
	// if forwards playback and isReversed == true, set reverse playback
	// or
	// if reverse playback and isReversed == false, set foward playback
	if (tp.phaseIncrement > 0 && isReversed == true) ||
		(tp.phaseIncrement < 0 && isReversed == false) {
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

// setters for the filter
func (tp *TablePlayer) SetFilterMode(filterMode FilterMode) {
	tp.filterLeft.setMode(filterMode)
	tp.filterRight.setMode(filterMode)
}
func (tp *TablePlayer) SetFilterCutoff(cutoff float64) {
	tp.filterLeft.setCutoff(cutoff)
	tp.filterRight.setCutoff(cutoff)
	// save the filter cutoff (in case it's used for envelope computation)
	//
	// NB. filter.setCutoff(x) limits x automatically (which ranges the
	// cutoff such that  0 < cutoff < 1).  Therefore, we just let it clamp
	// the cutoff, and save what it actually returned in the TablePlayer
	// (the left filter was arbitrarily chosen here, it doesn't matter)
	tp.filterCutoff = tp.filterLeft.cutoff
}
func (tp *TablePlayer) SetFilterResonance(resonance float64) {
	tp.filterLeft.setResonance(resonance)
	tp.filterRight.setResonance(resonance)
}

// setters filter cutoff envelope

// continuously updating the filter coefficients is expensive, hence there's an
// option to just turn the filter cutoff envelope on/off
func (tp *TablePlayer) SetFilterEnvelopeOn(filterEnvelopeOn bool) {
	tp.filterEnvelopeOn = filterEnvelopeOn
}

// how much to augment the filter cutoff when the envelope is on
func (tp *TablePlayer) SetFilterEnvelopeDepth(filterEnvelopeDepth float64) {
	tp.filterEnvelopeDepth = filterEnvelopeDepth
}

//adsr times
func (tp *TablePlayer) SetFilterAttack(attackTimeInSeconds float64) {
	tp.filterADSREnvelope.setAttack(attackTimeInSeconds)
}
func (tp *TablePlayer) SetFilterDecay(decayTimeInSeconds float64) {
	tp.filterADSREnvelope.setDecay(decayTimeInSeconds)
}
func (tp *TablePlayer) SetFilterSustain(sustainLevel float64) {
	tp.filterADSREnvelope.setSustain(sustainLevel)
}
func (tp *TablePlayer) SetFilterRelease(releaseTimeInSeconds float64) {
	tp.filterADSREnvelope.setRelease(releaseTimeInSeconds)
}

// (amplitude) ADSR setters
// can't use struct embedding here, as I might have multiple envelopes in the
// future... who knows
func (tp *TablePlayer) SetAmplitudeAttack(attackTimeInSeconds float64) {
	tp.amplitudeADSREnvelope.setAttack(attackTimeInSeconds)
}
func (tp *TablePlayer) SetAmplitudeDecay(decayTimeInSeconds float64) {
	tp.amplitudeADSREnvelope.setDecay(decayTimeInSeconds)
}
func (tp *TablePlayer) SetAmplitudeSustain(sustainLevel float64) {
	tp.amplitudeADSREnvelope.setSustain(sustainLevel)
}
func (tp *TablePlayer) SetAmplitudeRelease(releaseTimeInSeconds float64) {
	tp.amplitudeADSREnvelope.setRelease(releaseTimeInSeconds)
}
