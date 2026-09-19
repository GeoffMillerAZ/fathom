// Package pkgtwo imports pkgone and exercises a resolved cross-package
// reference (#Widget -> pkgone.#Base), a resolved cross-package reference
// from a PLAIN (non-definition) field (profile -> pkgone.#Base), and one
// deliberately UNRESOLVED cross-package reference (#Ghost -> a name pkgone
// never defines) — SU-10's fixture-pinned edge table.
package pkgtwo

import "example.com/cuefixture/pkgone"

#Widget: pkgone.#Base & {
	z: int
}

profile: pkgone.#Base & {
	q: int
}

#Ghost: pkgone.#Missing & {
	v: int
}
