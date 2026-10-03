import { mode } from '@ws/core/mode';
import localMod = require('./local');

export function legacy(): string {
  localMod.local();
  return mode();
}
