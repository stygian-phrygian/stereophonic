package stereophonic

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/mkb218/gosndfile/sndfile"
)

// A table represents audio frame data, and associated important
// playback data (channels, samplerate, fileName)
// It's used to hold single-cycle waveforms or whole files.
// NB. this struct holds an *entire* sound file's audio data in memory.
// Perhaps it's not efficient, but memory is cheap boy!
// Tables are essentially immutable after creation.

type Table struct {
	name       string
	channels   int
	sampleRate float64   // <--- float64 for convenience
	samples    []float64 // interleaved
	nFrames    int
	sync.Mutex // lock when mutating the samples
}

func (b *Table) Name() string {
	return b.name
}

func (b *Table) Channels() int {
	return b.channels
}

func (b *Table) SampleRate() float64 {
	return b.sampleRate
}

func (b *Table) NFrames() int {
	return b.nFrames
}

// create a new table from a sound file
// (most common use case for Table)
func NewTable(soundFileName string) (*Table, error) {
	b := &Table{}
	err := b.loadFile(soundFileName)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// create a new table and fill it with a single cycle waveform
func NewTableSine(frequency, phase, sampleRate float64) (*Table, error) {
	b := &Table{}
	err := b.loadSine(frequency, phase, sampleRate)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// create a new table and fill it with a single cycle waveform
func NewTableSaw(frequency, phase, sampleRate float64) (*Table, error) {
	b := &Table{}
	err := b.loadSaw(frequency, phase, sampleRate)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// create a new table and fill it with a single cycle waveform
func NewTableSquare(frequency, phase, sampleRate float64) (*Table, error) {
	b := &Table{}
	err := b.loadSquare(frequency, phase, sampleRate)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// create a new table and fill it with noise of a certain duration in seconds
func NewTableWhiteNoise(duration, sampleRate float64) (*Table, error) {
	b := &Table{}
	err := b.loadWhiteNoise(duration, sampleRate)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// create a new table and fill it with an impulse train
func NewTableImpulseTrain(frequency, phase, sampleRate float64) (*Table, error) {
	b := &Table{}
	err := b.loadImpulseTrain(frequency, phase, sampleRate)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// fill a Table with a sound file's samples
func (b *Table) loadFile(soundFileName string) error {

	var info sndfile.Info

	// try to open the sound file
	sf, err := sndfile.Open(soundFileName, sndfile.Read, &info)
	if err != nil {
		return err
	}
	defer sf.Close()

	// create a buffer to read our sound file's frames into
	// NB. the returned buffer of frame data
	// contains samples that are *interleaved*
	// ie. a stereo sound file is represented as a
	// 1 dimensional array where left/right pairs of samples
	// are located at index n, n+1 respectively
	n := int64(sf.Format.Channels) * sf.Format.Frames
	samples := make([]float64, n)

	// try to read the (entire) soundfile in 1 go
	framesRead, err := sf.ReadFrames(samples)
	if err != nil {
		return err
	}

	// lock self temporarily as we update it
	b.Lock()
	defer b.Unlock()

	// modify self
	b.name = soundFileName
	b.channels = int(sf.Format.Channels)
	b.sampleRate = float64(sf.Format.Samplerate)
	b.samples = samples
	b.nFrames = int(framesRead)

	// return without error
	return nil

}

// clamp phase
// make sure phase is in range [0, 1)
// else wrap around into the proper range
func clampPhase(phase float64) float64 {
	if phase < 0 {
		phase = math.Abs(phase)
	}
	if phase >= 1.0 {
		phase = phase - math.Floor(phase)
	}
	return phase
}

// creates a mono buffer of audio suitable for holding a single
// cycle waveform given a certain frequency and sampleRate
// NB. this doesn't actually populate the buffer with a waveform, it
// only creates it
func createSingleCycle(frequency, sampleRate float64) []float64 {
	var n int
	switch frequency {
	case 0.0:
		n = 1 // dc offset
	default:
		n = int(math.Abs(sampleRate / frequency))
	}
	return make([]float64, n)
}

// generates a single cycle sine waveform inside the Table
// The frequency should be >= 0 and <= the nyquist frequency (sampleRate/2)
// else aliasing will occur.  The phase should be in the range [0, 1) and
// anything outside of that will be wrapped around.
func (b *Table) loadSine(frequency, phase, sampleRate float64) error {

	// check that the sample rate is valid
	if sampleRate < 1 {
		return errors.New(fmt.Sprintf("Cannot create a buffer with sample rate: %f", sampleRate))
	}
	// create the samples necessary to store the waveform
	samples := createSingleCycle(frequency, sampleRate)

	// create iteration variables
	var (
		tau   float64 = 2.0 * math.Pi
		omega float64 = tau * frequency
		x     float64
	)

	// make sure phase is in range [0, 1)
	phase = clampPhase(phase)
	// then convert phase to radians
	phase = tau * phase

	// iterate the samples, writing the waveform data to it
	for i, _ := range samples {
		x = float64(i) / sampleRate
		samples[i] = math.Sin(omega*x + phase)
	}

	// update self
	b.Lock()
	defer b.Unlock()
	b.name = "sine"
	b.channels = 1
	b.sampleRate = sampleRate
	b.samples = samples
	b.nFrames = len(samples)

	return nil
}

// generates a single cycle sawtooth waveform inside the Table
// The frequency should be >= 0 and <= the nyquist frequency (sampleRate/2)
// else aliasing will occur.  The phase should be in the range [0, 1) and
// anything outside of that will be wrapped around.
func (b *Table) loadSaw(frequency, phase, sampleRate float64) error {

	// check that the sample rate is valid
	if sampleRate < 1 {
		return errors.New(fmt.Sprintf("Cannot create a buffer with sample rate: %f", sampleRate))
	}

	// create samples to store the waveform
	samples := createSingleCycle(frequency, sampleRate)

	// calculate correct starting phase and increment
	phase = clampPhase(phase)
	phase = phase*2.0 - 1.0
	phaseIncrement := 1.0 / float64(len(samples))

	// iterate samples
	// ramp up sawtooth starting from phase
	for i, _ := range samples {
		samples[i] = phase
		// update phase
		phase += phaseIncrement
		if phase > 1.0 {
			phase = -1.0
		}
	}

	// update self
	b.Lock()
	defer b.Unlock()
	b.name = "saw"
	b.channels = 1
	b.sampleRate = sampleRate
	b.samples = samples
	b.nFrames = len(samples)

	return nil
}

// generates a single cycle square waveform inside the Table
// The frequency should be >= 0 and <= the nyquist frequency (sampleRate/2)
// else aliasing will occur.  The phase should be in the range [0, 1) and
// anything outside of that will be wrapped around.
func (b *Table) loadSquare(frequency, phase, sampleRate float64) error {

	// check that the sample rate is valid
	if sampleRate < 1 {
		return errors.New(fmt.Sprintf("Cannot create a buffer with sample rate: %f", sampleRate))
	}

	// create the samples necessary to store the waveform
	samples := createSingleCycle(frequency, sampleRate)

	// we basically use a sawtooth waveform algorithm (see above)
	// at twice the speed of the provided frequency
	// to calculate when the square wave switches polarity

	// calculate correct starting phase and increment
	phase = clampPhase(phase)
	phase = phase*2.0 - 1.0
	//  note the 2.0, not 1.0 (twice the speed)
	phaseIncrement := 2.0 / float64(len(samples))

	// iterate samples (very similar to saw waveform code above)
	for i, _ := range samples {
		if phase < 0 {
			samples[i] = -1.0
		} else {
			samples[i] = 1.0
		}
		// update phase
		phase += phaseIncrement
		if phase > 1.0 {
			phase = -1.0
		}
	}

	// update self
	b.Lock()
	defer b.Unlock()
	b.name = "square"
	b.channels = 1
	b.sampleRate = sampleRate
	b.samples = samples
	b.nFrames = len(samples)

	return nil
}

// seed random number generator which will be used
// for generating noise
var (
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// generates white noise inside the buffer
func (b *Table) loadWhiteNoise(duration, sampleRate float64) error {

	// check that the sample rate is valid
	if sampleRate < 1 {
		return errors.New(fmt.Sprintf("Cannot create a buffer with sample rate: %f", sampleRate))
	}

	n := int(duration * sampleRate)
	samples := make([]float64, n)

	for i, _ := range samples {
		samples[i] = rng.Float64()*2.0 - 1.0
	}

	// update self
	b.Lock()
	defer b.Unlock()
	b.name = "white-noise"
	b.channels = 1
	b.sampleRate = sampleRate
	b.samples = samples
	b.nFrames = len(samples)

	return nil
}

// generates a single cycle impulse train waveform inside the Table
// The frequency should be >= 0 and <= the nyquist frequency (sampleRate/2)
// else aliasing will occur.  The phase should be in the range [0, 1) and
// anything outside of that will be wrapped around.
func (b *Table) loadImpulseTrain(frequency, phase, sampleRate float64) error {

	// check that the sample rate is valid
	if sampleRate < 1 {
		return errors.New(fmt.Sprintf("Cannot create a buffer with sample rate: %f", sampleRate))
	}

	samples := createSingleCycle(frequency, sampleRate)

	// make sure phase is valid [0, 1)
	phase = clampPhase(phase)
	// find phase's index into our samples
	// i is between [0, N) where N == len(samples)
	i := int(phase * float64(len(samples)))
	if i > 0 {
		// set impulse at len(samples) - phase
		samples[len(samples)-i] = 1.0
	} else {
		samples[0] = 1.0
	}

	// update self
	b.Lock()
	defer b.Unlock()
	b.name = "impulse-train"
	b.channels = 1
	b.sampleRate = sampleRate
	b.samples = samples
	b.nFrames = len(samples)

	return nil
}
