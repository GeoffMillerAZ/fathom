'use strict';

const util = require('util');
const logger = require('../log').payments;

// The card gateway client: authorizations, captures and declines.
class Gateway {
  constructor(merchant) {
    this.merchant = merchant;
  }

  authorize(amount, card) {
    logger.info({ merchant: this.merchant }, 'authorizing %s cents for merchant %s on card ending %s', amount, this.merchant, card.slice(-4));
    if (amount <= 0) {
      logger.warn('refusing authorization of %d cents for merchant %s', amount, this.merchant);
      return false;
    }
    return true;
  }

  decline(authId, code) {
    logger.error('authorization %s declined by issuer: code %s', authId, code);
  }

  timeout(authId, seconds) {
    logger.error(`gateway timed out after ${seconds}s waiting on authorization ${authId}`);
  }

  voidAuthorization(authId) {
    try {
      throw new Error('connection reset by peer');
    } catch (err) {
      logger.error(err, 'void of authorization %s failed', authId);
    }
  }

  capture(authId, amount) {
    logger.info('captured ' + amount + ' cents for authorization ' + authId);
    return authId;
  }

  chargeback(paymentId, reason) {
    logger.warn(util.format('chargeback opened on payment %s: %s', paymentId, reason));
  }
}

module.exports = { Gateway };
