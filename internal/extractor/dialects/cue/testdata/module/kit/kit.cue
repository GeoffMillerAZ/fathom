// kit.cue exercises where CUE declares fields (conjunction, disjunction and
// default operands, list elements and element types, comprehension sources
// and bodies, lets, aliases, patterns, deep nesting) and how it refers to them.
package kit

import (
	"strings"
	b "example.com/fix/base"
)

#Kit: {
	b.#Base
	name: string
	ports: [...{port: int, proto: *"tcp" | "udp"}]
	labels: [string]: {value: string, note?: string}
	...
}

web: #Kit & {
	name: strings.ToLower("WEB")
	ports: [{port: 80}]
	extra: {deep: {deeper: {deepest: {level5: {level6: {level7: 1}}}}}}
}

for k, v in {a: 1, b: 2} {
	"gen_\(k)": {val: v}
}

if web.name != "" {
	cond: web.ports[0].port
}

let L = {z: 1, w: {y: 2}}
usesLet: L.w.y

alt: *{d1: 1} | {d2: b.#Base.id}

X=aliased: {inner: X.other, other: 1}

sel:   web.extra.deep
other: shared
