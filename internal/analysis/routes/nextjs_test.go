package routes

import "testing"

func TestReadNext(t *testing.T) {
	page := "import Link from 'next/link'\nexport default function Home() { return <main/> }\n"
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
	}{
		{"app root page", "app/page.tsx", page, []string{"GET / Home"}},
		{"src/app nested page", "src/app/blog/page.tsx", page, []string{"GET /blog Home"}},
		{"route groups and slots add no segment", "app/(shop)/@modal/products/page.tsx", page, []string{"GET /products Home"}},
		{"dynamic, catch-all and optional catch-all segments", "app/docs/[id]/[...slug]/[[...rest]]/page.jsx", page, []string{"GET /docs/{}/{}/{} Home"}},
		{"an intercepting route is not a route", "app/feed/(..)photo/[id]/page.tsx", page, nil},
		{"a private folder is not a route", "app/_lib/page.tsx", page, nil},
		{"layouts are not routes", "app/blog/layout.tsx", page, nil},
		{
			"route handlers: one route per exported method",
			"app/api/items/[id]/route.ts",
			`import { NextResponse } from 'next/server'
export async function GET(request: Request) { return NextResponse.json([]) }
export const POST = async (req: Request) => new Response()
function handler() {}
export { handler as DELETE, handler as PUT }
export const dynamic = 'force-dynamic'
`,
			[]string{"DELETE /api/items/{} handler", "GET /api/items/{} GET", "POST /api/items/{} POST", "PUT /api/items/{} handler"},
		},
		{"a route.ts with no method export is not a route handler", "app/routes/route.tsx", "export default function R() { return <div/> }\nexport function loader() {}\n", nil},
		{"pages index", "pages/index.tsx", "const Home: NextPage = () => <div/>;\nexport default Home;\n", []string{"GET / Home"}},
		{"pages nested index and dynamic", "src/frontend/pages/product/[productId]/index.tsx", "const ProductDetail = () => <p/>\nexport default ProductDetail\n", []string{"GET /product/{} ProductDetail"}},
		{"pages file", "pages/cart.jsx", "export default function Cart() { return <p/> }\n", []string{"GET /cart Cart"}},
		{"pages api is ANY, a wrapper names its argument", "pages/api/products/index.ts", "import type { NextApiHandler } from 'next';\nconst handler: NextApiHandler = async (req, res) => {};\nexport default InstrumentationMiddleware(handler);\n", []string{"ANY /api/products handler"}},
		{"pages api anonymous default", "pages/api/hello.js", "export default (req, res) => { res.json({}) }\n", []string{"ANY /api/hello"}},
		{"special pages are not routes", "pages/_app.tsx", page, nil},
		{"404 is not a route", "pages/404.tsx", page, nil},
		{"a page without React signals is not a page", "pages/helpers.ts", "export default function x() { return 1 }\n", nil},
		{"tests are not pages", "pages/index.test.tsx", page, nil},
		{"a components/pages folder is not a pages router", "src/components/pages/Foo.tsx", page, nil},
		{"node_modules is never read", "node_modules/next/dist/pages/_app.js", page, nil},
		{"Angular's src/app is not an app router", "src/app/home/home.component.ts", "export class HomeComponent {}\n", nil},
		{"export { X as default }", "pages/about.tsx", "function About() { return <p/> }\nexport { About as default }\n", []string{"GET /about About"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkResult(t, readNext(c.path, []byte(c.src)), c.routes, nil)
		})
	}
}
