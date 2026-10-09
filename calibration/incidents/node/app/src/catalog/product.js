'use strict';

// A catalog product.
class Product {
  constructor(sku, price) {
    this.sku = sku;
    this.priceText = price;
  }

  validate() {
    if (!/^[A-Z]{3}-\d+$/.test(this.sku)) {
      throw new Error('bad sku');
    }
  }

  price() {
    const value = BigInt(this.priceText.replace(',', ''));
    return Number(value) / 100;
  }
}

module.exports = { Product };
