package tenancy

// PressureLevel is the host's memory-pressure level (§2): macOS
// kern.memorystatus_vm_pressure_level reads 1 (normal), 2 (warn) or 4
// (critical). The zero value is Unknown, which is disclosed and never a
// rung trigger.
type PressureLevel uint8

const (
	PressureUnknown  PressureLevel = 0
	PressureNormal   PressureLevel = 1
	PressureWarn     PressureLevel = 2
	PressureCritical PressureLevel = 4
)

func (p PressureLevel) String() string {
	switch p {
	case PressureNormal:
		return "normal"
	case PressureWarn:
		return "warn"
	case PressureCritical:
		return "critical"
	}
	return "unknown"
}
