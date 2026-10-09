'use strict';

const crypto = require('crypto');

// Authentication: tokens and password hashing.
class Auth {
  constructor(secret) {
    this.secret = secret;
  }

  verify(token) {
    if (!token.endsWith(this.secret)) {
      throw new Error('token rejected');
    }
  }

  hashPassword(password, algorithm) {
    return crypto.createHash(algorithm).update(password).digest('hex');
  }
}

module.exports = { Auth };
