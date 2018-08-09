package main

import (
	"fmt"
	"github.com/stygian-phrygian/stereophonic"
	"log"
	"os"
	"time"
)

func main() {

	// create an engine
	e, err := stereophonic.New()
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()

	// engine configuration goes here (before start)

	// start engine
	if err := e.Start(); err != nil {
		log.Fatal(err)
	}

	// load a sound file into a slot
	s1 := 1
	sampleDirectory := os.Getenv("GOPATH") + "src/github.com/stygian-phrygian/stereophonic/_examples/samples/"
	if err := e.Load(s1, sampleDirectory+"pad_shot.wav"); err != nil {
		log.Fatal(err)
	}

	// NB. an event only plays once, and cannot be replayed (it won't work)
	event, err := e.Prepare(s1, 0, 0)
	if err != nil {
		log.Fatal(err)
	}
	attackTime := 1.0
	decayTime := 1.0
	sustainLevel := 0.6
	releaseTime := 3.5
	event.SetLooping(true)
	event.SetLoopSlice(0.01, 0.035)
	event.SetAmplitudeAttack(attackTime)
	event.SetAmplitudeDecay(decayTime)
	event.SetAmplitudeSustain(sustainLevel)
	event.SetAmplitudeRelease(releaseTime)
	event.SetFilterMode(stereophonic.HPFilter)
	event.SetFilterCutoff(0.4)
	event.SetFilterResonance(0.9)
	event.SetFilterEnvelopeOn(true)
	event.SetFilterEnvelopeDepth(-0.5) // <--- negative depth inverts the filter cutoff envelope
	event.SetFilterAttack(attackTime * 5.0)
	event.SetFilterDecay(decayTime * 8.0)
	event.SetFilterSustain(sustainLevel * 0.25)
	event.SetFilterRelease(releaseTime)

	fmt.Printf("\n\nPress enter to start the sound *then* press enter again to release it\n")
	fmt.Scanln()
	fmt.Printf("Starting sound with attack: %0.2f, decay: %0.2f, sustain: %0.2f, release: %0.2f\n", attackTime, decayTime, sustainLevel, releaseTime)
	e.Play(event)
	fmt.Scanln()
	fmt.Printf("Releasing sound\n")
	event.Release()

	time.Sleep(time.Duration(1.0 + releaseTime*float64(time.Second)))

}
