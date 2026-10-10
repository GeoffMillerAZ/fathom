package syslink

// Every consumer of route: keys reads a service's routes composed across
// its files (LB-F353, O695.1): service_ref hops (BuildFromStore through
// RoutesFromRows), capability inventory items (inventory.Extract through
// routes.Compose) and syslink's route: provides rows (routeProvides). One
// cross-file fixture, asserted in each: an Express router mounted from
// another file with app.use('/api', router), and a Nest global prefix set
// in main.ts over a module tree in other files.

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var composedFleet = map[string]map[string]string{
	"shop": {
		"app.js": `const express = require('express')
const users = require('./routes/users')
const app = express()
app.use('/api', users)
app.get('/health', health)
`,
		"routes/users.js": `const router = require('express').Router()
router.get('/users/:id', show)
function show (req, res) {}
module.exports = router
`,
	},
	"cats": {
		"src/main.ts": `import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
const app = await NestFactory.create(AppModule);
app.setGlobalPrefix('v1');
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
		"src/cats/cats.controller.ts": `import { Controller, Get } from '@nestjs/common';
@Controller('cats')
export class CatsController {
  @Get(':id') findOne() {}
}
`,
	},
	"web": {
		"client.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc fetch(id int) {\n\thttp.Get(fmt.Sprintf(\"http://shop/api/users/%d\", id))\n\thttp.Get(fmt.Sprintf(\"http://cats/v1/cats/%d\", id))\n}\n",
	},
}

// fleetFiles returns a service's files sorted by path, as SourceFiles.
func fleetFiles(svc string) []SourceFile {
	var out []SourceFile
	for p, c := range composedFleet[svc] {
		out = append(out, SourceFile{Path: p, Content: []byte(c)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func TestRouteKeys_ComposedInEveryConsumer(t *testing.T) {
	ctx := context.Background()
	// Served at the composed path, by the handler in the mounted file.
	want := map[string]string{
		"shop": "route:GET /api/users/{} show routes/users.js:2;route:GET /health health app.js:5",
		"cats": "route:GET /v1/cats/{} CatsController.findOne src/cats/cats.controller.ts:4",
	}
	local := map[string]string{"shop": "route:GET /users/{}", "cats": "route:GET /cats/{}"}

	// syslink: route: provides rows.
	for svc, w := range want {
		var got []string
		for _, r := range routeProvides(fleetFiles(svc)) {
			if r.Role != RoleProvides || r.NodeKind != "route" || r.TierHint != TierLiteral || r.Extractor != routeExtractorID || r.ExtractorEpoch != routeExtractorEpoch {
				t.Errorf("%s row %+v", svc, r)
			}
			got = append(got, r.Key.String()+" "+r.NodeID+" "+r.File+":"+strconv.Itoa(r.Line))
		}
		sort.Strings(got)
		if strings.Join(got, ";") != w {
			t.Errorf("syslink %s route: provides\n got %s\nwant %s", svc, strings.Join(got, ";"), w)
		}
	}

	// inventory: route: items.
	for svc := range want {
		var files []inventory.File
		for _, f := range fleetFiles(svc) {
			files = append(files, inventory.File{Path: f.Path, Content: f.Content})
		}
		var keys []string
		for _, it := range inventory.Extract(files, nil) {
			if it.Kind != "route" {
				continue
			}
			k, err := ParseContractKey(it.Key)
			if err != nil {
				t.Fatalf("inventory key %q: %v", it.Key, err)
			}
			keys = append(keys, k.String())
		}
		joined := strings.Join(keys, ";")
		for _, w := range strings.Split(want[svc], ";") {
			if !strings.Contains(joined, strings.Fields(w)[0]+" "+strings.Fields(w)[1]) {
				t.Errorf("inventory %s: %s missing from %s", svc, strings.Fields(w)[0]+" "+strings.Fields(w)[1], joined)
			}
		}
		if strings.Contains(joined+";", local[svc]+";") {
			t.Errorf("inventory %s keyed the local path %s: %s", svc, local[svc], joined)
		}
	}

	// serviceref: hops, from the rows an ingest stores.
	st := store.NewMemoryStore()
	var ids []string
	for svc := range composedFleet {
		var all store.ServiceRefs
		var paths []string
		for _, f := range fleetFiles(svc) {
			r := serviceref.ExtractFile(f.Path, f.Content)
			all.Routes = append(all.Routes, r.Routes...)
			all.Calls = append(all.Calls, r.Calls...)
			paths = append(paths, f.Path)
		}
		if err := st.ReplaceServiceRefs(ctx, svc, paths, all, true); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, svc)
	}
	sort.Strings(ids)
	hops, _, err := serviceref.BuildFromStore(ctx, st, ids)
	if err != nil {
		t.Fatal(err)
	}
	var joined []string
	for _, h := range hops {
		joined = append(joined, h.ToService+" "+h.Method+" "+h.Template+" "+h.ToHandler+" "+h.ToFile)
	}
	sort.Strings(joined)
	if w := "cats GET /v1/cats/{} CatsController.findOne src/cats/cats.controller.ts;shop GET /api/users/{} show routes/users.js"; strings.Join(joined, ";") != w {
		t.Errorf("hops\n got %s\nwant %s", strings.Join(joined, ";"), w)
	}

	// The three agree: every hop's key is a provides row of its service.
	for _, h := range hops {
		k, err := ParseContractKey("route:" + h.Method + " " + h.Template)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range routeProvides(fleetFiles(h.ToService)) {
			found = found || r.Key == k && r.NodeID == h.ToHandler && r.File == h.ToFile
		}
		if !found {
			t.Errorf("hop %s %s has no route: provides row", h.ToService, k)
		}
	}
}

// A mount is composed away before any key: none reaches a route: key, and
// a file another file mounts is keyed only at its composed path.
func TestRouteProvides_NoMountKeys(t *testing.T) {
	for svc := range composedFleet {
		for _, r := range routeProvides(fleetFiles(svc)) {
			if r.Key.Body == "" || strings.Contains(r.Key.String(), serviceref.MountMethod) || strings.Contains(r.Key.String(), "./") {
				t.Errorf("%s: a mount reached a route: key: %+v", svc, r)
			}
		}
	}
}
