// index.ts exercises a NAMED cross-file import (run -> util.helper) and a
// same-file module-scope call (run -> local).
import { helper } from './util';

export function run(): number {
	return helper() + local();
}

function local(): number {
	return 3;
}
