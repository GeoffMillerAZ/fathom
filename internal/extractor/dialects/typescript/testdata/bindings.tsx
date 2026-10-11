// bindings.tsx — module-level bindings (owner ruling S3, O901, LB-F486): a let
// or var at module scope or exported, and each name a destructuring one binds,
// is a definition (JSConst), its initializer's calls its own; one bound to a
// function or a component stays that definition.
import { forwardRefWithAs } from './render';
let count = 0;
var a = 1, b = make();
let { Provider, Consumer: Reader } = createContext(null);
let [first, second] = list();
let pending: number | undefined;
let DEFAULT_TAG = 'button' as const;
function ButtonFn(props: { disabled?: boolean }) {
  return <button disabled={props.disabled} />;
}
export let Button = forwardRefWithAs(ButtonFn) as typeof ButtonFn & { displayName: string };
let Card = () => <div />;
let Fancy = memo(() => null);
var handler = function () {
  return run(count);
};
if (count) {
  var hoisted = 1;
}
for (var i = 0; i < 2; i++) {
  count += i;
}
