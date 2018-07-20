package main

import (
	"github.com/stygian-phrygian/stereophonic"
	"log"
	"os"
	"time"
)

var (
	BPM                            = 120.0
	quarterNoteDurationInSeconds   = 60.0 / BPM
	eigthNoteDurationInSeconds     = quarterNoteDurationInSeconds * 0.5
	sixteenthNoteDurationInSeconds = eigthNoteDurationInSeconds * 0.5
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

	// find where sample directory is located (relative to GOPATH)
	sampleDirectory := os.Getenv("GOPATH") + "src/github.com/stygian-phrygian/stereophonic/_examples/samples/"

	// load sound files into slots
	kick := 1
	snar := 2
	chat := 3
	ohat := 4
	rims := 5
	tomm := 6
	conm := 7
	if err := e.Load(kick, sampleDirectory+"808kick.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(snar, sampleDirectory+"808snare.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(chat, sampleDirectory+"808closedhat.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(ohat, sampleDirectory+"808openhat.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(rims, sampleDirectory+"808rimshot.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(tomm, sampleDirectory+"808tommid.wav"); err != nil {
		log.Fatal(err)
	}
	if err := e.Load(conm, sampleDirectory+"808congamid.wav"); err != nil {
		log.Fatal(err)
	}

	// spawn goroutines which simulate a beat
	go loop(e, kick, []int{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	go loop(e, snar, []int{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0})
	go loop(e, rims, []int{0, 0, 0, 1, 0})
	go loop(e, chat, []int{1, 0})
	go loop(e, ohat, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0})
	go loop(e, tomm, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	go loop(e, conm, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0})

	// allow events to occur
	time.Sleep(time.Duration(32 * quarterNoteDurationInSeconds * float64(time.Second)))
}

func loop(e *stereophonic.Engine, slot int, steps []int) {
	delayInSeconds := 0.0    // no offset start time
	durationInSeconds := 1.0 // 1s
	i := 0
	for {
		v := steps[i%len(steps)]
		i += 1
		if v != 0 {
			// prepare event
			event, err := e.Prepare(slot, delayInSeconds, durationInSeconds)
			if err != nil {
				log.Fatal(err)
			}

			// on some steps (1, 6, 11, 16...), reverse
			if i%5 == 1 {
				event.SetReverse(true)
				event.Trigger()
			}

			// play event
			e.Play(event)
		}
		// wait period
		time.Sleep(time.Duration(sixteenthNoteDurationInSeconds * float64(time.Second)))
	}
}
