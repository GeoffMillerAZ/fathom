// Local, cached-toolchain proof. No installs, real data or production server.
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import crypto from 'node:crypto'
import { execFileSync } from 'node:child_process'

const root = fs.realpathSync(process.argv[2] || process.cwd())
const sha = b => crypto.createHash('sha256').update(b).digest('hex')
const archive = 'application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8'
const member = 'html/forms/input-groups/03-input-with-validation-error.html'
const sourceSHA = 'c35ba209333b7a4897c93a8c696b7fe306f4797625bc42906090e6edf864cc19'
const fontRoot = 'data/design-assets/reference-assets/inter/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441'
const fontSHA = '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3'
const licenseSHA = '262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a'
const tailwindSHA = '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'
const env = { ...process.env, GOENV:'off', GOWORK:'off', GOTOOLCHAIN:'local', GOFLAGS:'-mod=readonly', GOPROXY:'off', GOSUMDB:'sum.golang.org', GOPRIVATE:'', GONOPROXY:'', GONOSUMDB:'', GOINSECURE:'', NODE_OPTIONS:'', NODE_PATH:'' }
const run = (bin,args,cwd=root,overrides={}) => execFileSync(bin,args,{ cwd,env:{...env,...overrides},timeout:120000,maxBuffer:16<<20,stdio:['ignore','pipe','pipe'] })
const requireCheck = (ok,message) => { if(!ok) throw Error(message) }
const read = (p,max=2<<20) => { const s=fs.lstatSync(p); requireCheck(s.isFile()&&!s.isSymbolicLink()&&s.size<=max,'invalid fixture input'); return fs.readFileSync(p) }
const write = (p,b) => fs.writeFileSync(p,b,{flag:'wx',mode:0o600})
const out = fs.mkdtempSync(path.join(os.tmpdir(),'appkit-p05-'))
fs.chmodSync(out,0o700)
const report = { schema:1,scope:'cached-private-module-and-native-fixture',verification:'not-verified',source:{archive,member,sha256:sourceSHA},checks:[],versions:{},directory:out }
const record = (name,ok) => { report.checks.push({name,ok}); requireCheck(ok,name) }
const assets=path.join(root,'data/design-assets')
const sources=[path.join(assets,'catalog.json'),path.join(assets,'archives',archive.split('@')[1]+'.zip')]
const sourceHashes=sources.map(p=>sha(read(p,512<<20)))
try {
  requireCheck(process.platform==='darwin'&&process.arch==='arm64','this bounded harness pins macOS arm64')
  record('original-archive',sourceHashes[1]===archive.split('@')[1])
  const original=JSON.parse(run('go',['run','./cmd/appkit','data','assets-get','-root',assets,'-id',archive,'-path',member]))
  record('original-member',original.sha256===sourceSHA&&sha(Buffer.from(original.content))===sourceSHA)
  write(path.join(out,'source.html'),original.content)
  const delivery=path.join(out,'delivery')
  run('go',['run','./cmd/appkit','data','assets-adapt-form','-root',assets,'-out',delivery])
  run('go',['run','./cmd/appkit','data','assets-adapt-form','-root',assets,'-check',delivery])
  const artifact=JSON.parse(read(path.join(delivery,'artifact.json')))
  record('artifact-identity',artifact.source.sha256===sourceSHA&&artifact.verification==='not-verified'&&artifact.adapter==='p05-email-field-v1')
  const build=path.join(out,'build');fs.mkdirSync(build,{mode:0o700})
  for(const name of ['form.go','form.templ','defaults.json'])write(path.join(build,name),read(path.join(delivery,name)))
  const module=read(path.join(root,'docs/assessments/fixtures/pricing-adapter/consumer.mod.txt')).toString()
  requireCheck(module.startsWith('module example.invalid/pricing-proof\n'),'module fixture identity')
  write(path.join(build,'go.mod'),module.replace('module example.invalid/pricing-proof\n','module example.invalid/form-proof\n'))
  write(path.join(build,'go.sum'),read(path.join(root,'docs/assessments/fixtures/pricing-adapter/consumer.sum.txt')))
  write(path.join(build,'workflow_test.go'),read(path.join(root,'docs/assessments/fixtures/form-adapter/workflow_test.go.txt')))
  write(path.join(build,'parity_test.go'),`package converted
import("bytes";"context";"encoding/json";"os";"reflect";"strings";"testing";"golang.org/x/net/html")
func TestActualSourceParity(t *testing.T){source,e:=os.ReadFile(os.Getenv("APPKIT_P05_ORIGINAL"));if e!=nil{t.Fatal(e)};raw,e:=os.ReadFile("defaults.json");if e!=nil{t.Fatal(e)};var data EmailField;if json.Unmarshal(raw,&data)!=nil{t.Fatal("defaults")};c,e:=NewEmailField(data);if e!=nil{t.Fatal(e)};var b bytes.Buffer;if c.Render(context.Background(),&b)!=nil{t.Fatal("render")};a,e:=html.Parse(strings.NewReader(string(source)));if e!=nil{t.Fatal(e)};got,e:=html.Parse(strings.NewReader(b.String()));if e!=nil||!reflect.DeepEqual(a,got){t.Fatal("actual original/default parsed DOM differs; private bodies withheld")}}
`)
  const buildInputs=['go.mod','go.sum','form.go','form.templ','workflow_test.go','parity_test.go','defaults.json']
  const buildHashes=buildInputs.map(p=>sha(read(path.join(build,p))))
  record('own-module',fs.realpathSync(run('go',['env','GOMOD'],build).toString().trim())===fs.realpathSync(path.join(build,'go.mod')))
  run('go',['mod','verify'],build);record('cached-module-checksums',true)
  let canary=false
  try { run('go',['test','-run','^TestActualSourceParity$'],build) } catch(e) { canary=Buffer.concat([e.stdout||Buffer.alloc(0),e.stderr||Buffer.alloc(0)]).toString().includes('undefined: emailField') }
  record('missing-generation-canary',canary)
  report.versions.go=run('go',['version'],build).toString().trim()
  report.versions.templ=run('go',['tool','templ','version'],build).toString().trim()
  run('go',['tool','templ','generate','-f','form.templ','-include-version=false'],build)
  const fixtureEnv={APPKIT_P05_ORIGINAL:path.join(out,'source.html'),APPKIT_P05_DEFAULTS:path.join(build,'defaults.json')}
  const tests=run('go',['test','-count=1','-v','.'],build,fixtureEnv)
  write(path.join(out,'tests.txt'),tests);record('actual-source-and-native-http-tests',true)
  const deps=run('go',['list','-deps','-f','{{.ImportPath}} {{.Dir}}','.'],build).toString()
  record('consumer-no-kit-dependency',deps.includes('github.com/a-h/templ ')&&!deps.includes('github.com/geoffmilleraz/')&&!deps.includes(root))
  record('build-inputs-unchanged',buildInputs.every((p,i)=>sha(read(path.join(build,p)))===buildHashes[i]))
  const binary=path.join(out,'workflow.test');run('go',['test','-c','-o',binary,'.'],build)
  const tw=path.join(os.homedir(),'Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64')
  record('cached-tailwind-pin',sha(read(tw,512<<20))===tailwindSHA)
  const twCopy=path.join(out,'tailwindcss');write(twCopy,read(tw,512<<20));fs.chmodSync(twCopy,0o700)
  const font=read(path.join(root,fontRoot,'font-files/InterVariable.woff2'))
  const license=read(path.join(root,fontRoot,'LICENSE.txt'))
  record('font-and-license-pins',sha(font)===fontSHA&&sha(license)===licenseSHA)
  write(path.join(out,'input.css'),`@import "tailwindcss" source(none);
@source "./source.html";
@source "./build/form.templ";
@source "./build/form.go";
@custom-variant dark (&:where(.dark, .dark *));
@theme { --font-sans: "InterVariable", sans-serif; }
:root { color-scheme:light; }
:root.dark { color-scheme:dark; }
body { margin:0; background:#fff; color:#111827; }
.dark body { background:#111827; color:#f9fafb; }
`)
  run(twCopy,['-i','input.css','-o','tailwind.css','--minify'],out,{TMPDIR:out})
  const css=read(path.join(out,'tailwind.css'))
  write(path.join(out,'form.css'),`/* Inter v4.1 SIL Open Font License\n${license}\n*/\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(data:font/woff2;base64,${font.toString('base64')}) format('woff2')}\n${css}`)
  run('go',['run','./cmd/appkit','data','assets-adapt-form','-root',assets,'-check',delivery])
  record('delivery-remains-immutable',true)
  report.versions.tailwind='v4.3.0';report.artifact_sha256=sha(read(path.join(delivery,'artifact.json')))
  report.css_sha256=sha(read(path.join(out,'form.css')));report.css_bytes=read(path.join(out,'form.css')).length
  report.binary_sha256=sha(read(binary,128<<20))
  record('original-catalog-unchanged',sources.every((p,i)=>sha(read(p,512<<20))===sourceHashes[i]))
  report.status='passed'
} catch(e) {
  report.status='failed';report.error=e.message
  // Keep diagnostics but never echo paid artifact bodies.
  if(e.stdout||e.stderr)write(path.join(out,'failure.txt'),Buffer.concat([e.stdout||Buffer.alloc(0),e.stderr||Buffer.alloc(0)]))
  process.exitCode=1
} finally {
  write(path.join(out,'prepare.json'),JSON.stringify(report,null,2)+'\n')
  console.log(JSON.stringify(report))
}
