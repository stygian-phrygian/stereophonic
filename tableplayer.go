package stereophonic

import (
	"errors"
	"fmt"
	"math"
)

// TablePlayer (obviously enough) keeps track of playback
// within 1 table, a collection of audio frames
// (which is itself a contiguous array of (interleaved) samples)
//
// This is basically a wave table synthesizer for 1 (stereo output) voice
//
// Currently, TablePlayer only plays Tables of mono or stereo audio.
// Mono (1 channel) audio will be automatically converted to stereo (2 channel)
// output (which is the only output available)
//
// The Table (from which audio frames are drawn) *cannot* be changed
// after construction of a TablePlayer, although a number of playback
// variables are modifiable in real-time.
//
// namely:
// the speed (pitch), amplitude, dc-offset, start/end points,
// as well as loop-start/loop-end points, forwards and reverse playback,
// sample-offset
//TODO: glide, distortion/bitcrush,  attack-hold-decay
//
/*

issues:
	how to have a glide() and speed() function
	integrate Trigger() with attack hold decay
	waveshaping distortion introduces (possibly) aliased frequencies

	clipping audio?  should we flatten all input outside of [-1..1]
	thereby achieving a distortion effect.  Maybe we should instead
	have a DB() func over an Amp() func, ie, it should change the
	resulting amplitude logarithmically (not linearly)

	we need a method to signal when playback is done

	we really do need a filter (to avoid aliasing on pitch change)

solutions?

	glide could be 2 variables, a portamentoIncrement and destinationPitch variable
	we stop incrementing our speed variable (by portamentoIncrement) when
	we've reached the destination pitch
	so to slide somewhere do: Portamento(1.0); Speed(2.0)
	Portamento can be specified in seconds (relative to the SR)
*/

type TablePlayer struct {
	// sample rate of the original sound file
	sampleRate float64
	// used to determine mismatch between playback sample rate and original sound file sample rate
	srFactor float64
	// amplitude is multiplied by the result of reading the table
	amplitude float64
	// dc offset is added to the result of reading the table
	dcOffset float64
	// the frame data we read from (the Table)
	table *Table
	// current frame index in the table
	// which ranges from 0 to table.nFrames - 1
	phase float64
	// speed with which to increment phase
	// (simulates pitch)
	// when
	//   phaseIncrement > 0 --> forwards playback
	//   phaseIncrement < 0 --> reverse playback
	phaseIncrement float64
	// true  --> looping
	// false --> one shot
	isLooping bool
	// offset (and looping offset) frame indices
	start     int
	end       int
	loopStart int
	loopEnd   int
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
		sampleRate:     sampleRate,
		srFactor:       srFactor,
		amplitude:      1.0,
		dcOffset:       0.0,
		table:          t,
		phase:          0.0,
		phaseIncrement: srFactor, /* speed == 1.0 at player's sampleRate */
		isLooping:      false,
		donePlayback:   false,
		start:          0,
		end:            t.nFrames - 1,
		loopStart:      0,
		loopEnd:        t.nFrames - 1,
	}
	// correct possible sample rate mismatch between the table and the table player
	tp.Speed(1.0)

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

	// multiply by amplitude
	left *= tp.amplitude
	right *= tp.amplitude

	// add dc offset
	left += tp.dcOffset
	right += tp.dcOffset

	// compute next frame index (phase)
	// there are 4 main cases to consider:
	// forwards, forwards-looping, reverse, reverse-looping

	// update phase
	tp.phase += tp.phaseIncrement

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

// set looping mode
func (tp *TablePlayer) Looping(loopingOn bool) {
	if loopingOn {
		tp.donePlayback = false
	}
	tp.isLooping = loopingOn

}

// set start/end
// where start/end are in the range [0, 1)
// and start < end
func (tp *TablePlayer) Slice(start, end float64) {

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
func (tp *TablePlayer) LoopSlice(loopStart, loopEnd float64) {

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
func (tp *TablePlayer) Trigger() {

	if tp.phaseIncrement >= 0 {
		// if forwards playback
		// begin playback at "start" position
		tp.phase = float64(tp.start)
	} else {
		// else reverse playback
		// begin playback at "end" position
		tp.phase = float64(tp.end)
	}
}

// return next len(out) samples from the TablePlayer,
// *overwriting* whatever is in out already
// the argument is assumed to be *stereo* interleaved samples
func (tp *TablePlayer) ProcessReplacing(out []float64) {
	var nFrames = len(out) >> 1 // divide by 2
	for i := 0; i < nFrames; i++ {
		out[2*i], out[2*i+1] = tp.tick()
	}
}

// return next len(out) samples from the TablePlayer,
// *mixing* with whatever is in there already
// the argument is assumed to be *stereo* interleaved samples
func (tp *TablePlayer) Process(out []float64) {
	var (
		left, right float64
	)
	for i := 0; i < len(out); i += 2 {
		left, right = tp.tick() // get table player's next frame
		out[i] += left          // mix left
		out[i+1] += right       // mix right
	}
}

func (tp *TablePlayer) DCOffset(dc float64) {
	tp.dcOffset = dc
}

// specify the amplitude of the TablePlayer using decibels (0dBFS)
// ex. tp.DB(6.0)  // => 6db increase in volume
// ex. tp.DB(-3.0) // => 3db decrease in volume
// ex. tp.DB(0.0)  // => 0db (no change in volume)
// awesome brief discussion here:
//   https://sound.stackexchange.com/a/25533
func (tp *TablePlayer) DB(db float64) {
	// db        = 20*log10(amplitude/1.0)
	// amplitude = 10^(db/20)
	tp.amplitude = math.Pow(10, db*0.05)
}

// adjust playback rate of the table
// only accepts values > 0
func (tp *TablePlayer) Speed(speed float64) {
	// if speed is an acceptable value
	if speed > 0.0 {
		// handle cases where playback is forwards/reverse
		if tp.phaseIncrement > 0.0 {
			// forwards playback
			tp.phaseIncrement = speed * tp.srFactor
		} else {
			// reverse playback
			tp.phaseIncrement = speed * tp.srFactor * -1.0
		}
	}
}

// turn on reverse playback(if it's not already on)
func (tp *TablePlayer) Reverse() {
	if tp.phaseIncrement > 0.0 {
		tp.phaseIncrement *= -1.0
	}
}

// turn on fowards playback (if it's not already on)
func (tp *TablePlayer) Forward() {
	if tp.phaseIncrement < 0 {
		tp.phaseIncrement *= -1.0
	}
}
