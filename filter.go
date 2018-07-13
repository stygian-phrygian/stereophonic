package stereophonic

// a "simple" resonant 24db per octave filter algorithm
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

func newFilter() (*filter, error) {

	f := &filter{
		filterMode: LPFilter,
		cutoff:     0.999,
		resonance:  0.0,
	}

	f.calculateFeedback()

	return f, nil

}

// it's very critical the feedback is controlled
// lest the filter become unstable
// so FIXME that instability, k
func (f *filter) calculateFeedback() {
	f.feedback = f.resonance + f.resonance/(1.0-f.cutoff)
}

// filter the input
func (f *filter) tick(input float64) float64 {
	f.buf0 += f.cutoff * (input - f.buf0 + f.feedback*(f.buf0-f.buf3))
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
	// make sure cutoff isn't 1 (as this would create a divide
	// by 0 error for the feedback computation)
	if 0 <= cutoff && cutoff < 0.9999 {
		f.cutoff = cutoff
		f.calculateFeedback()
	}
}

func (f *filter) setResonance(resonance float64) {
	if resonance >= 0 {
		f.resonance = resonance
		f.calculateFeedback()
	}
}
