package main

import (
	"fmt"
	"github.com/stygian-phrygian/stereophonic"
	"log"
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
	if err := e.Load(s1, sampleDirectory+"707ride.wav"); err != nil {
		log.Fatal(err)
	}

	// prepare a playback event
	startTimeInMs := 0.0    // no offset start time
	durationInMs := 20000.0 // 1s (1000ms) in duration
	downSpeed := 0.5
	upSpeed := 2.0
	slideUp := false
	slideTime := 3.0 // seconds
	event, err := e.Prepare(s1, startTimeInMs, durationInMs)
	if err != nil {
		log.Fatal(err)
	}
	event.SetLooping(true) // <--- important for this example
	event.SetGain(-6)
	event.SetSpeed(downSpeed)
	event.SetReverse(true)
	event.Trigger()

	// start playback
	e.Play(event)

	// spawn a thread which listens to keyboard input
	go func() {
		fmt.Printf("Press the enter/return key... ;)\n")
		for {
			// press "return" key
			fmt.Scanln()

			// slide up/down
			slideUp = !slideUp
			if slideUp {
				event.SetSpeed(upSpeed, slideTime)
				fmt.Printf("sliding up   to speed: %0.1f in %0.1fs\n", upSpeed, slideTime)
			} else {
				event.SetSpeed(downSpeed, slideTime)
				fmt.Printf("sliding down to speed: %0.1f in %0.1fs\n", downSpeed, slideTime)
			}
		}
	}()

	// allow events to occur
	fmt.Printf("\n\nPlay around for %0.2fms\n", durationInMs)
	time.Sleep(time.Duration((durationInMs * float64(time.Millisecond))))
}
