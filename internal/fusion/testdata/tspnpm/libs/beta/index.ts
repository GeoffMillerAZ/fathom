import { alpha } from 'alpha';
import { util } from 'alpha/src/util';
import { other } from 'beta/other';

export function beta(): string {
  return alpha() + util() + other();
}
