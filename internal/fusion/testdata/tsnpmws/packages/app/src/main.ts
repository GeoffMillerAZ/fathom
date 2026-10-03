import { coreMain } from '@ws/core';
import { featureFn } from '@ws/core/feature';
import { mode } from '@ws/core/mode';
import { frLocale } from '@ws/core/locales/fr.js';
import { secret } from '@ws/core/internal/secret';
import { typed } from '@ws/types';
import { local } from './local.js';
import { dirFn } from './dir';
import { nope } from 'left-pad';

export function main(): number {
  typed();
  featureFn();
  mode();
  frLocale();
  secret();
  local();
  dirFn();
  nope();
  return coreMain();
}
