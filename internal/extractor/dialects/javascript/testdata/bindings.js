// bindings.js — module-level bindings (O749): a let or var at module scope,
// each name a destructuring one binds, and a CommonJS export assigned a value
// are definitions (JSConst), their initializer's calls their own.
var express = require('express');
var app = express();
let count = 0;
var a = 1, b = make();
var { join, resolve: res } = require('path');
let [first, second] = list();
let pending;
var handler = function () {
  return app.listen(count);
};
exports.version = '1.0.0';
exports.router = express.Router();
module.exports.config = { port: 80 };
exports.create = function create() {
  var local = make();
  let inner = 2;
  return local + inner;
};
exports.wrapped = guard(() => handler());
if (app) {
  var hoisted = 1;
}
for (var i = 0; i < 2; i++) {
  count += i;
}
app.settings = {};
