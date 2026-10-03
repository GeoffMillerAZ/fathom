import { coreFn } from '@app/core';
import { add } from '@app/core/math';
import { special } from '@app/special';
import { coreFn as fb } from '@fallback/core';
import { leftPad } from 'left-pad';

export function main(): number {
  special();
  leftPad();
  fb();
  return coreFn() + add(1, 2);
}
