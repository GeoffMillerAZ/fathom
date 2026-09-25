// Local, cached-toolchain proof. No installs, real data or production server.
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import crypto from 'node:crypto'
import { execFileSync } from 'node:child_process'

const root = fs.realpathSync(process.argv[2] || process.cwd())
const sha = b => crypto.createHash('sha256').update(b).digest('hex')
const archive = 'application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8'
const member = 'html/navigation/tabs/01-tabs-with-underline.html'
const sourceSHA = '01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31'
const fontRoot = 'data/design-assets/reference-assets/inter/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441'
const fontSHA = '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3'
const licenseSHA = '262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a'
const tailwindSHA = '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e'
const env = { ...process.env, GOENV:'off', GOWORK:'off', GOTOOLCHAIN:'local', GOFLAGS:'-mod=readonly', GOPROXY:'off', GOSUMDB:'sum.golang.org', GOPRIVATE:'', GONOPROXY:'', GONOSUMDB:'', GOINSECURE:'', NODE_OPTIONS:'', NODE_PATH:'' }
const run = (bin,args,cwd=root,overrides={}) => execFileSync(bin,args,{cwd,env:{...env,...overrides},timeout:120000,maxBuffer:16<<20,stdio:['ignore','pipe','pipe']})
const requireCheck = (ok,message) => { if(!ok) throw Error(message) }
const read = (p,max=2<<20) => { const s=fs.lstatSync(p);requireCheck(s.isFile()&&!s.isSymbolicLink()&&s.size<=max,'invalid fixture input');return fs.readFileSync(p) }
const write = (p,b) => fs.writeFileSync(p,b,{flag:'wx',mode:0o600})
const out = fs.mkdtempSync(path.join(os.tmpdir(),'appkit-p06-'))
fs.chmodSync(out,0o700)
const report = {schema:1,scope:'cached-private-module-and-navigation-fixture',verification:'not-verified',source:{archive,member,sha256:sourceSHA},checks:[],versions:{},directory:out}
const record = (name,ok) => { report.checks.push({name,ok});requireCheck(ok,name) }
const assets = path.join(root,'data/design-assets')
const sources = [path.join(assets,'catalog.json'),path.join(assets,'archives',archive.split('@')[1]+'.zip')]
const sourceHashes = sources.map(p=>sha(read(p,512<<20)))

try {
  requireCheck(process.platform==='darwin'&&process.arch==='arm64','this bounded harness pins macOS arm64')
  record('original-archive',sourceHashes[1]===archive.split('@')[1])
  const original = JSON.parse(run('go',['run','./cmd/appkit','data','assets-get','-root',assets,'-id',archive,'-path',member]))
  record('original-member',original.sha256===sourceSHA&&sha(Buffer.from(original.content))===sourceSHA)
  write(path.join(out,'source.html'),original.content)

  const delivery = path.join(out,'delivery')
  run('go',['run','./cmd/appkit','data','assets-adapt-navigation','-root',assets,'-out',delivery])
  run('go',['run','./cmd/appkit','data','assets-adapt-navigation','-root',assets,'-check',delivery])
  const artifactBytes = read(path.join(delivery,'artifact.json'))
  const artifact = JSON.parse(artifactBytes)
  record('artifact-identity',artifact.source.sha256===sourceSHA&&artifact.verification==='not-verified'&&artifact.adapter==='p06-page-navigation-v1')

  const build = path.join(out,'build');fs.mkdirSync(build,{mode:0o700})
  for(const name of ['navigation.go','navigation.templ','defaults.json']) write(path.join(build,name),read(path.join(delivery,name)))
  const module = read(path.join(root,'docs/assessments/fixtures/pricing-adapter/consumer.mod.txt')).toString()
  requireCheck(module.startsWith('module example.invalid/pricing-proof\n'),'module fixture identity')
  write(path.join(build,'go.mod'),module.replace('module example.invalid/pricing-proof\n','module example.invalid/navigation-proof\n'))
  write(path.join(build,'go.sum'),read(path.join(root,'docs/assessments/fixtures/pricing-adapter/consumer.sum.txt')))
  const defaults = read(path.join(build,'defaults.json')).toString()
  const runtime = read(path.join(root,'internal/designadapt/navigation_runtime_test.go.txt')).toString()
  write(path.join(build,'runtime_test.go'),runtime+`\nconst sourceExpected = ${JSON.stringify(original.content)}\nconst sourceDefaults = ${JSON.stringify(defaults)}\n`)
  write(path.join(build,'preview_test.go'),read(path.join(root,'docs/assessments/fixtures/navigation-adapter/preview_test.go.txt')))

  const buildInputs = ['go.mod','go.sum','navigation.go','navigation.templ','runtime_test.go','preview_test.go','defaults.json']
  const buildHashes = buildInputs.map(p=>sha(read(path.join(build,p))))
  record('own-module',fs.realpathSync(run('go',['env','GOMOD'],build).toString().trim())===fs.realpathSync(path.join(build,'go.mod')))
  run('go',['mod','verify'],build);record('cached-module-checksums',true)
  let canary = false
  try { run('go',['test','-run','^TestRuntimeSourceDefaultsAndExactDOMParity$'],build) } catch(e) {
    const diagnostic = Buffer.concat([e.stdout||Buffer.alloc(0),e.stderr||Buffer.alloc(0)]).toString()
    canary = diagnostic.includes('undefined: pageNavigation') || diagnostic.includes('undefined: navigationLink')
  }
  record('missing-generation-canary',canary)
  report.versions.go = run('go',['version'],build).toString().trim()
  report.versions.templ = run('go',['tool','templ','version'],build).toString().trim()
  run('go',['tool','templ','generate','-f','navigation.templ','-include-version=false'],build)
  const rawTests = run('go',['test','-count=1','-v','.'],build)
  write(path.join(out,'tests-raw.txt'),rawTests);record('raw-source-runtime-and-native-http-tests',true)

  const formatted = path.join(out,'formatted')
  fs.cpSync(build,formatted,{recursive:true,errorOnExist:true,force:false})
  run('go',['tool','templ','fmt','navigation.templ'],formatted)
  fs.rmSync(path.join(formatted,'navigation_templ.go'))
  run('go',['tool','templ','generate','-f','navigation.templ','-include-version=false'],formatted)
  const formattedTests = run('go',['test','-count=1','-v','.'],formatted)
  write(path.join(out,'tests-formatted.txt'),formattedTests);record('formatted-source-runtime-and-native-http-tests',true)

  const deps = run('go',['list','-deps','-f','{{.ImportPath}} {{.Dir}}','.'],build).toString()
  record('consumer-no-kit-dependency',deps.includes('github.com/a-h/templ ')&&!deps.includes('github.com/geoffmilleraz/')&&!deps.includes(root))
  record('build-inputs-unchanged',buildInputs.every((p,i)=>sha(read(path.join(build,p)))===buildHashes[i]))
  const binary = path.join(out,'preview.test')
  run('go',['test','-c','-o',binary,'.'],formatted)

  const tw = path.join(os.homedir(),'Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64')
  record('cached-tailwind-pin',sha(read(tw,512<<20))===tailwindSHA)
  const twCopy = path.join(out,'tailwindcss');write(twCopy,read(tw,512<<20));fs.chmodSync(twCopy,0o700)
  const font = read(path.join(root,fontRoot,'font-files/InterVariable.woff2'))
  const license = read(path.join(root,fontRoot,'LICENSE.txt'))
  record('font-and-license-pins',sha(font)===fontSHA&&sha(license)===licenseSHA)
  write(path.join(out,'input.css'),`@import "tailwindcss" source(none);
@source "./source.html";
@source "./build/navigation.templ";
@source "./build/navigation.go";
@source "./build/preview_test.go";
@custom-variant dark (&:where(.dark, .dark *));
@theme { --font-sans: "InterVariable", sans-serif; }
:root { color-scheme:light; }
:root.dark { color-scheme:dark; }
body { margin:0; background:#fff; color:#111827; font-family:InterVariable,sans-serif; }
.dark body { background:#111827; color:#f9fafb; }
main { padding:24px; }
`)
  run(twCopy,['-i','input.css','-o','tailwind.css','--minify'],out,{TMPDIR:out})
  const css = read(path.join(out,'tailwind.css'))
  write(path.join(out,'navigation.css'),`/* Inter v4.1 SIL Open Font License\n${license}\n*/\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(data:font/woff2;base64,${font.toString('base64')}) format('woff2')}\n${css}`)

  run('go',['run','./cmd/appkit','data','assets-adapt-navigation','-root',assets,'-check',delivery])
  record('delivery-remains-immutable',true)
  record('original-catalog-export-unchanged',sources.every((p,i)=>sha(read(p,512<<20))===sourceHashes[i])&&sha(read(path.join(delivery,'artifact.json')))===sha(artifactBytes))
  report.versions.tailwind = 'v4.3.0'
  report.artifact_sha256 = sha(artifactBytes)
  report.css_sha256 = sha(read(path.join(out,'navigation.css')))
  report.css_bytes = read(path.join(out,'navigation.css')).length
  report.binary_sha256 = sha(read(binary,128<<20))
  report.status = 'passed'
} catch(e) {
  report.status = 'failed';report.error = e.message
  if(e.stdout||e.stderr) write(path.join(out,'failure.txt'),Buffer.concat([e.stdout||Buffer.alloc(0),e.stderr||Buffer.alloc(0)]))
  process.exitCode = 1
} finally {
  write(path.join(out,'prepare.json'),JSON.stringify(report,null,2)+'\n')
  console.log(JSON.stringify(report))
}
