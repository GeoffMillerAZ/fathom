package command

type invocationMarker interface {
	InvocationErrorKind() string
}

type oneUnwrapper interface {
	Unwrap() error
}

type manyUnwrapper interface {
	Unwrap() []error
}

// InvocationOriginCode classifies a complete composition of recognized
// framework invocation errors as usage exit 2. Mixed, unknown, and ordinary
// application errors remain unclassified so the caller's fallback policy wins.
func InvocationOriginCode(err error) (int, bool) {
	if invocationOnly(err) {
		return 2, true
	}
	return 0, false
}

func invocationOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(manyUnwrapper); ok {
		n := 0
		for _, child := range joined.Unwrap() {
			if child == nil {
				continue
			}
			n++
			if !invocationOnly(child) {
				return false
			}
		}
		return n > 0
	}
	if marker, ok := err.(invocationMarker); ok {
		switch marker.InvocationErrorKind() {
		case "command", "arguments", "required-flags", "flag-group", "flags":
			return true
		default:
			return false
		}
	}
	if wrapped, ok := err.(oneUnwrapper); ok {
		return invocationOnly(wrapped.Unwrap())
	}
	return false
}
