// util.ts — the SU-11 fixture's dependency module: two module-scope
// functions, consumed both by a named import (index.ts) and by a
// namespace import's member access (wrapper.ts).
export function helper(): number {
	return 1;
}

export function other(): number {
	return 2;
}
