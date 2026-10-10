'use strict';

// An order and its lines.
class Order {
  constructor(stock) {
    this.quantities = [];
    this.stock = stock;
  }

  addLine(qty) {
    if (qty <= 0) {
      throw new RangeError('quantity must be positive');
    }
    this.quantities.push(qty);
  }

  reserve(qty) {
    if (qty > this.stock) {
      throw new Error('not enough stock');
    }
    this.stock -= qty;
  }

  summarize(divisor) {
    return this.quantities.map(function lineShare(q) {
      if (divisor === 0) {
        throw new RangeError('divisor must be positive');
      }
      return q / divisor;
    });
  }

  deep(n) {
    return this.level1(n);
  }

  level1(n) {
    return this.level2(n) + 1;
  }

  level2(n) {
    return this.level3(n) + 1;
  }

  level3(n) {
    this.reserve(n);
    return this.stock;
  }
}

module.exports = { Order };
