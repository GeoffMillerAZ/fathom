package routes

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// routeLines renders routes as "METHOD path handler" lines, sorted.
func routeLines(rs []Route) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, strings.TrimSpace(r.Method+" "+r.Path+" "+r.Handler))
	}
	sort.Strings(out)
	return out
}

// mountLines renders mounts as "prefix -> target" lines, sorted.
func mountLines(ms []Mount) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, fmt.Sprintf("%s -> %s", m.Prefix, m.Target))
	}
	sort.Strings(out)
	return out
}

func checkResult(t *testing.T, got Result, routes, mounts []string) {
	t.Helper()
	if routes == nil {
		routes = []string{}
	}
	if mounts == nil {
		mounts = []string{}
	}
	if r := routeLines(got.Routes); !reflect.DeepEqual(r, routes) {
		t.Errorf("routes:\n got %q\nwant %q", r, routes)
	}
	if m := mountLines(got.Mounts); !reflect.DeepEqual(m, mounts) {
		t.Errorf("mounts:\n got %q\nwant %q", m, mounts)
	}
}

func TestLexJS(t *testing.T) {
	src := "const a = 'x\\'y' // app.get('/no', h)\n/* app.get('/no2', h) */ const re = /a\\/b[/]c/g; const t = `p/${a}/q`; x = 4 / 2 / 1\n"
	toks := lexJS([]byte(src))
	var got []string
	for _, tk := range toks {
		got = append(got, fmt.Sprintf("%d:%s", tk.kind, strings.ReplaceAll(tk.val, "\x00", "$")))
	}
	want := []string{
		"1:const", "1:a", "6:=", "2:x'y",
		"1:const", "1:re", "6:=", `5:/a\/b[/]c/g`, "6:;",
		"1:const", "1:t", "6:=", "3:p/$/q", "6:;",
		"1:x", "6:=", "4:4", "6:/", "4:2", "6:/", "4:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens:\n got %q\nwant %q", got, want)
	}
	if toks[len(toks)-1].line != 2 {
		t.Errorf("line = %d, want 2", toks[len(toks)-1].line)
	}
}

func TestReadExpress(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "app methods, a use() with no path is not a route, settings are not routes",
			path: "app.js",
			src: `const express = require('express')
const app = express()
app.use(express.json())
app.use(logger)
app.set('view engine', 'ejs')
console.log(app.get('env'))
app.get('/', (req, res) => res.send('hi'))
app.post('/users', auth, createUser)
app.all('/any', anyHandler)
app.delete("/users/:id", function (req, res) {})
// app.get('/commented', nope)
const s = "app.get('/in-a-string', nope)"
`,
			routes: []string{"ANY /any anyHandler", "DELETE /users/:id", "GET /", "POST /users createUser"},
		},
		{
			name: "route chains and arrays of paths",
			path: "r.js",
			src: `import express from 'express'
const router = express.Router()
router.route('/book').get(getBook).post(addBook).put((req, res) => {})
router.get(['/a', '/b'], both)
export default router
`,
			routes: []string{"GET /a both", "GET /b both", "GET /book getBook", "POST /book addBook", "PUT /book"},
		},
		{
			name: "in-file router mounted under a prefix, twice, and a chain of routers",
			path: "server.js",
			src: `var express = require('express');
var app = module.exports = express();
var api = express.Router();
var v1 = express.Router();
api.get('/users', listUsers);
v1.get('/ping', ping);
api.use('/v1', v1);
app.use('/api', api);
app.use('/api2', api);
`,
			routes: []string{
				"GET /api/users listUsers", "GET /api/v1/ping ping",
				"GET /api2/users listUsers", "GET /api2/v1/ping ping",
			},
		},
		{
			name: "mounts of repo modules: require, import, require().router, no path, a router of a mounted router",
			path: "src/app.ts",
			src: `import express, { Router } from 'express'
import users from './routes/users'
import * as admin from '../admin/index.js'
const app = express()
const api = Router()
app.use('/api', api)
api.use('/users', users)
app.use('/admin', requireAuth, admin.router)
app.use(require('./routes'))
app.use('/blog', require('./blog').router)
app.use('/static', express.static('public'))
app.use('/ext', require('some-package'))
`,
			mounts: []string{
				" -> ./routes",
				"/admin -> ../admin/index",
				"/api/users -> ./routes/users",
				"/blog -> ./blog",
			},
		},
		{
			name: "a module called with the app is mounted under the app's prefix",
			path: "index.js",
			src: `const express = require('express')
const app = express()
const r = express.Router()
app.use('/v2', r)
require('./routes/site')(app)
const users = require('./routes/users')
users(r)
`,
			mounts: []string{" -> ./routes/site", "/v2 -> ./routes/users"},
		},
		{
			name: "handler names: module members, destructured requires, wrappers, this, classes",
			path: "routes.js",
			src: `const express = require('express')
const user = require('./user')
const { show: showPost } = require('./post')
const app = express()
class Ctl { static list(req, res) {} }
app.get('/users', user.list)
app.get('/posts/:id', showPost)
app.get('/wrapped', asyncHandler(load))
app.get('/static', Ctl.list)
app.get('/bound', handle.bind(null))
app.get('/obj', handlers.one)
`,
			routes: []string{
				"GET /bound handle", "GET /obj one", "GET /posts/:id show",
				"GET /static Ctl.list", "GET /users list", "GET /wrapped load",
			},
		},
		{
			name: "a parameter named app receives routes",
			path: "routes/index.js",
			src: `module.exports = function (app) {
  app.get('/login', site.login)
  app.post('/logout', function logout (req, res) {})
}
const notAReceiver = (cache) => cache.get('/x', 1)
`,
			routes: []string{"GET /login login", "POST /logout"},
		},
		{
			name: "a file importing another framework is not Express",
			path: "server.ts",
			src: `import Fastify from 'fastify'
const app = Fastify()
app.get('/x', async () => 1)
`,
		},
		{
			name: "client calls and maps are not routes",
			path: "client.js",
			src: `const express = require('express')
axios.get('/api/users', { params })
cache.get('/key', fallback)
const m = new Map(); m.get('/k')
`,
		},
		{
			name:   "string constants and templates fold into paths",
			path:   "consts.js",
			src:    "const express = require('express')\nconst app = express()\nconst BASE = '/api'\nconst V = 'v1'\napp.get(BASE + '/items', items)\napp.get(`${BASE}/${V}/things`, things)\napp.get(`${unknown}/x`, nope)\n",
			routes: []string{"GET /api/items items", "GET /api/v1/things things"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkResult(t, readExpress(c.path, []byte(c.src)), c.routes, c.mounts)
		})
	}
}

func TestReadExpressLines(t *testing.T) {
	src := "const express = require('express')\nconst app = express()\n\napp.get('/a', a)\napp.use('/b', require('./b'))\n"
	got := readExpress("x.js", []byte(src))
	if len(got.Routes) != 1 || got.Routes[0].Line != 4 || got.Routes[0].File != "x.js" {
		t.Fatalf("route = %+v", got.Routes)
	}
	if len(got.Mounts) != 1 || got.Mounts[0].Line != 5 || got.Mounts[0].File != "x.js" {
		t.Fatalf("mount = %+v", got.Mounts)
	}
}
