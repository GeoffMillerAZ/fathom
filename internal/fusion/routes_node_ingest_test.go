package fusion_test

// Wave c2 item node (LB-F326..LB-F329): an ingest stores what the Node
// framework readers read — Express, Fastify and Nest routes and their
// mounts, Next.js file routes — and RoutesFromRows composes the mounts,
// so a service's routes are the paths it serves and a client call joins
// the handler behind a mount.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// nodeServices are four small services, one per framework reader.
var nodeServices = map[string]map[string]string{
	"shop-express": {
		"app.js": `const express = require('express')
const users = require('./routes/users')
const app = module.exports = express()
const admin = express.Router()
admin.get('/stats', stats)
app.use('/admin', admin)
app.use('/api/users', users)
app.use(express.static('public'))
app.get('/health', (req, res) => res.send('ok'))
`,
		"routes/users.js": `const router = require('express').Router()
router.get('/:id', show)
router.route('/').get(list).post(create)
function show (req, res) {}
function list (req, res) {}
function create (req, res) {}
module.exports = router
`,
	},
	"shop-fastify": {
		"src/server.ts": `import Fastify from 'fastify'
import orders from './orders.js'
const app = Fastify()
app.register(orders, { prefix: '/orders' })
app.register(async (v2) => { v2.get('/ping', ping) }, { prefix: '/v2' })
`,
		"src/orders.ts": `import { FastifyInstance } from 'fastify'
export default async function orders (fastify: FastifyInstance) {
  fastify.get('/:id', getOrder)
  fastify.route({ method: 'POST', url: '/', handler: createOrder })
}
`,
	},
	"shop-nest": {
		"src/main.ts": `import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
const app = await NestFactory.create(AppModule);
app.setGlobalPrefix('api');
await app.listen(3000);
`,
		"src/app.module.ts": `import { Module } from '@nestjs/common';
import { CatsModule } from './cats/cats.module';
@Module({ imports: [CatsModule] })
export class AppModule {}
`,
		"src/cats/cats.module.ts": `import { Module } from '@nestjs/common';
import { CatsController } from './cats.controller';
@Module({ controllers: [CatsController] })
export class CatsModule {}
`,
		"src/cats/cats.controller.ts": `import { Controller, Get, Post } from '@nestjs/common';
@Controller('cats')
export class CatsController {
  @Get(':id') findOne() {}
  @Post() create() {}
}
`,
	},
	"shop-next": {
		"app/(shop)/products/[id]/page.tsx": "export default function Product() { return <main/> }\n",
		"app/api/cart/route.ts":             "export async function GET() {}\nexport async function POST() {}\n",
		"pages/api/legacy.ts":               "export default function handler(req, res) {}\n",
		"app/ui/button.tsx":                 "export default function Button() { return <button/> }\n",
	},
	"client": {
		"cli.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc fetch(id int) {\n\thttp.Get(fmt.Sprintf(\"http://shop/api/users/%d\", id))\n\thttp.Get(fmt.Sprintf(\"http://shop/api/cats/%d\", id))\n}\n",
	},
}

func TestIngest_NodeFrameworkRoutes(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })
	ids := make([]string, 0, len(nodeServices))
	for id, files := range nodeServices {
		root := t.TempDir()
		for rel, data := range files {
			full := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := p.Ingest(ctx, core.ServiceRef{ID: id, Path: root}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	want := map[string]string{
		// Compose joins a mount to a router's "/" as "/api/users/"; the
		// route: key's NormalizeTemplate drops the trailing slash.
		"shop-express": "GET /admin/stats stats;GET /api/users/ list;GET /api/users/:id show;GET /health;POST /api/users/ create",
		"shop-fastify": "GET /orders/:id getOrder;GET /v2/ping ping;POST /orders/ createOrder",
		"shop-nest":    "GET /api/cats/:id CatsController.findOne;POST /api/cats CatsController.create",
		"shop-next":    "ANY /api/legacy handler;GET /api/cart GET;GET /products/{} Product;POST /api/cart POST",
	}
	for id, w := range want {
		refs, complete, err := st.ServiceRefsFor(ctx, id)
		if err != nil || !complete {
			t.Fatalf("%s: complete=%v err=%v", id, complete, err)
		}
		var got []string
		for _, r := range serviceref.RoutesFromRows(refs.Routes) {
			if r.Method == serviceref.MountMethod {
				t.Fatalf("%s: a mount row leaked: %+v", id, r)
			}
			got = append(got, strings.TrimSpace(r.Method+" "+r.Path+" "+r.Handler))
		}
		sort.Strings(got)
		if strings.Join(got, ";") != w {
			t.Errorf("%s routes:\n got %s\nwant %s", id, strings.Join(got, ";"), w)
		}
	}

	// A client call joins the handler behind an Express mount and a Nest
	// global prefix.
	hops, _, err := serviceref.BuildFromStore(ctx, st, ids)
	if err != nil {
		t.Fatal(err)
	}
	var joined []string
	for _, h := range hops {
		joined = append(joined, h.ToService+" "+h.Method+" "+h.Template+" "+h.ToHandler)
	}
	sort.Strings(joined)
	if w := "shop-express GET /api/users/{} show;shop-nest GET /api/cats/{} CatsController.findOne"; strings.Join(joined, ";") != w {
		t.Errorf("hops:\n got %s\nwant %s", strings.Join(joined, ";"), w)
	}
}
