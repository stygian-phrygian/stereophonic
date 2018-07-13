package stereophonic

import (
	"math"
)

var (
	maxCutoff float64 = math.Nextafter(1.0, 0.0)
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
	feedback   float64
	buf0, buf1 float64
}

func newFilter() (*filter, error) {

	f := &filter{
		filterMode: LPFilter,
		cutoff:     0.9999999999999999,
		resonance:  0.0,
	}

	f.calculateFeedback()

	return f, nil

}

func (f *filter) calculateFeedback() {
	f.feedback = f.resonance + f.resonance/(1.0-f.cutoff)
}

// filter the input
func (f *filter) tick(input float64) float64 {
	f.buf0 += f.cutoff * (input - f.buf0 + f.feedback*(f.buf0-f.buf1))
	f.buf1 += f.cutoff * (f.buf0 - f.buf1)
	switch f.filterMode {
	case LPFilter:
		return f.buf1
	case HPFilter:
		return input - f.buf1
	case BPFilter:
		return f.buf0 - f.buf1
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
	// make sure cutoff isn't 1 (as this would create a divide
	// by 0 error for the feedback computation)
	f.cutoff = math.Max(math.Min(cutoff, 0.9999999999999999), 0.0)
	f.calculateFeedback()
}

func (f *filter) setResonance(resonance float64) {
	// make sure resonance is always 0 to 1
	if 0 <= resonance && resonance <= 1 {
		f.resonance = resonance
		f.calculateFeedback()
	}
}
