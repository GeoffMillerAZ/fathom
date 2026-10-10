'use strict';

const util = require('./util');

// A shipment: weight, validation and tracking.
class Shipment {
  constructor(grams, country) {
    this.grams = grams;
    this.country = country;
  }

  packed(step) {
    return util.round(this.grams, step);
  }

  band() {
    const bands = util.bands();
    const i = Math.floor(this.grams / 1000);
    return bands[i + 2].toFixed(0);
  }

  validate() {
    if (!this.country) {
      throw new Error('shipment has no destination');
    }
  }

  async track(carriers) {
    await Promise.resolve();
    const carrier = carriers[this.country];
    return carrier.toUpperCase() + '-' + this.grams;
  }
}

module.exports = { Shipment };
