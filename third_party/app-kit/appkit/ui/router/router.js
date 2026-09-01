/*!
 * appkit view-router — framework-agnostic client routing for static SPAs.
 *
 * History API navigation, deep-link query params, and document.title sync,
 * with zero dependencies and no build step. Designed for static hosts
 * (e.g. GitHub Pages) where there is no server router: pair it with a
 * `404.html` that is a byte-copy of `index.html` and a `<base href="/">`
 * so hard loads of deep paths resolve assets and boot the app.
 *
 * Canonical source: app-kit  appkit/ui/router/router.js
 * Consumers vendor this file verbatim; do not fork it locally.
 *
 * Usage:
 *   const router = AppKitRouter.create({
 *     routes: [
 *       { id: "home",   path: "/",       title: "Home" },
 *       { id: "resume", path: "/resume", title: "Résumé" },
 *     ],
 *     notFound: "home",
 *     onChange: (state) => render(state),   // state: {id, path, params, hash, matched}
 *   });
 *   router.start();            // read current URL, emit once, listen for back/forward
 *   router.navigate("resume", { params: { role: "fcb" } });  // pushState + emit
 */
(function (global) {
  "use strict";

  function createRouter(config) {
    config = config || {};
    var routes = config.routes || [];
    var base = (config.base || "").replace(/\/+$/, "");
    var fallback = config.notFound || (routes[0] && routes[0].id);
    var listeners = [];
    if (typeof config.onChange === "function") listeners.push(config.onChange);

    function routeById(id) {
      for (var i = 0; i < routes.length; i++) {
        if (routes[i].id === id) return routes[i];
      }
      return null;
    }

    function stripBase(pathname) {
      if (base && pathname.indexOf(base) === 0) {
        return pathname.slice(base.length) || "/";
      }
      return pathname || "/";
    }

    // Match a pathname to a route. Exact match wins; otherwise a single
    // trailing segment under a known parent becomes params.id (e.g.
    // "/resume/fcb" -> route "resume", params.id = "fcb").
    function matchPath(pathname) {
      var p = stripBase(pathname).replace(/\/+$/, "") || "/";
      for (var i = 0; i < routes.length; i++) {
        if (routes[i].path === p) return { route: routes[i], param: null };
      }
      var seg = p.split("/");
      if (seg.length > 2) {
        var parent = "/" + seg[1];
        for (var j = 0; j < routes.length; j++) {
          if (routes[j].path === parent) {
            return { route: routes[j], param: seg.slice(2).join("/") };
          }
        }
      }
      return null;
    }

    function parseQuery(search) {
      var params = {};
      var q = (search || "").replace(/^\?/, "");
      if (!q) return params;
      q.split("&").forEach(function (pair) {
        if (!pair) return;
        var eq = pair.indexOf("=");
        var k = eq < 0 ? pair : pair.slice(0, eq);
        var v = eq < 0 ? "" : pair.slice(eq + 1);
        params[decodeURIComponent(k)] = decodeURIComponent(v.replace(/\+/g, " "));
      });
      return params;
    }

    function currentState() {
      var loc = global.location;
      var m = matchPath(loc.pathname);
      var route = m ? m.route : routeById(fallback);
      var params = parseQuery(loc.search);
      if (m && m.param != null && params.id == null) params.id = m.param;
      return {
        id: route ? route.id : fallback,
        path: route ? route.path : "/",
        params: params,
        hash: loc.hash.replace(/^#/, ""),
        matched: !!m,
      };
    }

    function buildUrl(id, params) {
      var route = routeById(id);
      var path = route ? route.path : "/";
      var url = (base + path) || "/";
      if (params) {
        var qs = Object.keys(params)
          .filter(function (k) { return params[k] != null && params[k] !== ""; })
          .map(function (k) {
            return encodeURIComponent(k) + "=" + encodeURIComponent(params[k]);
          })
          .join("&");
        if (qs) url += "?" + qs;
      }
      return url;
    }

    function applyTitle(state) {
      var t = null;
      if (typeof config.titleFor === "function") t = config.titleFor(state);
      if (!t) {
        var r = routeById(state.id);
        t = r && r.title ? r.title : null;
      }
      if (t) global.document.title = t;
    }

    function emit() {
      var state = currentState();
      applyTitle(state);
      for (var i = 0; i < listeners.length; i++) listeners[i](state);
    }

    // navigate(idOrPath, { params, replace })
    function navigate(idOrPath, opts) {
      opts = opts || {};
      var url;
      if (typeof idOrPath === "string" && idOrPath.charAt(0) === "/") {
        url = (base + idOrPath) || "/";
      } else {
        url = buildUrl(idOrPath, opts.params);
      }
      if (opts.replace) global.history.replaceState({ id: idOrPath }, "", url);
      else global.history.pushState({ id: idOrPath }, "", url);
      emit();
    }

    function onChange(fn) {
      listeners.push(fn);
      return function () {
        var i = listeners.indexOf(fn);
        if (i >= 0) listeners.splice(i, 1);
      };
    }

    function start() {
      global.addEventListener("popstate", emit);
      var init = currentState();
      // Normalize an unknown deep path to the fallback without leaving a
      // dead entry in history.
      if (!init.matched) {
        global.history.replaceState({ id: init.id }, "", buildUrl(init.id, init.params));
      }
      emit();
      return router;
    }

    var router = {
      navigate: navigate,
      onChange: onChange,
      current: currentState,
      url: buildUrl,
      start: start,
      routes: routes,
    };
    return router;
  }

  var api = { create: createRouter };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  global.AppKitRouter = api;
})(typeof window !== "undefined" ? window : this);
