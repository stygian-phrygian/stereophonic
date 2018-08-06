package stereophonic

import (
	"fmt"
	"math"
)

// adsr envelope
// attack, decay, sustain level, and release all have setters
// to "note on" (or retrigger multiple times) call attack()
// to "note off" (enter the release stage) call release()
// upon creation, the envelope is in the attack stage
// a doneAction callback can also be specified, and which runs
// when the adsr envelope finishes the release stage (when release()
// is called)

// much of the algorithm below is inspired from the article found here:
// http://www.martin-finke.de/blog/articles/audio-plugins-011-envelopes/
// thank you

const (
	adsrOffStage int = iota
	adsrAttackStage
	adsrDecayStage
	adsrSustainStage
	adsrReleaseStage
	//
	adsrNumberOfStages
	//
	adsrMinimumLevel float64 = 0.0001
)

type adsrEnvelope struct {
	// this stores the value of each stage's duration (except the sustain
	// and off stage values, which represent levels).  This is a float64
	// slice because of the sustain level (which must be a float64)
	stage []float64
	// which stage we're in (used for indexing into the stage[] above)
	currentStage int
	// which tick we are at (how far from stage completion that is)
	currentTick int
	// the level of the envelope (obviously)
	currentLevel float64
	// the multiplier to increment/decrement the current level each tick
	multiplier float64
	//
	sampleRate float64
	// the done action callback (called after the release stage finishes)
	doneAction func()
}

// setters
func (adsr *adsrEnvelope) setAttack(attackTimeInSeconds float64) {
	attackTimeInFrames := math.Floor(math.Max(attackTimeInSeconds*adsr.sampleRate, 0.0))
	adsr.stage[adsrAttackStage] = attackTimeInFrames
	// [edge case] if we're in the same stage currently, fix the multiplier
	if adsr.currentStage == adsrAttackStage {
		// calculate the discrepancy of ticks left to compute
		ticksLeft := attackTimeInFrames - float64(adsr.currentTick)
		// update multipler
		adsr.multiplier = calculateLevelMultiplier(adsr.currentLevel, 1.0, ticksLeft)
	}
}
func (adsr *adsrEnvelope) setDecay(decayTimeInSeconds float64) {
	decayTimeInFrames := math.Floor(math.Max(decayTimeInSeconds*adsr.sampleRate, 0.0))
	adsr.stage[adsrDecayStage] = decayTimeInFrames
	// [edge case] if we're in the same stage currently, fix the multiplier
	if adsr.currentStage == adsrDecayStage {
		// calculate the discrepancy of ticks left to compute
		ticksLeft := decayTimeInFrames - float64(adsr.currentTick)
		// update multipler
		adsr.multiplier = calculateLevelMultiplier(adsr.currentLevel, adsr.stage[adsrSustainStage], ticksLeft)
	}
}
func (adsr *adsrEnvelope) setSustain(sustainLevel float64) {
	sl := math.Min(1.0, math.Max(sustainLevel, adsrMinimumLevel))
	adsr.stage[adsrSustainStage] = sl
	// [edge case] if we're in the decay/sustain stages
	switch adsr.currentStage {
	case adsrDecayStage:
		// update multipler (as we're altering the slope of the decay)
		// calculate the discrepancy of ticks left to compute
		ticksLeft := adsr.stage[adsrDecayStage] - float64(adsr.currentTick)
		// update multipler
		adsr.multiplier =
			calculateLevelMultiplier(adsr.currentLevel, sl, ticksLeft)
	case adsrSustainStage:
		// update currentLevel
		adsr.currentLevel = sl
	}
}
func (adsr *adsrEnvelope) setRelease(releaseTimeInSeconds float64) {
	releaseTimeInFrames := math.Floor(math.Max(releaseTimeInSeconds*adsr.sampleRate, 0.0))
	adsr.stage[adsrReleaseStage] = releaseTimeInFrames
	// [edge case] if we're in the same stage currently, fix the multiplier
	if adsr.currentStage == adsrReleaseStage {
		// calculate the discrepancy of ticks left to compute
		ticksLeft := releaseTimeInFrames - float64(adsr.currentTick)
		// update multipler
		adsr.multiplier = calculateLevelMultiplier(adsr.currentLevel, adsrMinimumLevel, ticksLeft)
	}
}

// immediately enter the attack stage from the beginning
// this is also for (re)triggering the adsr envelope
func (adsr *adsrEnvelope) attack() {
	// update current stage, update the multiplier, and reset current tick
	adsr.currentStage = adsrAttackStage
	adsr.currentTick = 0
	adsr.multiplier = calculateLevelMultiplier(
		adsrMinimumLevel,
		1.0,
		adsr.stage[adsrAttackStage])
}

// immediately enter the release stage from the beginning
func (adsr *adsrEnvelope) release() {
	// update current stage, update the multiplier, and reset current tick
	adsr.currentStage = adsrReleaseStage
	adsr.currentTick = 0
	adsr.multiplier = calculateLevelMultiplier(
		adsr.stage[adsrSustainStage],
		adsrMinimumLevel,
		adsr.stage[adsrReleaseStage])
}

// a callback which runs when the release stage finishes
// NB. the release stage is only entered by calling release()
func (adsr *adsrEnvelope) setDoneAction(doneAction func()) {
	adsr.doneAction = doneAction
}

// constructor
func newADSREnvelope(
	attackTimeInSeconds,
	decayTimeInSeconds,
	sustainLevel,
	releaseTimeInSeconds,
	sampleRate float64) (*adsrEnvelope, error) {

	if sampleRate <= 0 {
		return nil, fmt.Errorf("cannot create ADSR envelope with sample rate %d\n", sampleRate)
	}

	// create an adsr object (unspecifed attack/decay/sustain/release, that
	// will be set below)
	adsr := &adsrEnvelope{
		sampleRate:   sampleRate,
		currentLevel: adsrMinimumLevel,
		multiplier:   1.0,
	}
	// create the stage values
	adsr.stage = make([]float64, 5)
	// set the off stage value
	adsr.stage[adsrOffStage] = adsrMinimumLevel
	// set the adsr times
	adsr.setAttack(attackTimeInSeconds)
	adsr.setDecay(decayTimeInSeconds)
	adsr.setSustain(sustainLevel)
	adsr.setRelease(releaseTimeInSeconds)
	// set the done action to nil (can be set later after creation)
	adsr.doneAction = nil
	// now, set the initial stage to attack
	adsr.attack()

	// return it
	return adsr, nil
}

// compute a tick of the envelope generator
func (adsr *adsrEnvelope) tick() float64 {

	// if we're *not* in the off stage or the sustain stage
	if adsr.currentStage != adsrOffStage && adsr.currentStage != adsrSustainStage {
		// if there are ticks left in this stage
		if float64(adsr.currentTick) < adsr.stage[adsr.currentStage] {
			// adjust the current level by multiplier and increment
			// the current tick.  NB. at this point we're only
			// within the attack, decay, release stage)
			adsr.currentLevel *= adsr.multiplier
			adsr.currentTick += 1
		} else {
			// reset the current tick
			adsr.currentTick = 0
			// find which stage is next (given the current)
			switch adsr.currentStage {
			case adsrAttackStage:
				// attack -> decay
				adsr.currentStage = adsrDecayStage
				// update the multiplier (for decay stage)
				adsr.multiplier = calculateLevelMultiplier(
					1.0,
					adsr.stage[adsrSustainStage],
					adsr.stage[adsrDecayStage])
				// NB, when adsr attack time is very small
				// (around 0s) the attack stage does not have
				// sufficient duration to ramp up the peak adsr
				// level (of 1.) hence we just set it to 1 now.
				adsr.currentLevel = 1.0
			case adsrDecayStage:
				// decay -> sustain
				adsr.currentStage = adsrSustainStage
				// update the multiplier (for sustain stage)
				adsr.multiplier = adsr.stage[adsrSustainStage]
			case adsrReleaseStage:
				// release -> off
				adsr.currentStage = adsrOffStage
				// update the multiplier (for off stage)
				adsr.currentLevel = adsrMinimumLevel
				// run the done action
				if adsr.doneAction != nil {
					adsr.doneAction()
				}
			// we shouldn't get here ever
			default:
				//do nothing
			}
		}
	}
	return adsr.currentLevel
}

// calculate the multiplier to increase/decrease
// the current level in an exponential manner
func calculateLevelMultiplier(startLevel, targetLevel, numberOfFrames float64) float64 {
	if numberOfFrames < 1.0 {
		return 1.0 + (math.Log(targetLevel) - math.Log(startLevel))
	} else {
		return 1.0 + (math.Log(targetLevel)-math.Log(startLevel))/numberOfFrames
	}
}
