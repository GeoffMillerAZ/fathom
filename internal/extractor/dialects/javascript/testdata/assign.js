// assign.js — assignment-based definitions the express/axios calibration
// idioms rely on (calibration/corpus.yaml: "prototype-based method
// assignment", "dynamically attached methods on prototypes") and
// universal-ctags already classifies as real definitions (LB-Q-javascript
// measurement gap, design §13 step 21). A non-prototype multi-level member
// chain (module.exports.foo = ...) and a bare identifier assignment (foo =
// ...) are deliberately not exercised here — see tags.scm's comment.
function View(name) {
  this.name = name;
}

View.prototype.render = function render(options) {
  return options;
};

View.prototype.lookup = function (name) {
  return name;
};

const app = {};

app.set = function set(key, val) {
  return val;
};

app.use = (fn) => fn();

exports.helper = function helper(x) {
  return x + 1;
};
