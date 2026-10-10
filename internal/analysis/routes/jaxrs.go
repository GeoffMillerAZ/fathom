package routes

// jaxrs.go — the JAX-RS route reader (LB-F323, wave c2), over the Java
// declaration scanner in spring.go.
//
// A root resource is a type annotated @Path; each of its methods carrying
// @GET, @POST, @PUT, @DELETE, @PATCH, @HEAD or @OPTIONS is a route at the
// type's path joined with the method's own @Path, if any (javax.ws.rs and
// jakarta.ws.rs alike, matched by simple name). A @Path value that names a
// String constant declared in the same file is folded; one naming anything
// else drops the route. Not read, and counted as ceilings by the reading:
// a sub-resource locator (a method with @Path and no HTTP method
// annotation, whose routes live on the type it returns), a type with no
// @Path of its own, and an @ApplicationPath or configured root path. A type
// annotated @RegisterRestClient is a MicroProfile REST client, never a
// server. The handler is the java dialect's name: Class.method.

import "bytes"

func init() {
	Register(Reader{Framework: "jaxrs", Langs: []string{"java"}, Read: readJAXRS})
}

// jaxrsMethods are the HTTP method annotations, by simple name.
var jaxrsMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}

// readJAXRS is the jaxrs reader.
func readJAXRS(path string, src []byte) Result {
	if !bytes.Contains(src, []byte("ws.rs")) || !bytes.Contains(src, []byte("Path")) {
		return Result{}
	}
	jf := scanJava(src)
	var out []Route
	for _, t := range jf.types {
		if t.has("RegisterRestClient") {
			continue
		}
		ta, ok := t.ann("Path")
		if !ok {
			continue
		}
		base, ok := jaxrsPath(ta, jf.consts)
		if !ok {
			continue
		}
		for _, m := range t.methods {
			sub := ""
			if pa, ok := annNamed(m.anns, "Path"); ok {
				if sub, ok = jaxrsPath(pa, jf.consts); !ok {
					continue
				}
			}
			for _, a := range m.anns {
				if !jaxrsMethods[a.name] || len(a.args) > 0 {
					continue
				}
				out = append(out, Route{Method: a.name, Path: jaxrsJoin(base, sub), Handler: t.name + "." + m.name, File: path, Line: a.line})
			}
		}
	}
	return Result{Routes: out}
}

// jaxrsPath is a @Path annotation's single value.
func jaxrsPath(a jann, consts *jconsts) (string, bool) {
	ps, ok := a.paths(consts)
	if !ok || len(ps) != 1 {
		return "", false
	}
	return ps[0], true
}

// jaxrsJoin joins a resource path and a method path as JAX-RS does: a
// leading "/" is implied on each, and exactly one "/" joins them.
func jaxrsJoin(base, sub string) string { return springCombine(base, sub) }
