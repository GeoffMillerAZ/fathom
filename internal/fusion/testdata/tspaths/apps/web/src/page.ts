import { util } from '@shared/util';
import { thing } from '@local/thing';

export function page(): string {
  return util() + thing();
}
