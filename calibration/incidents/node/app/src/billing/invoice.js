'use strict';

const util = require('./util');

// An invoice: totals, tax and refunds.
class Invoice {
  constructor(lines, currency) {
    if (currency.length !== 3) {
      throw new TypeError('currency must be an ISO code');
    }
    this.lines = lines;
    this.currency = currency;
  }

  total(parts) {
    const sum = this.lines.reduce((a, b) => a + b, 0);
    return util.round(sum, parts);
  }

  applyTax(region) {
    const table = Invoice.taxTable(region);
    return (this.total(1) * table.rate) / 100;
  }

  parseAndAdd(raw) {
    return this.total(1) + util.parseAmount(raw);
  }

  static taxTable(region) {
    return region === 'EU' ? { rate: 20 } : undefined;
  }

  static refund(invoice, amount) {
    const paid = invoice.total(1);
    if (amount > paid) {
      throw new Error('refund exceeds the amount paid');
    }
    return paid - amount;
  }
}

module.exports = { Invoice };
