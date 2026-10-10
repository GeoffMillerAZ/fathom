'use strict';

// Audit events, written to the console.

function loginFailed(user, attempts) {
  console.error('login failed for user %s after %d attempts', user, attempts);
}

function roleGranted(user, role, grantedBy) {
  console.info(`role ${role} granted to ${user} by ${grantedBy}`);
}

function exportDenied(user, dataset) {
  console.error('export of ' + dataset + ' denied for ' + user);
}

function passwordReset(user) {
  console.warn('password reset requested for %s', user);
}

function accessDenied(user, resource) {
  console.error(`user ${user} cannot access ${resource}`);
}

function sessionExpired(session) {
  console.log('session %s expired after inactivity', session);
}

module.exports = { loginFailed, roleGranted, exportDenied, passwordReset, accessDenied, sessionExpired };
