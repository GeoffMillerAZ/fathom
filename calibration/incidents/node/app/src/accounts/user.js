'use strict';

// A user account.
class User {
  constructor(name, email) {
    this.name = name;
    this.email = email;
  }

  rename(full) {
    const first = full.match(/^(\S+) /)[1];
    this.name = first;
  }

  domain() {
    const parts = this.email.split('@');
    return parts[1].toLowerCase();
  }
}

module.exports = { User };
