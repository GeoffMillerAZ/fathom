'use strict';

// Money helpers for billing.

function round(cents, parts) {
  if (parts === 0) {
    throw new RangeError('cannot split an amount into zero parts');
  }
  return Math.floor(cents / parts) * parts;
}

function parseAmount(raw) {
  const n = JSON.parse(raw.trim());
  return n.cents;
}

module.exports = { round, parseAmount };
