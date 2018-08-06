package stereophonic

import (
	"math"
)

// a "simple" 12db per octave resonant filter algorithm
//
//
// adapted from here:
// http://www.martin-finke.de/blog/articles/audio-plugins-013-filter/
// which is itself adapted from this:
// http://www.musicdsp.org/showone.php?id=29
//
// thanks guys

// filter type enum
type FilterMode int

const (
	NoFilter FilterMode = iota
	LPFilter
	HPFilter
	BPFilter
)

type filter struct {
	filterMode FilterMode
	// cutoff, q (resonance), and the sample rate of the filter
	// nb, cutoff cannot (ever) equal 1
	cutoff, resonance float64
	// these values are used in the actual IIR filter computation
	feedback               float64
	buf0, buf1, buf2, buf3 float64
}

func newFilter() *filter {

	f := &filter{
		filterMode: LPFilter,
		cutoff:     0.9999999999999999,
		resonance:  0.0,
	}

	f.calculateFeedback()

	return f

}

func (f *filter) calculateFeedback() {
	f.feedback = math.Min(f.resonance+f.resonance/(1.0-f.cutoff), 1.0)
}

// filter the input
func (f *filter) tick(input float64) float64 {
	// ordinarily ----------------------------------> (this term    )
	// is buf0 - buf3, but I chose buf0 - buf2... seemed to ease back
	// the filter instability a bit
	f.buf0 += f.cutoff * (input - f.buf0 + f.feedback*(f.buf0-f.buf2))
	f.buf1 += f.cutoff * (f.buf0 - f.buf1)
	f.buf2 += f.cutoff * (f.buf1 - f.buf2)
	f.buf3 += f.cutoff * (f.buf2 - f.buf3)
	switch f.filterMode {
	case LPFilter:
		return f.buf3
	case HPFilter:
		return input - f.buf3
	case BPFilter:
		return f.buf0 - f.buf3
	default: // NoFilter
		return input
	}
}

func (f *filter) setMode(filterMode FilterMode) {
	f.filterMode = filterMode
}

// set the cutoff frequency (as a value from 0.0 < 1.0)
// NB. never set frequency greater than or equal to 1
func (f *filter) setCutoff(cutoff float64) {
	// clamp cutoff such that:
	// 0 <= cutoff < 1
	// cutoff cannot be 1 as this would create a divide by 0 error for the
	// feedback computation)
	f.cutoff = math.Max(math.Min(cutoff, 0.9999999999999999), 0.0)
	f.calculateFeedback()
}

func (f *filter) setResonance(resonance float64) {
	// make sure resonance is always 0 to 1
	f.resonance = math.Max(math.Min(resonance, 0.9999999999999999), 0.0)
	f.calculateFeedback()
}
