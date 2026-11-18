import './side.js';
import value, {other} from './named.js';
export {other as result} from './exported.js';
export * from './barrel.js';
export * as ns from './namespace.js';
require('./common.cjs');
require.resolve('./resolved.cjs');
import(`./template.js`);
import(`./${name}`);
require(name);
/* require('./ignored-comment.js') */
const text = "import './ignored-string.js'";
const regex = /require\(['"]\.\/ignored-regex/;
object.require('./ignored-property.js');
const tagged = tag`import('./ignored-template.js') ${import('./nested.js')}`;
