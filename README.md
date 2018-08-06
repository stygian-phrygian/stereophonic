# Stereophonic
audio sample player written in go

## What it does
It plays audio samples and let's you tweak them in realtime.

## Prerequisites
installation of portaudio is required prior to usage of this library

## Usage
Check out `_examples/`

## TODO / Ideas

    handle edge case where a very small duration event will round into a 0
    frame event thereby making it unlimited duration accidentally.  If the user
    thinks it's a limited duration event, but it ends up being unlimited accidentally,
    this could create problems, as the done action will never run
    ( think this is solved)

    maybe the adsr envelope should be part of the event itself, as it's a
    special case of envelope.

    this should be viewed as a synthesizer really, where each event/voice
    is just a voice of our overarching synth.  Hence there will only be 1 FX
    bus (as it's just a single synth *wink wink nudge nudge*) that can be sort
    "pretended" into a multitracker

    chorusSend, delaySend, reverbSend, sideChainSend (instead of one FX send)
    the master fx will be laid out thusly:
    chorus -> delay -> reverb -> compressor

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
implement for TablePlayer:
    2 adsr (1 vol & 1 filter cutoff)
    #
    there is a race condition with phaseIncrement in TablePlayer, consider
    that we run SetReverse and SetSpeed simultaneously, there's a possibility
    that phaseIncrement will have different values concurrently, so you could
    reverse phase increment, then it is promptly ignored in SetSpeed
    TODO:? fix the race condition, use a master channel to access the TablePlayer
    TODO:? make a player pool, so we don't stress the GC maybe?
    TODO: stabilize the filter
    TODO: envelope/lfo object? what to call it... waveform?
    Voice manager?  voice instead of playback event
    how to handle releases or indefinite notes
    how to route voices to fx-busses or returns or sends or whatever
    how do tracks work?
    should there be a master fx buss only? or shall we allow more flexibility
    //
    maybe we should mandate the creation of tracks prior to any sample playback
    e.NewTrack("1")
    sends should be an aspect of the Track NOT the playback event/voices
    we can also limit the sends to only fx-busses *not* other tracks to avoid
    weird recursive things
  
    no tracks, just fx busses we can send things to
    en effect bus can have any number of effects on it, and refer to other
    fx busses

    slideFactor should be called speedLag
    
    so a track is really just a chain of effects, which has a summing circuit
    (allowing summing multiple playback events into it)
    how about 1 fx send, each track has a balance, gain, filter, distortion,
    send

    maybe playbackEvent should be called track event (with an added)
    track specified to engine.Prepare, which creates a playbackEvent whose
    tick() method accumulates to a track? and the streamCallback just
    calls tick for every active event, then sums the tracks

    maybe newPlaybackEvent should have an "indefiniteDuration" flag
    we need to handle cases: indefinite/definite in both tick() and Done()
    func tick()
    .. if delayInFrames > 0 {
           delayInFrames -= 1
           return tp.tick()
    }
    if indefiniteDuration {
        // doneAction was passed to the TablePlayer in the constructor
        // the TablePlayer removes itself when we tell it... 
        return tp.tick()
    }
    if durationInFrames > 0 {
        durationInFrames -= 1
        return tp.tick()
    }
    if !ranDoneAction {
        p.doneAction()
    }
    ranDoneAction = true
     
    func Done() {
        // set delayInFrames, edurationInFrames = 0
        // set indefiniteDuration = false
    } 


    so an event with finite duration -> done just sets the internal durationInFrames
    to 0, thereby activating the doneaction (all further ticks return 0)
    an event with indefinite duration however, Done() tells the table player
    gate to turn off 
