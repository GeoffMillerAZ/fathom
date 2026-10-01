package tenancy

// ladder.go — the ladder (design §2.1; RT-33, RT-154, RT-171). normal →
// shed → refuse → exit. Entry needs two consecutive over-mark samples (a
// sample more than 3× late counts twice); step-down needs 60 s below 0.6 ×
// the current rung's mark with its host triggers clear, one rung at a time.
// Footprint + children is compared with the shed and refuse marks; the exit
// rung compares the daemon's own footprint. The swap-used fraction is never
// a trigger (macOS grows swap files on demand: 80% used is routine at
// normal pressure). Pure: StepLadder mutates only the state it is given.

import (
	"fmt"
	"time"
)

// Level is a rung.
type Level uint8

const (
	LevelNormal Level = iota
	LevelShed
	LevelRefuse
	LevelExit
	numLevels
)

func (l Level) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelShed:
		return "shed"
	case LevelRefuse:
		return "refuse"
	case LevelExit:
		return "exit"
	}
	return fmt.Sprintf("level(%d)", uint8(l))
}

// Ladder timing (§2.1).
const (
	stepDownAfter    = 60 * time.Second
	stepDownFraction = 0.6
	criticalExitHold = 60 * time.Second
	lateFactor       = 3 // a gap ≥ 3 × SampleEvery counts as two samples
	swapRing         = 256
)

// why codes a trigger without allocating; the reason text is built only on
// a transition.
type why uint8

const (
	whyNone why = iota
	whyShedMark
	whyPressureWarn
	whyRefuseMark
	whyPressureCritical
	whySwapGrowth
	whySwapVolume
	whyExitMark
	whyCriticalSustained
)

// LadderState is the ladder's hysteresis state. The zero value is normal.
type LadderState struct {
	Level Level
	Since time.Time
	// Off is true while the footprint is unreadable: the ladder does not
	// move (disclosed as "ladder: off (footprint unknown)").
	Off bool

	over          [numLevels]int // consecutive over-mark weight per rung
	belowSince    time.Time
	criticalSince time.Time

	swapAt   [swapRing]int64 // unix nanos
	swapUsed [swapRing]int64
	swapN    int
	swapHead int
}

// Transition is one step's outcome. Changed reports anything worth a log
// line: a level change, or the ladder switching off or on.
type Transition struct {
	From, To Level
	At       time.Time
	Changed  bool
	Reason   string
	// Cause is what lifted the ladder on a rise (zero otherwise): the gate
	// names it in a refusal, so a swap-volume floor never reads as a memory
	// budget (EW-37).
	Cause Cause
}

// CauseKind classes what lifted the ladder.
type CauseKind uint8

const (
	// CauseUnknown: no cause published; refusals keep the budget wording.
	CauseUnknown CauseKind = iota
	// CauseMark: a footprint mark (shed, refuse or exit): the memory budget.
	CauseMark
	// CauseHost: a host trigger that is not fathom's own footprint.
	CauseHost
)

// Cause is the ladder's cause: its class and the formatted reason.
type Cause struct {
	Kind   CauseKind
	Detail string
}

// BudgetCause reports whether the refusal should keep the memory-budget
// wording: a footprint mark, or no cause known.
func (c Cause) BudgetCause() bool { return c.Kind == CauseUnknown || c.Kind == CauseMark }

func causeOf(w why, reason string) Cause {
	switch w {
	case whyNone:
		return Cause{}
	case whyShedMark, whyRefuseMark, whyExitMark:
		return Cause{Kind: CauseMark, Detail: reason}
	}
	return Cause{Kind: CauseHost, Detail: reason}
}

// lateEscalates reports whether s ran at least lateFactor cadences after
// the previous sample.
func lateEscalates(s *Sample) bool {
	return SampleEvery+s.Late >= lateFactor*SampleEvery
}

func (st *LadderState) recordSwap(s *Sample) {
	if s.SwapUsed < 0 {
		return
	}
	st.swapAt[st.swapHead] = s.At.UnixNano()
	st.swapUsed[st.swapHead] = s.SwapUsed
	st.swapHead = (st.swapHead + 1) % swapRing
	if st.swapN < swapRing {
		st.swapN++
	}
}

// swapGrowth is s.SwapUsed minus the least swap used within window.
func (st *LadderState) swapGrowth(s *Sample, window time.Duration) int64 {
	if s.SwapUsed < 0 || st.swapN == 0 {
		return 0
	}
	from := s.At.Add(-window).UnixNano()
	low := s.SwapUsed
	for i := 0; i < st.swapN; i++ {
		if st.swapAt[i] >= from && st.swapUsed[i] < low {
			low = st.swapUsed[i]
		}
	}
	return s.SwapUsed - low
}

func pressureAtLeast(p, trigger PressureLevel) bool {
	return trigger != PressureUnknown && p != PressureUnknown && p >= trigger
}

// StepLadder advances the ladder by one sample. Pure over (st, s, b).
func StepLadder(st *LadderState, s *Sample, b *Budget) Transition {
	from := st.Level
	if s.Footprint < 0 {
		wasOff := st.Off
		st.Off = true
		st.over = [numLevels]int{}
		st.belowSince, st.criticalSince = time.Time{}, time.Time{}
		if from == LevelNormal && wasOff {
			return Transition{From: from, To: from, At: s.At}
		}
		st.Level, st.Since = LevelNormal, s.At
		return Transition{From: from, To: LevelNormal, At: s.At, Changed: true, Reason: "ladder: off (footprint unknown)"}
	}
	resumed := st.Off
	st.Off = false

	children := s.Children
	if children < 0 {
		children = 0
	}
	combined := s.Footprint + children
	weight := 1
	if lateEscalates(s) {
		weight = 2
	}
	st.recordSwap(s)
	if s.Pressure == PressureCritical {
		if st.criticalSince.IsZero() {
			st.criticalSince = s.At
		}
	} else {
		st.criticalSince = time.Time{}
	}

	// Triggers, highest first within each rung.
	shed := whyNone
	switch {
	case combined >= b.Shed:
		shed = whyShedMark
	case pressureAtLeast(s.Pressure, b.Host.ShedOnPressure):
		shed = whyPressureWarn
	}
	refuse := whyNone
	switch {
	case combined >= b.Refuse:
		refuse = whyRefuseMark
	case pressureAtLeast(s.Pressure, b.Host.RefuseOnPressure):
		refuse = whyPressureCritical
	case combined >= b.Shed && st.swapGrowth(s, b.Host.SwapGrowthWindow) > b.Host.SwapGrowth:
		refuse = whySwapGrowth
	case s.SwapVolumeFree >= 0 && s.SwapVolumeFree < b.Host.MinSwapVolumeFree:
		refuse = whySwapVolume
	}
	exit := whyNone
	switch {
	case s.Footprint >= b.Exit:
		exit = whyExitMark
	case !st.criticalSince.IsZero() && s.At.Sub(st.criticalSince) >= criticalExitHold && s.Footprint > b.Budget:
		exit = whyCriticalSustained
	}
	bump := func(l Level, hit bool) {
		if hit {
			st.over[l] += weight
		} else {
			st.over[l] = 0
		}
	}
	bump(LevelShed, shed != whyNone || refuse != whyNone)
	bump(LevelRefuse, refuse != whyNone)
	bump(LevelExit, exit == whyExitMark)

	target, cause := LevelNormal, whyNone
	if st.over[LevelShed] >= 2 {
		target, cause = LevelShed, shed
		if cause == whyNone {
			cause = refuse
		}
	}
	if st.over[LevelRefuse] >= 2 {
		target, cause = LevelRefuse, refuse
	}
	if b.ExitEnabled && (st.over[LevelExit] >= 2 || exit == whyCriticalSustained) {
		target, cause = LevelExit, exit
	}

	switch {
	case target > from:
		st.Level, st.Since, st.belowSince = target, s.At, time.Time{}
		reason := reasonFor(cause, s, b, combined, st, weight == 2)
		return Transition{From: from, To: target, At: s.At, Changed: true,
			Reason: reason, Cause: causeOf(cause, reason)}
	case from == LevelExit && target < LevelExit:
		// The exit rung is an action (the daemon exits, or halts an ingest
		// and lets the next sample decide): not sustained, it holds refuse
		// and steps down from there under the usual hysteresis.
		st.Level, st.Since, st.belowSince = LevelRefuse, s.At, time.Time{}
		return Transition{From: from, To: LevelRefuse, At: s.At, Changed: true,
			Reason: fmt.Sprintf("exit mark not sustained: own footprint %s < %s", FormatBytes(s.Footprint), FormatBytes(b.Exit))}
	case from > LevelNormal && from < LevelExit:
		mark, hostActive := b.Shed, shed == whyPressureWarn
		if from == LevelRefuse {
			mark = b.Refuse
			hostActive = refuse == whyPressureCritical || refuse == whySwapGrowth || refuse == whySwapVolume
		}
		if float64(combined) < stepDownFraction*float64(mark) && !hostActive {
			if st.belowSince.IsZero() {
				st.belowSince = s.At
			}
			if s.At.Sub(st.belowSince) >= stepDownAfter {
				to := from - 1
				st.Level, st.Since, st.belowSince = to, s.At, time.Time{}
				st.over = [numLevels]int{}
				return Transition{From: from, To: to, At: s.At, Changed: true,
					Reason: fmt.Sprintf("60 s below 0.6 × the %s mark (%s < %s)", from, FormatBytes(combined), FormatBytes(int64(stepDownFraction*float64(mark))))}
			}
		} else {
			st.belowSince = time.Time{}
		}
	}
	if resumed {
		return Transition{From: from, To: st.Level, At: s.At, Changed: true, Reason: "ladder: on (footprint readable again)"}
	}
	return Transition{From: from, To: st.Level, At: s.At}
}

func reasonFor(w why, s *Sample, b *Budget, combined int64, st *LadderState, late bool) string {
	var r string
	switch w {
	case whyShedMark:
		r = fmt.Sprintf("footprint+children %s ≥ shed mark %s", FormatBytes(combined), FormatBytes(b.Shed))
	case whyPressureWarn:
		r = "host memory pressure " + s.Pressure.String()
	case whyRefuseMark:
		r = fmt.Sprintf("footprint+children %s ≥ refuse mark %s", FormatBytes(combined), FormatBytes(b.Refuse))
	case whyPressureCritical:
		r = "host memory pressure critical"
	case whySwapGrowth:
		r = fmt.Sprintf("swap grew %s in %s while fathom ≥ the shed mark", FormatBytes(st.swapGrowth(s, b.Host.SwapGrowthWindow)), b.Host.SwapGrowthWindow)
	case whySwapVolume:
		r = fmt.Sprintf("swap volume free %s < %s", FormatBytes(s.SwapVolumeFree), FormatBytes(b.Host.MinSwapVolumeFree))
	case whyExitMark:
		r = fmt.Sprintf("own footprint %s ≥ exit mark %s", FormatBytes(s.Footprint), FormatBytes(b.Exit))
	case whyCriticalSustained:
		r = fmt.Sprintf("host pressure critical for 60 s with footprint %s > budget %s", FormatBytes(s.Footprint), FormatBytes(b.Budget))
	default:
		r = "over mark"
	}
	if s.Children > 0 && w != whyExitMark {
		r += fmt.Sprintf(" (children %s)", FormatBytes(s.Children))
	}
	if late {
		r += fmt.Sprintf("; sample %s late, counted twice", s.Late.Round(time.Millisecond))
	}
	return r
}
