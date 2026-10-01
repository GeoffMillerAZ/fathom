// scope.js — which bindings are definitions (LB-Q-javascript-rest): a
// module-scope binding, an exported declaration, a class method, a
// module-scope prototype or member assignment and a declared function (at any
// depth) are; a function-local arrow const or function expression, a handler
// assigned inside a function, a test-callback helper and an object-literal
// method or pair are not, and a call written in one belongs to the definition
// around it.
const topArrow = (x) => wrap(x);
var topExpr = function () {
  return wrap(1);
};
let topLet = async () => wrap(2);

export const exportedArrow = () => topArrow(1);
export function exportedFn() {
  const localArrow = () => localCall();
  const localExpr = function named() {
    return otherLocal();
  };
  let reassigned;
  reassigned = () => assigned();
  function nestedDecl() {
    return nestedCall();
  }
  return localArrow() + localExpr() + nestedDecl();
}

function Server(opts) {
  this.opts = opts;
  this.handler = function () {
    return onThis();
  };
  const req = {};
  req.onload = function handleLoad() {
    return loaded();
  };
  Server.prototype.late = function () {
    return lateCall();
  };
}

Server.prototype.listen = function listen(port) {
  return bind(port);
};

exports.create = function create() {
  return new Server({});
};

app.get = app.head = function head() {
  return route();
};

class Router {
  route(path) {
    const match = (p) => compare(p, path);
    return match(path);
  }
}

const routes = {
  index() {
    return render();
  },
  show: () => present(),
  edit: function () {
    return change();
  },
};

module.exports = {
  build() {
    return assemble();
  },
  helper: () => help(),
};

describe('server', () => {
  const setup = () => boot();
  function fixture() {
    return load();
  }
  it('listens', function () {
    const check = function () {
      return verify();
    };
    check();
  });
});

if (process.env.DEBUG) {
  const debugOnly = () => trace();
  debugOnly();
}

(function () {
  var hidden = function () {
    return secret();
  };
  hidden();
})();
