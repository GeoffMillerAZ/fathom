package routes

import "testing"

func TestReadFastify(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "shorthand methods, route(), options with a handler, inline plugins with prefixes",
			path: "server.js",
			src: `const fastify = require('fastify')({ logger: true })
const users = require('./routes/users')
fastify.get('/', async (request, reply) => ({ hello: 'world' }))
fastify.post('/items', { schema }, createItem)
fastify.put('/items/:id', { handler: updateItem, schema })
fastify.route({ method: ['GET', 'HEAD'], url: '/health', handler: health })
fastify.route({ method: 'delete', path: '/items/:id', handler: async () => {} })
fastify.register(async function (child, opts) {
  child.get('/ping', ping)
  child.register(async (inner) => { inner.all('/deep', deep) }, { prefix: '/v2' })
}, { prefix: '/v1' })
fastify.register(users, { prefix: '/users' })
fastify.register(require('./routes/admin'), { prefix: '/admin' })
fastify.register(import('./routes/dyn.js'))
fastify.register(require('@fastify/cors'))
fastify.listen({ port: 3000 })
`,
			routes: []string{
				"ANY /v1/v2/deep deep", "DELETE /items/:id", "GET /", "GET /health health", "GET /v1/ping ping",
				"HEAD /health health", "POST /items createItem", "PUT /items/:id updateItem",
			},
			mounts: []string{" -> ./routes/dyn", "/admin -> ./routes/admin", "/users -> ./routes/users"},
		},
		{
			name: "a plugin module's instance parameter, typed or named",
			path: "src/plugins/users.ts",
			src: `import { FastifyInstance } from 'fastify'
export default async function users (server: FastifyInstance, opts: object) {
  server.get('/users', listUsers)
  server.get('/users/:id', async (request, reply) => { request.get('/not-a-route', x) })
}
`,
			routes: []string{"GET /users listUsers", "GET /users/:id"},
		},
		{
			name: "autoload mounts its directory with options.prefix; a routes/ plugin reads its folders",
			path: "src/app.ts",
			src: `import path from 'node:path'
import AutoLoad from '@fastify/autoload'
export default async function app (fastify, opts) {
  fastify.register(AutoLoad, { dir: path.join(import.meta.dirname, 'plugins'), options: { ...opts } })
  fastify.register(AutoLoad, { dir: path.join(__dirname, 'routes'), options: { prefix: '/api' } })
}
`,
			mounts: []string{" -> ./plugins/", "/api -> ./routes/"},
		},
		{
			name: "a route plugin under routes/ takes its folders as prefix",
			path: "src/routes/api/tasks/index.ts",
			src: `import { FastifyPluginAsyncTypebox } from '@fastify/type-provider-typebox'
const plugin: FastifyPluginAsyncTypebox = async (fastify) => {
  fastify.get('/', { schema: {} }, async function (request) { return [] })
  fastify.get('/:id', async function (request, reply) {})
}
export default plugin
`,
			routes: []string{"GET /api/tasks", "GET /api/tasks/:id"},
		},
		{
			name: "an exported autoPrefix is added",
			path: "routes/root.js",
			src: `module.exports = async function (fastify, opts) {
  fastify.get('/', async () => 'root')
}
module.exports.autoPrefix = '/x'
`,
			routes: []string{"GET /x"},
		},
		{
			name: "chained shorthand methods; a vendored fastify; a plugin module's routes chained",
			path: "examples/plugin.js",
			src: `const fastify = require('../fastify')({ logger: true })
fastify
  .get('/', opts, function (req, reply) {})
  .post('/', opts, function (req, reply) {})
fastify.register(require('./plugin'), { prefix: '/p' })
`,
			routes: []string{"GET /", "POST /"},
			mounts: []string{"/p -> ./plugin"},
		},
		{
			name: "an Express file and a file without Fastify are not read",
			path: "a.js",
			src: `const express = require('express')
const fastify = require('fastify')()
fastify.get('/x', h)
`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkResult(t, readFastify(c.path, []byte(c.src)), c.routes, c.mounts)
		})
	}
	if got := readFastify("b.js", []byte("app.get('/x', h)\n")); len(got.Routes)+len(got.Mounts) != 0 {
		t.Errorf("a file that never names fastify: %+v", got)
	}
}
