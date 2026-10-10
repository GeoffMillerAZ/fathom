'use strict';

const log = require('../log').payments.child({ module: 'refunds' });

// Refunds against captured payments.
function requestRefund(paymentId, amount, reason) {
  log.info('refund of %s requested on payment %s: %s', amount, paymentId, reason);
}

function rejectRefund(paymentId, limit) {
  log.warn('refund on payment %s rejected: over the %d cent limit', paymentId, limit);
}

function settleRefund(paymentId, batch) {
  log.debug('refund %s settled late in batch %s', paymentId, batch);
}

function flagRefund(paymentId) {
  const message = 'manual review needed for refund ' + paymentId;
  log.warn(message);
}

function issueCredit(refundId, amount) {
  log.info({ refundId, amount }, 'refund %s issued as store credit', refundId);
}

module.exports = { requestRefund, rejectRefund, settleRefund, flagRefund, issueCredit };
