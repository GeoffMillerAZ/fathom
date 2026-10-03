import { secret } from '#internal/secret';
import { featureFn } from '@ws/core/feature';
import { typed } from '#types';

export function coreMain(): number {
  typed();
  featureFn();
  return secret();
}
