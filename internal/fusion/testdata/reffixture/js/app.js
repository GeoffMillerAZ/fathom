import { Base } from './lib/index.js';
const util = require('./util.js');

export class Widget extends Base {
  render() {
    return util.format;
  }
}
