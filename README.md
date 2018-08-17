# Stereophonic
audio sample player library (written in go)

## What it does
It plays audio samples and let's you tweak them in realtime (reverse, loop,
filter, adsr envelopes, etc).

## Prerequisites
installation of portaudio & libsndfile is required prior to usage of this library

## Usage
Check out `_examples/` but the gist is:

* Create an engine (then tweak/start it)
``` go
	engine, err := stereophonic.New()
	if err != nil {
		log.Fatal(err)
	}
	defer engine.Close()
	
	// engine configuration goes here (before start)
	// one may want to change the output device, sample rate, etc
	
	// start engine
	if err := engine.Start(); err != nil {
		log.Fatal(err)
	}
```
* Load some audio samples (into engine slot indices)
``` go
	// the slot where the sample is loaded into the engine
	slot := 1
	if err := engine.Load(slot, sampleDirectory+"808kick.wav"); err != nil {
		log.Fatal(err)
	}
```
* Prepare events (of either limited or unlimited duration)
``` go
	// this specifies how long to wait before playback starts
	// once the event is actually passed to Play()
	delayInSeconds := 0.0
	// duration is always specified in seconds
	// duration <= 0 will incite an unlimited duration event which must be released
	unlimitedDuration := 0.0
	event, err := engine.Prepare(slot, delayInSeconds, unlimitedDuration)
	if err != nil {
		log.Fatal(err)
	}
	// reverse playback
	event.SetReverse(true)
	// trigger (to update current playback position relative to direction)
	event.Trigger()
	// low pass filter is default
	// cutoff ranges from 0 to 1 * the nyquist frequency (dependent on sample rate)
	event.SetFilterCutoff(0.1)
	// set a note / pitch value (an octave up)
	event.SetNote(12)
```
* Play events (releasing the unlimited duration events as necessary)
``` go
	engine.Play(event)
	time.Sleep(3 * time.Second)
	event.Release()
```
