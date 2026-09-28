const path = require('path');

module.exports.handler = function handler(event) {
  return path.basename(event);
};

function helper() {
  return module.exports.handler('x');
}

module.exports = { helper };
