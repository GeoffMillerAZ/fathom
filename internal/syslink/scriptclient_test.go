package syslink

import "testing"

// TestScriptClientConsumes is SG-6's contract: JS/TS (fetch, axios, ky)
// and Python (requests, httpx, aiohttp) client calls, read through the
// embedded selector file, become route: consumes rows at literal or
// template tier; a call inside a comment or with an unresolvable URL
// produces nothing.
func TestScriptClientConsumes(t *testing.T) {
	ts := []byte(`import axios from "axios";
import ky from "ky";

const api = process.env.API_URL;
const orders = axios.create({ baseURL: api });

export async function loadOrder(id: string) {
  const r = await fetch(` + "`${api}/orders/${id}`" + `);
  // fetch("/commented/out")
  /* axios.get("/also/commented") */
  await fetch(api + "/orders", { method: "post", body: "{}" });
  await axios.delete("https://orders.example/orders/" + id);
  await ky.get(` + "`${api}/${kind}/list`" + `);
  await orders.put("/orders/" + id);
  await fetch(url);
  const s = "fetch('/in/a/string')";
}
`)
	checkRows(t, "ts", scriptClientConsumes("typescript", "web/api.ts", ts), []rowWant{
		{"route:GET /orders/{}", "consumes", "loadOrder", TierLiteral, 8},
		{"route:POST /orders", "consumes", "loadOrder", TierLiteral, 11},
		{"route:DELETE /orders/{}", "consumes", "loadOrder", TierLiteral, 12},
		{"route:GET /{}/list", "consumes", "loadOrder", TierTemplate, 13},
		{"route:PUT /orders/{}", "consumes", "loadOrder", TierLiteral, 14},
	})

	py := []byte(`import requests, httpx, aiohttp

BASE = "http://orders.example"

def get_order(order_id):
    # requests.get("/commented")
    return requests.get(f"{BASE}/orders/{order_id}")

def create(payload):
    httpx.post(BASE + "/orders", json=payload)

async def by_kind(kind):
    async with aiohttp.ClientSession() as session:
        async with session.get(f"/{kind}/list") as r:
            return await r.json()

def unresolved(u):
    return requests.get(u)
`)
	checkRows(t, "py", scriptClientConsumes("python", "svc/client.py", py), []rowWant{
		{"route:GET /orders/{}", "consumes", "get_order", TierLiteral, 7},
		{"route:POST /orders", "consumes", "create", TierLiteral, 10},
		{"route:GET /{}/list", "consumes", "by_kind", TierTemplate, 14},
	})

	if got := scriptClientConsumes("ruby", "x.rb", py); got != nil {
		t.Errorf("unsupported dialect produced rows: %v", got)
	}
}
