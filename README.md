# Stereophonic
audio sample player written in go

## What it does
It plays audio samples and let's you tweak them in realtime.
It listens for score data on an OSC port / stdin.

## Prerequisites
installation of portaudio is required prior to usage of this library

## Usage
Check out `_examples/`

## TODO / Ideas
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
    in order of increasing difficulty to implement
    balance, attack/decay vol env, glide, distortion, filter

