# Stereophonic
audio sample player written in go

## What it does
It plays audio samples and let's you tweak them in realtime.

## Prerequisites
installation of portaudio is required prior to usage of this library

## Usage
Check out `_examples/`

## TODO / Ideas

    1. recording audio / resampling
    2. multiple track / effect bus
    3. master effects (eq filter & side chain compressor?)

    // we should have different streamCallbacks for each possible scenario
    output (no input)
    output (mono input)
    output (stereo input)
    as you can't suddenly get input when the stream is started, you have to
    stop the engine, adjust accordingly (SetDevice, SetInputChannels
    this might be a microoptimization however.


    // delay send (with ping pong)
    e.SetDelayTimeLeft

    // reverb send would be nice...

    // master compressor (with sidechain)
    //
    // turn the compressor on/off
    e.SetCompressorOn()
    // turn sidechain on/off (if on, event sidechain sends duck master)
    e.SetCompressorSideChainOn() 
    // standard compressor commands 
    e.SetCompressorAttack()
    e.SetCompressorRelease()
    e.SetCompressorThreshold()
    e.SetCompressorRatio()

    //master filter
    //
    e.SetFilterMode()
    e.SetFilterCutoff()
    e.SetFilterResonance()

    // master gain
    e.SetGain()


    
sendSideChainAmplitude
sendDelayAmplitude


    PROBLEM: updating the engine's sample rate MUST also update the FX chain
    (as this would alter the way a delay works.  Effect Send.  Effects Bus (bus
    is the correct term, not buss)


    what about: engine.Prepare() automatically sends everything to channel 0
    (first track) which is (also automatically) wired to be the ducking track
    for the master compressor to read from?

    make events abstract, so not a playback event struct, just an interface
    which has like a tick()/pull() method or something, then we can have playback
    events AND maybe like an input event (so sampling) which can pull in audio
    from the audio input (so sampling).  So each of it's calls to tick just
    pulls in another frame maybe (or a buffer of frames to be quicker and work
    with the system more)

    this should be viewed as a synthesizer really, where each event/voice
    is just a voice of our overarching synth.  Hence there will only be 1 FX
    bus (as it's just a single synth *wink wink nudge nudge*) that can be sort
    "pretended" into a multitracker
