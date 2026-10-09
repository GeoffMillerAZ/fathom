'use strict';

// Weight helpers for shipping: the same basename as billing's util.

function round(grams, step) {
  if (step === 0) {
    throw new RangeError('step must be positive');
  }
  return Math.floor(grams / step) * step;
}

function bands() {
  return [500, 1000, 2000];
}

module.exports = { round, bands };
