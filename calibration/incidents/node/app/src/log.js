'use strict';

// The service's loggers: pino for payments, winston for fulfillment (audit
// writes to the console). Each logger renders its line and hands it to
// sink.line; whoever starts the service points the sink at its output.

const pino = require('pino');
const winston = require('winston');

const sink = {
  line: (text) => process.stdout.write(`${text}\n`),
};

const payments = pino(
  { level: 'debug', base: { service: 'shop' }, timestamp: pino.stdTimeFunctions.isoTime },
  { write: (s) => sink.line(s.trimEnd(), 'node.pino') },
);

const MESSAGE = Symbol.for('message');
const emit = winston.format((info) => {
  sink.line(info[MESSAGE], 'node.winston');
  return info;
});

const fulfillment = winston.createLogger({
  level: 'debug',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.splat(),
    winston.format.printf((i) => `${i.timestamp} ${i.level}: ${i.message}`),
    emit(),
  ),
  transports: [new winston.transports.Console({ silent: true })],
});

module.exports = { sink, payments, fulfillment };
