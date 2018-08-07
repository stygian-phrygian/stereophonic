# Stereophonic
audio sample player written in go

## What it does
It plays audio samples and let's you tweak them in realtime.

## Prerequisites
installation of portaudio is required prior to usage of this library

## Usage
Check out `_examples/`

## TODO / Ideas


    1. master multi-tap delay fx send / tracks
    2. resampling
    3. sidechain compressor?


    make events abstract, so not a playback event struct, just an interface
    which has like a tick()/pull() method or something, then we can have playback
    events AND maybe like an input event (so sampling) which can pull in audio
    from the audio input (so sampling).  So each of it's calls to tick just
    pulls in another frame maybe (or a buffer of frames to be quicker and work
    with the system more)

    rewrite playbackEvent.go (too complicated)?

    this should be viewed as a synthesizer really, where each event/voice
    is just a voice of our overarching synth.  Hence there will only be 1 FX
    bus (as it's just a single synth *wink wink nudge nudge*) that can be sort
    "pretended" into a multitracker

    make N tracks (in the beginning) could be an array, or even a hashmap
    afterwards, each playback event/voice has a track it writes its tick()
    to... maybe tick() returns 4 values, the wet and dry stereo signals?

    no master fx send per playback event, instead, there will be a master fx
    channel but each playback event specifies 
mixer:
    each engine HAS A mixer inside of it, from which playback events
    are sent to various TRACKS
    a mixer is a collection of tracks, and sends
    each track only handles 1 voice at a time.  To simulate polyphony
    you can control multiple tracks with one method simultaneously.
    track methods/setters: gain, pan, filter, fx-send-dry/wet
    the sends take some audio frame input and output there own (processed)
    effects, probably I'll only implement delay (as other things are
    too costly to compute (glares at you, reverb), and I want this to be minimal)
    
    maybe playbackEvent should be called track event (with an added)
    track specified to engine.Prepare, which creates a playbackEvent whose
    tick() method accumulates to a track? and the streamCallback just
    calls tick for every active event, then sums the tracks
