// wrapper.ts exercises a NAMESPACE cross-file import: wrap -> util.other,
// resolved via member access on the "u" binding.
import * as u from './util';

export function wrap(): number {
	return u.other();
}
