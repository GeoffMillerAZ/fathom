import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import { chmod, mkdir, mkdtemp, readFile, readdir, realpath, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import {
  ProbeError, assertBundleCheckReceipt, assertBundleParentTransition, assertStrictState, assertWindowState,
  bundleLayoutForLauncher, bundleLaunchInvocation, finalOutcome, isProcessAbsentError, parseArgs,
  parseServerDiagnostic, requireNativeChildPID, run, validateDigest, validatedBundleInvocation,
  cleanupEarlyOwnedProcesses, processAbsent,
} from "./native_delivery_probe.mjs";

const digest = "a".repeat(64);

async function waitForFile(file) {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    try { return await readFile(file, "utf8"); } catch { await new Promise((resolve) => setTimeout(resolve, 20)); }
  }
  throw new Error(`timed out waiting for ${file}`);
}

async function waitForProcessPath(pid, expectedPath) {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    const observed = spawnSync("/bin/ps", ["-p", String(pid), "-o", "comm="], { encoding: "utf8", timeout: 2_000 });
    if (observed.status === 0 && observed.stdout.trim() === expectedPath) return;
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
  throw new Error(`timed out waiting for process ${pid} identity`);
}

async function stopExactPID(pid, expectedPath, expectedSHA) {
  if (!pid || await processAbsent(pid)) return;
  const observed = spawnSync("/bin/ps", ["-p", String(pid), "-o", "comm="], { encoding: "utf8", timeout: 2_000 });
  if (observed.status === 1 && observed.stdout === "" && observed.stderr === "" && await processAbsent(pid)) return;
  assert.equal(observed.status, 0, `could not inspect cleanup PID ${pid}`);
  assert.equal(observed.stdout.trim(), expectedPath, `cleanup PID ${pid} changed identity`);
  assert.equal(await fileSHA(expectedPath), expectedSHA, `cleanup PID ${pid} executable changed`);
  process.kill(pid, "SIGKILL");
  for (let attempt = 0; attempt < 100 && !await processAbsent(pid); attempt += 1) await new Promise((resolve) => setTimeout(resolve, 20));
}

const fileSHA = async (file) => createHash("sha256").update(await readFile(file)).digest("hex");

function args(extra = []) {
  return ["--server", "/private/server", "--server-sha256", digest, "--native", "/private/native", "--native-sha256", digest, "--driver", "/private/driver", "--driver-sha256", digest, "--output-parent", "/private/out", ...extra];
}

function bundleArgs(extra = []) {
  return args(["--bundle-launcher", "/private/Gallery.app/Contents/MacOS/gallery-launcher",
    "--bundle-launcher-sha256", digest, "--bundle-manifest-sha256", digest,
    "--bundle-plist-sha256", digest, ...extra]);
}

function bundleFixture() {
  const layout = bundleLayoutForLauncher("/private/Gallery.app/Contents/MacOS/gallery-launcher");
  const identity = (file, bytes) => ({ path: file, sha256: digest, bytes, mode: 0o700, dev: 1, ino: bytes });
  const snapshot = {
    layout,
    launcher: identity(layout.launcher, 101), server: identity(layout.server, 102), native: identity(layout.native, 103),
    manifest: identity(layout.manifest, 104), plist: identity(layout.plist, 105),
  };
  snapshot.manifestValue = { schema_version: 1, mode: "private", kit_revision: "b".repeat(40),
    wails_version: "v3.0.0-beta.21", min_macos: "12.0", files: [
      { path: "Contents/MacOS/gallery-launcher", sha256: digest, size: 101 },
      { path: "Contents/Resources/gallery-server", sha256: digest, size: 102 },
      { path: "Contents/Resources/gallery-native", sha256: digest, size: 103 },
    ] };
  const receipt = { checked_only: true, plan: { Executable: layout.server,
    Args: ["-native-host", layout.native, "-native-host-sha256", digest], Manifest: snapshot.manifestValue } };
  return { snapshot, receipt, options: { bundleMode: true } };
}

test("parseArgs accepts only the complete explicit input set", () => {
  const parsed = parseArgs(args());
  assert.equal(parsed.server, "/private/server");
  assert.equal(parsed.outputParent, "/private/out");
  assert.equal(parsed.serverSha256, digest);
});

test("parseArgs refuses malformed, missing, repeated, and unrecognised options", () => {
  assert.throws(() => parseArgs(args().slice(0, -2)), ProbeError);
  assert.throws(() => parseArgs(args(["--server", "/other"])), ProbeError);
  assert.throws(() => parseArgs(args(["--unexpected", "x"])), ProbeError);
  assert.throws(() => parseArgs(args().map((value) => value === digest ? "A".repeat(64) : value)), ProbeError);
  assert.throws(() => parseArgs(args().map((value) => value === digest ? "a".repeat(63) : value)), ProbeError);
});

test("bundle options are all-or-none and repeated options remain forbidden", () => {
  const parsed = parseArgs(bundleArgs());
  assert.equal(parsed.bundleMode, true);
  assert.equal(parsed.bundleLauncherSha256, digest);
  assert.throws(() => parseArgs(args(["--bundle-launcher", "/private/Gallery.app/Contents/MacOS/gallery-launcher"])),
    (error) => error.code === "invalid-arguments");
  assert.throws(() => parseArgs(bundleArgs(["--bundle-launcher", "/other"])),
    (error) => error.code === "invalid-arguments");
  assert.equal(parseArgs(args()).bundleMode, false);
});

test("bundle layout and checked-only plan stay fixed to the paired resources", () => {
  const { snapshot, receipt } = bundleFixture();
  assert.equal(bundleLayoutForLauncher(snapshot.layout.launcher).server, snapshot.layout.server);
  assert.deepEqual(assertBundleCheckReceipt(receipt, snapshot), receipt);
  assert.throws(() => bundleLayoutForLauncher("/private/Gallery.app/gallery-launcher"), ProbeError);
  assert.throws(() => assertBundleCheckReceipt({ ...receipt, plan: { ...receipt.plan, Executable: "/private/other" } }, snapshot),
    (error) => error.code === "bundle-plan-mismatch");
  assert.throws(() => assertBundleCheckReceipt({ ...receipt, plan: { ...receipt.plan, Args: [...receipt.plan.Args, "-browser"] } }, snapshot),
    (error) => error.code === "bundle-plan-mismatch");
});

test("failed check cannot yield launch and successful bundle launch has no arguments", () => {
  const { snapshot, receipt, options } = bundleFixture();
  const failed = { code: 1, signal: null, timedOut: false, stdoutOverflow: false, stderrOverflow: false, stdout: "", stderr: "refused" };
  assert.throws(() => validatedBundleInvocation(options, snapshot, failed), (error) => error.code === "bundle-check-failed");
  const checked = { ...failed, code: 0, stderr: "", stdout: JSON.stringify(receipt) };
  const resolved = validatedBundleInvocation(options, snapshot, checked);
  assert.deepEqual(resolved.invocation, bundleLaunchInvocation(options, snapshot));
  assert.deepEqual(resolved.invocation.args, []);
  assert.equal(resolved.invocation.command, snapshot.layout.launcher);
});

test("synthetic exec transition binds the same PID and exact launcher/server identities", () => {
  const { snapshot, options } = bundleFixture();
  const invocation = bundleLaunchInvocation(options, snapshot);
  assert.deepEqual(assertBundleParentTransition(invocation, 4242,
    { pid: 4242, path: snapshot.server.path, sha256: snapshot.server.sha256 }),
  { pid: 4242, launcher: snapshot.launcher, server: snapshot.server, launcher_args: [] });
  assert.throws(() => assertBundleParentTransition(invocation, 4242,
    { pid: 4243, path: snapshot.server.path, sha256: snapshot.server.sha256 }), ProbeError);
  assert.throws(() => assertBundleParentTransition(invocation, 4242,
    { pid: 4242, path: snapshot.native.path, sha256: snapshot.native.sha256 }), ProbeError);
});

test("bundle check refusal runs only -check and extracts no bundle executables", { skip: process.platform !== "darwin" }, async (t) => {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "appkit-delivery-bundle-refusal-")));
  await chmod(root, 0o700);
  const bundle = path.join(root, "Synthetic.app");
  const macOS = path.join(bundle, "Contents", "MacOS");
  const resources = path.join(bundle, "Contents", "Resources");
  const output = path.join(root, "output");
  for (const dir of [bundle, path.join(bundle, "Contents"), macOS, resources, output]) await mkdir(dir, { mode: 0o700 });
  const calls = path.join(root, "launcher-calls.txt");
  const launcher = path.join(macOS, "gallery-launcher");
  const server = path.join(resources, "gallery-server");
  const native = path.join(resources, "gallery-native");
  const driver = path.join(root, "driver");
  await writeFile(launcher, `#!/bin/sh\nprintf '%s\\n' "$#:$*" >> '${calls}'\nif [ "$1" = "-check" ] && [ "$#" -eq 1 ]; then exit 41; fi\nexit 97\n`, { mode: 0o700 });
  for (const file of [server, native, driver]) await writeFile(file, "#!/bin/sh\nexit 97\n", { mode: 0o700 });
  const manifest = path.join(resources, "bundle.json");
  const plist = path.join(bundle, "Contents", "Info.plist");
  await writeFile(manifest, "{}\n", { mode: 0o600 });
  await writeFile(plist, "synthetic plist\n", { mode: 0o600 });
  const sum = async (file) => createHash("sha256").update(await readFile(file)).digest("hex");
  const options = ["--server", server, "--server-sha256", await sum(server), "--native", native,
    "--native-sha256", await sum(native), "--driver", driver, "--driver-sha256", await sum(driver),
    "--output-parent", output, "--bundle-launcher", launcher, "--bundle-launcher-sha256", await sum(launcher),
    "--bundle-manifest-sha256", await sum(manifest), "--bundle-plist-sha256", await sum(plist)];
  await assert.rejects(run(options), (error) => error.code === "bundle-check-failed");
  assert.equal(await readFile(calls, "utf8"), "1:-check\n");
  const runs = await readdir(output);
  assert.equal(runs.length, 1);
  const runDir = path.join(output, runs[0]);
  const names = await readdir(runDir);
  assert(!names.includes("gallery-server"));
  assert(!names.includes("gallery-native-input"));
  assert(!names.includes("gallery-launcher"));
  const receipt = JSON.parse(await readFile(path.join(runDir, "receipt.json"), "utf8"));
  assert.equal(receipt.error.code, "bundle-check-failed");
  assert.deepEqual(receipt.bundle_check.args, ["-check"]);
  t.diagnostic(`retained synthetic refusal: ${runDir}`);
});

test("validateDigest requires lower-case sha256", () => {
  assert.equal(validateDigest(digest), digest);
  for (const bad of ["", "0".repeat(63), "A".repeat(64), "g".repeat(64), null]) assert.throws(() => validateDigest(bad), ProbeError);
});

test("parseServerDiagnostic accepts only the non-secret expected startup line", () => {
  assert.equal(parseServerDiagnostic("Experimental synthetic Gallery at http://127.0.0.1:49152 (authorization required)\n"), 49152);
  for (const bad of [
    "Experimental synthetic Gallery at http://localhost:49152 (authorization required)",
    "Experimental synthetic Gallery at http://127.0.0.1:0 (authorization required)",
    "Experimental synthetic Gallery at http://127.0.0.1:49152/?token=x (authorization required)",
  ]) assert.throws(() => parseServerDiagnostic(bad), ProbeError);
});

test("strict driver receipts refuse extra fields, malformed values, and wrong window counts", () => {
  const base = { ok: true, operation: "inspect", pid: 42, windows: 1, new_window_enabled: true, browser_enabled: true };
  assert.deepEqual(assertStrictState(base, "inspect", 42), base);
  assert.throws(() => assertStrictState({ ...base, token: "no" }, "inspect", 42), ProbeError);
  assert.throws(() => assertStrictState({ ...base, windows: -1 }, "inspect", 42), ProbeError);
  assert.throws(() => assertStrictState({ ...base, windows: 1.5 }, "inspect", 42), ProbeError);
  assert.throws(() => assertStrictState({ ...base, operation: "open-browser" }, "inspect", 42), ProbeError);
  assert.throws(() => assertStrictState({ ...base, pid: 43 }, "inspect", 42), ProbeError);
  assert.throws(() => assertStrictState({ ...base, browser_enabled: "true" }, "inspect", 42), ProbeError);
});

test("scenario state refuses incorrect window and enablement transitions", () => {
  const one = { ok: true, operation: "inspect", pid: 42, windows: 1, new_window_enabled: true, browser_enabled: true };
  assert.doesNotThrow(() => assertWindowState(one, { windows: 1, newWindowEnabled: true }));
  assert.throws(() => assertWindowState({ ...one, windows: 2 }, { windows: 1, newWindowEnabled: true }), ProbeError);
  assert.throws(() => assertWindowState({ ...one, new_window_enabled: false }, { windows: 1, newWindowEnabled: true }), ProbeError);
  assert.throws(() => assertWindowState({ ...one, browser_enabled: false }, { windows: 1, newWindowEnabled: true }), ProbeError);
});

test("cleanup outcome and absence classification fail closed", () => {
  assert.equal(finalOutcome(true, undefined), "passed");
  assert.equal(finalOutcome(false, undefined), "failed");
  assert.equal(finalOutcome(true, new ProbeError("cleanup failed")), "failed");
  assert.equal(isProcessAbsentError({ code: "ESRCH" }), true);
  assert.equal(isProcessAbsentError({ code: "EPERM" }), false);
  assert.equal(isProcessAbsentError({}), false);
  assert.throws(() => requireNativeChildPID([]), (error) => error.code === "native-child-pending");
  assert.equal(requireNativeChildPID([42]), 42);
  assert.throws(() => requireNativeChildPID([42, 43]), (error) => error.code === "native-child-missing");
});

test("early cleanup discovers and removes the exact private direct child", { skip: process.platform !== "darwin" }, async (t) => {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "appkit-delivery-early-cleanup-")));
  await chmod(root, 0o700);
  const nativeDir = path.join(root, "appkit-native-companion-synthetic");
  await mkdir(nativeDir, { mode: 0o700 });
  const native = path.join(nativeDir, "gallery-native");
  await writeFile(native, await readFile("/bin/sleep"), { mode: 0o500 });
  const pidFile = path.join(root, "child.pid");
  const script = `const {spawn}=require('node:child_process');const fs=require('node:fs');const child=spawn(process.argv[1],['30'],{stdio:'ignore'});fs.writeFileSync(process.argv[2],String(child.pid));process.on('SIGTERM',()=>process.exit(0));setInterval(()=>{},1000);`;
  const parentSHA = await fileSHA(process.execPath);
  const nativeSHA = await fileSHA(native);
  const parent = spawn(process.execPath, ["-e", script, native, pidFile], { stdio: "ignore" });
  let childPID;
  t.after(async () => {
    if (!childPID) { try { childPID = Number(await readFile(pidFile, "utf8")); } catch { /* no observed child to authorize */ } }
    await stopExactPID(parent.pid, process.execPath, parentSHA);
    if (childPID) await stopExactPID(childPID, native, nativeSHA);
  });
  childPID = Number(await waitForFile(pidFile));
  await waitForProcessPath(childPID, native);
  const context = { runDir: root, env: { PATH: "/usr/bin:/bin", TMPDIR: root }, deadline: Date.now() + 10_000 };
  const identity = { pid: parent.pid, label: "synthetic parent", permitted: [{ path: process.execPath, sha256: parentSHA }] };
  // A refused digest must leave this same fixture intact for a valid cleanup.
  // Keep the original failing sequence; separate fixtures cannot replace it.
  await assert.rejects(cleanupEarlyOwnedProcesses(identity, "b".repeat(64), context),
    (error) => error.code === "identity-mismatch");
  assert.equal(await processAbsent(parent.pid), false);
  assert.equal(await processAbsent(childPID), false);
  const result = await cleanupEarlyOwnedProcesses(identity, nativeSHA, context);
  assert.equal(result.parent_absent, true);
  assert.equal(await processAbsent(parent.pid), true);
  assert.equal(result.native_absent, true);
  assert.equal(await processAbsent(childPID), true);
});

test("early cleanup refuses a mismatched direct-child digest without signaling either process", { skip: process.platform !== "darwin" }, async (t) => {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "appkit-delivery-child-refusal-")));
  await chmod(root, 0o700);
  const nativeDir = path.join(root, "appkit-native-companion-synthetic");
  await mkdir(nativeDir, { mode: 0o700 });
  const native = path.join(nativeDir, "gallery-native");
  await writeFile(native, await readFile("/bin/sleep"), { mode: 0o500 });
  const pidFile = path.join(root, "child.pid");
  const script = `const {spawn}=require('node:child_process');const fs=require('node:fs');const child=spawn(process.argv[1],['30'],{stdio:'ignore'});fs.writeFileSync(process.argv[2],String(child.pid));process.on('SIGTERM',()=>process.exit(0));setInterval(()=>{},1000);`;
  const parentSHA = await fileSHA(process.execPath);
  const nativeSHA = await fileSHA(native);
  const parent = spawn(process.execPath, ["-e", script, native, pidFile], { stdio: "ignore" });
  let childPID;
  t.after(async () => {
    if (!childPID) { try { childPID = Number(await readFile(pidFile, "utf8")); } catch { /* no observed child to authorize */ } }
    await stopExactPID(parent.pid, process.execPath, parentSHA);
    if (childPID) await stopExactPID(childPID, native, nativeSHA);
  });
  childPID = Number(await waitForFile(pidFile));
  await waitForProcessPath(childPID, native);
  const context = { runDir: root, env: { PATH: "/usr/bin:/bin", TMPDIR: root }, deadline: Date.now() + 10_000 };
  const identity = { pid: parent.pid, label: "synthetic parent", permitted: [{ path: process.execPath, sha256: parentSHA }] };
  await assert.rejects(cleanupEarlyOwnedProcesses(identity, "b".repeat(64), context),
    (error) => error.code === "identity-mismatch");
  assert.equal(await processAbsent(parent.pid), false);
  assert.equal(await processAbsent(childPID), false);
});

test("early cleanup handles no child, exited parent, and parent identity mismatch", { skip: process.platform !== "darwin" }, async (t) => {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "appkit-delivery-parent-cleanup-")));
  await chmod(root, 0o700);
  const context = { runDir: root, env: { PATH: "/usr/bin:/bin", TMPDIR: root }, deadline: Date.now() + 10_000 };
  const hold = `process.on('SIGTERM',()=>process.exit(0));setInterval(()=>{},1000);`;
  const parentSHA = await fileSHA(process.execPath);
  const parent = spawn(process.execPath, ["-e", hold], { stdio: "ignore" });
  t.after(async () => stopExactPID(parent.pid, process.execPath, parentSHA));
  const good = { pid: parent.pid, label: "synthetic parent", permitted: [{ path: process.execPath, sha256: parentSHA }] };
  await assert.rejects(cleanupEarlyOwnedProcesses({ ...good, permitted: [{ path: process.execPath, sha256: "c".repeat(64) }] }, digest, context),
    (error) => error.code === "identity-mismatch");
  assert.equal(await processAbsent(parent.pid), false);
  assert.deepEqual(await cleanupEarlyOwnedProcesses(good, digest, context), { parent_absent: true, native_absent: true, native: null });

  const exited = spawn(process.execPath, ["-e", ""], { stdio: "ignore" });
  await new Promise((resolve) => exited.once("close", resolve));
  assert.deepEqual(await cleanupEarlyOwnedProcesses({ ...good, pid: exited.pid }, digest, context),
    { parent_absent: true, native_absent: null, native: null, reason: "parent-exited-before-child-observation" });
});

test("a zero-exit non-host cannot produce native delivery success", { skip: process.platform !== "darwin" }, async (t) => {
  // Deliberately inert executable: no native application or UI driver runs.
  // This exercises orchestration/receipt refusal, not only a pure helper.
  const noop = "/usr/bin/true";
  const sum = createHash("sha256").update(await readFile(noop)).digest("hex");
  const parent = await mkdtemp(path.join(tmpdir(), "appkit-delivery-refusal-"));
  const options = ["--server", noop, "--server-sha256", sum, "--native", noop, "--native-sha256", sum,
    "--driver", noop, "--driver-sha256", sum, "--output-parent", parent];
  await assert.rejects(run(options), (error) => error.code === "server-exited");
  const dirs = await readdir(parent);
  assert.equal(dirs.length, 1);
  const receiptPath = path.join(parent, dirs[0], "receipt.json");
  const receipt = JSON.parse(await readFile(receiptPath, "utf8"));
  assert.equal(receipt.outcome, "failed");
  assert.equal(receipt.scenario_succeeded, undefined);
  assert.equal(receipt.observed_parent_exit.code, 0);
  assert.equal(receipt.error.code, "server-exited");
  assert.equal(receipt.canaries.length, 2);
  t.diagnostic(`retained refusal receipt: ${receiptPath}`);
});
