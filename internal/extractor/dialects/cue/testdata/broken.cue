// broken.cue is a mid-edit save: a malformed import, an unclosed struct.
package broken

import 42

good: "value"

#Open: {
	name: string
