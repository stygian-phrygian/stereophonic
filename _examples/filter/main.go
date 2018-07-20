package main

import (
	"fmt"
	"github.com/stygian-phrygian/stereophonic"
	"log"
	"math"
	"os"
	"time"
)

func main() {

	// create engine
	e, err := stereophonic.New()
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()

	//
	// configure if you desire so, here
	//

	// start engine
	if err := e.Start(); err != nil {
		log.Fatal(err)
	}

	// load a sound file into a slot
	s1 := 1
	sampleDirectory := os.Getenv("GOPATH") + "src/github.com/stygian-phrygian/stereophonic/_examples/samples/"
	if err := e.Load(s1, sampleDirectory+"beats.wav"); err != nil {
		log.Fatal(err)
	}

	// prepare a playback event
	startTimeInSeconds := 0.0
	durationInSeconds := 20.0
	event, err := e.Prepare(s1, startTimeInSeconds, durationInSeconds)
	if err != nil {
		log.Fatal(err)
	}
	event.SetLooping(true) // <--- important for this example
	event.SetReverse(true)
	event.Trigger()
	event.SetFilterMode(stereophonic.BPFilter)
	event.SetFilterCutoff(0.0)
	event.SetFilterResonance(0.999)

	// start playback
	e.Play(event)

	// spawn a thread which sweeps the filter
	go func() {
		t := 0.005
		for {
			time.Sleep(200 * time.Millisecond)
			c := math.Abs(math.Sin(2 * math.Pi * t))
			event.SetFilterCutoff(c)
			t += 0.005
		}

	}()

	fmt.Printf("\n\nFilter sweep for %0.2fs\n", durationInSeconds)
	time.Sleep(time.Duration((durationInSeconds * float64(time.Second))))
}
