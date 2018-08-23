package main

import (
	// "github.com/rivo/tview"
	"github.com/stygian-phrygian/stereophonic"
	"log"
	"os"
	"time"
)

var (
	// tempo
	BPM                            = 120.0
	quarterNoteDurationInSeconds   = 60.0 / BPM
	eigthNoteDurationInSeconds     = quarterNoteDurationInSeconds / 2.0
	sixteenthNoteDurationInSeconds = eigthNoteDurationInSeconds / 2.0

	// 16 step sequence (where lastStep determines its length)
	stepSequence     = [16]Step{}
	sequenceLength   = 16
	currentStepIndex = 0
	currentStep      Step
	normalGain       = 0.0
	accentGain       = 7.0
	// synth config
	noteOffset            = -64
	filterAttackInSeconds = 0.01
	filterDecayInSeconds  = 1.0
	filterCutoff          = 0.06
	filterResonance       = 0.9
	filterEnvelopeDepth   = 0.5
	// waveforms
	waveformSlot = 0
)

// represents the necessary information for each step in the sequence
type Step struct {
	Note   int  // the note value (pitch)
	Accent bool // accent (similar to tb303)
	Slide  bool //  whether this note is slid to the next note
	Off    bool // whether this is actually a note off
}

func main() {

	// create an engine
	e, err := stereophonic.New()
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()
	// start engine
	if err := e.Start(); err != nil {
		log.Fatal(err)
	}
	// find where the sample directory is located (relative to GOPATH)
	sampleDirectory := os.Getenv("GOPATH") + "src/github.com/stygian-phrygian/stereophonic/_examples/samples/"
	// load waveform
	if err := e.Load(waveformSlot, sampleDirectory+"808kick.wav"); err != nil {
		log.Fatal(err)
	}

	// populate a step sequence
	stepSequence[0x0] = Step{Note: 0, Slide: true}
	stepSequence[0x1] = Step{Note: 0, Slide: true}
	stepSequence[0x2] = Step{Note: 15}
	stepSequence[0x3] = Step{Off: true}
	stepSequence[0x4] = Step{Note: 0, Slide: true}
	stepSequence[0x5] = Step{Note: 0, Slide: true}
	stepSequence[0x6] = Step{Note: 0, Slide: true}
	stepSequence[0x7] = Step{Note: 0, Slide: true}
	stepSequence[0x8] = Step{Note: 0}
	stepSequence[0x9] = Step{Note: 10}
	stepSequence[0xa] = Step{Note: 15, Slide: true}
	stepSequence[0xb] = Step{Note: 12}
	stepSequence[0xc] = Step{Note: 0, Accent: true}
	stepSequence[0xd] = Step{Off: true}
	stepSequence[0xe] = Step{Note: 0}
	stepSequence[0xf] = Step{Note: 3, Accent: true}

	// create an unlimited duration playback event
	// so we can simulate a mono-synth with proper note slides
	event, _ := e.Prepare(waveformSlot, 0, 0)
	// we use a single cycle waveform, so turn on looping
	event.SetLooping(true)
	event.SetLoopSlice(0.0, 0.001)
	// initially set the gain to negative infinity
	event.SetGain(stereophonic.GainNegativeInfinity)
	// set initial filter values
	event.SetFilterCutoff(filterCutoff)
	event.SetFilterResonance(filterResonance)
	event.SetFilterEnvelopeOn(true)
	event.SetFilterEnvelopeDepth(filterEnvelopeDepth)
	event.SetFilterAttack(filterAttackInSeconds)
	event.SetFilterDecay(filterDecayInSeconds)
	event.SetFilterSustain(0)
	event.SetFilterResonance(filterResonance)
	// commence play
	e.Play(event)

	// for each step in our step sequence
	go func() {
		for {
			// get the current step
			step := stepSequence[currentStepIndex]
			// determine next step index
			currentStepIndex = (currentStepIndex + 1) % sequenceLength
			// determine what to do at this moment in time with
			// these current step values
			if step.Off {
				// === off-step ===
				event.SetGain(stereophonic.GainNegativeInfinity)
			} else {
				// === on-step ===
				// set note value
				event.SetNote(step.Note + noteOffset)
				// if the step's slide is on
				if step.Slide {
					// slide to next step (assuming that it
					// is also an on-step)
					if !stepSequence[currentStepIndex].Off {
						event.SetNote(
							stepSequence[currentStepIndex].Note+noteOffset,
							sixteenthNoteDurationInSeconds)
					}
				} else {
					// else the slide wasn't on and we
					// reset the envelopes to attack stage
					event.Attack()
				}
				// set gain for this step
				event.SetGain(normalGain)
				if step.Accent {
					event.SetGain(accentGain)
				}
			}
			waitStep()
		}
	}()

	// allow events to occur
	time.Sleep(time.Duration(32 * quarterNoteDurationInSeconds * float64(time.Second)))
	// turn off the (indefinite and looping) event
	event.SetLooping(false)
	event.Release()

}

func waitStep() {
	time.Sleep(time.Duration(sixteenthNoteDurationInSeconds * float64(time.Second)))
}
