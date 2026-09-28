import def from './default';
import * as ns from './ns';
import { a, b as bee } from '../named';
import './side-effect';
const fs = require('fs');
const { join, resolve } = require('path');

export class Widget extends Base {
  #secret = 1;

  constructor(opts) {
    super(opts);
    this.init();
  }

  render() {
    return helper(this.#secret);
  }

  #hidden() {
    ns.run();
  }

  [Symbol.iterator]() {
    yield* this.items;
  }

  static create() {
    return new Widget({});
  }

  get value() {
    return this._v;
  }
}

function plain(x) {
  return fs.readFileSync(join(x, 'y'));
}

function* gen() {
  yield compute();
}

const arrow = (a, b) => a + add(b);
const expr = function named() {
  return bee();
};
let later;
later = () => 1;

const obj = {
  method() {
    return inner();
  },
  prop: () => other(),
};

export default function () {
  anonymousCall();
}

async function asyncFn() {
  await fetchAll().then(done);
}
