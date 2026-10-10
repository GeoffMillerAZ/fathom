'use strict';

const logger = require('../log').fulfillment;

// Warehouse picking and stock checks.
class Warehouse {
  constructor(site) {
    this.site = site;
  }

  pick(sku, bin) {
    logger.info('picked %s from bin %s at %s', sku, bin, this.site);
    return sku;
  }

  shortPick(sku, wanted, found) {
    logger.warn('short pick on %s: wanted %d, found %d in the bin', sku, wanted, found);
  }

  restock(sku, qty) {
    logger.info(`restocked ${sku} with ${qty} units after a cycle count`);
  }

  quarantine(lot) {
    logger.log('warn', 'lot %s quarantined at the receiving dock', lot);
  }

  recount(sku) {
    logger.debug('recount scheduled for %s at site %s', sku, this.site);
  }

  cycleCountFailed(sku, attempts) {
    logger.error('cycle count failed for %s after %d attempts', sku, attempts);
  }
}

module.exports = { Warehouse };
