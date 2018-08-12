package stereophonic

import (
	"math"
)

// aggregate gain related functions here (to avoid source duplication)

const (
	// A gain of -80 db is a 1/1000th reduction in amplitude, which is (for
	// our musical purposes) enough to be considered amplitude == 0.  With
	// this constant, we allow an enduser to specify that they want the
	// gain to be basically negative infinity (and thereby avoid very small
	// float64 calculations)
	GainNegativeInfinity float64 = -80.0
)

// convert audio db to amplitude
func decibelsToAmplitude(db float64) float64 {
	amplitude := 0.0
	if db > GainNegativeInfinity {
		// 20*log10(amplitude/1.0) = db
		//    log10(amplitude/1.0) = db/20
		//          amplitude      = 10^(db/20)
		amplitude = math.Pow(10, db*0.05)
	}
	return amplitude
}
