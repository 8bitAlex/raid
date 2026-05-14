package telemetry

import "math/rand/v2"

// TaskSampleRate controls how many `raid_task_executed` events fire,
// as a fraction in [0, 1]. Tasks fire on average rate% of the time —
// the issue's "sampled to avoid flooding" requirement. Tests pin this
// to 1 (deterministic capture) or 0 (deterministic drop).
//
// Default 0.1 keeps the per-invocation event volume bounded for
// commands with hundreds of tasks while still giving statistically
// useful samples at fleet scale.
var TaskSampleRate = 0.1

// rngFn returns a uniform [0, 1) sample. Indirected so tests can
// force-deterministic Sampled() behavior without seeding math/rand
// globally (which would race against any other rand consumers).
var rngFn = func() float64 { return rand.Float64() }

// Sampled reports whether a task event should be captured this time
// per TaskSampleRate. Caller fires Capture only when this returns
// true. Fast-paths when telemetry is inactive so opted-out users don't
// pay the per-task RNG call.
func Sampled() bool {
	if !IsActive() {
		return false
	}
	if TaskSampleRate <= 0 {
		return false
	}
	if TaskSampleRate >= 1 {
		return true
	}
	return rngFn() < TaskSampleRate
}
