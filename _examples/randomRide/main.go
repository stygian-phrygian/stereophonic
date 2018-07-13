package main

import (
	"fmt"
	"github.com/stygian-phrygian/stereophonic"
	"log"
	"math/rand"
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

	// spawn a thread which listens to
	// keyboard input, playing sounds on keypresses
	go func() {
		fmt.Printf("Press the enter/return key... ;)\n")
		for {
			// press "return" key
			fmt.Scanln()

			// prepare playback
			delayInMs := 0.0       // no offset start time
			durationInMs := 2000.0 // 1s (1000ms) in duration
			event, err := e.Prepare(s1, delayInMs, durationInMs)
			if err != nil {
				log.Fatal(err)
			}

			// get random numbers
			r1 := rand.Float64()
			r2 := rand.Float64()
			r3 := rand.Float64()
			r4 := rand.Float64()

			// randomly set speed
			event.SetSpeed(r1 + 0.5)
			fmt.Printf("speed: %.3f, ", r1)

			// randomly loop sometimes
			if r2 > 0.5 {
				event.SetLoopSlice(r1, 1.0)
				event.SetLooping(true)
				fmt.Printf("looping: true and loopslice: (%.3f, %.3f), ", r1, 1.0)
			}
			// randomly reverse sometimes
			if r3 > 0.5 {
				event.SetReverse(true)
				event.Trigger() // <--- necessary to fix the phase, otherwise phase won't be at the end of the table
				fmt.Printf("reverse: true, ")
			}
			// randomly balance the signal
			if r4 > 0.5 {
				event.SetBalance(1.0)
				fmt.Printf("balance: right, ")
			} else {
				event.SetBalance(-1.0)
				fmt.Printf("balance: left, ")
			}
			// play it
			fmt.Printf("\n")
			e.Play(event)
		}
	}()

	// allow events to occur
	fmt.Print("\n\nPlay around for 30s\n")
	time.Sleep(30 * time.Second)
}
