#!/usr/bin/env node
// Bounded, opt-in runtime evidence harness for the already-built Gallery pair.
// It deliberately does not build either binary or open a browser itself.
import { createHash } from "node:crypto";
import { constants as fsConstants } from "node:fs";
import {
  access,
  chmod,
  copyFile,
  lstat,
  mkdir,
  mkdtemp,
  open,
  readFile,
  realpath,
  stat,
  writeFile,
} from "node:fs/promises";
import net from "node:net";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawn } from "node:child_process";

const MAX_PARENT_BUFFER = 64 * 1024;
const MAX_DRIVER_OUTPUT = 4 * 1024;
const COMMAND_TIMEOUT_MS = 5_000;
const HARD_BUDGET_MS = 90_000;
const MAX_METADATA_BYTES = 64 * 1024;
const FLEET_SOURCE_ROOTS = ["/Volumes/m2ext/code/geoffmilleraz", "/Users/geoff/code/geoffmilleraz"];
const REQUIRED = ["server", "server-sha256", "native", "native-sha256", "driver", "driver-sha256", "output-parent"];
const BUNDLE_REQUIRED = ["bundle-launcher", "bundle-launcher-sha256", "bundle-manifest-sha256", "bundle-plist-sha256"];
const ALLOWED = [...REQUIRED, ...BUNDLE_REQUIRED];
// The runtime accepts only an identity-pinned driver executable. Frozen Swift-source
// provenance belongs to the separately retained build receipt, not this launch harness.

export class ProbeError extends Error {
  constructor(message, code = "probe-failed") {
    super(message);
    this.name = "ProbeError";
    this.code = code;
  }
}

export function validateDigest(value, label = "sha256") {
  if (typeof value !== "string" || !/^[0-9a-f]{64}$/.test(value)) {
    throw new ProbeError(`${label} must be exactly 64 lowercase hexadecimal characters`, "invalid-sha256");
  }
  return value;
}

export function parseArgs(argv) {
  const values = Object.create(null);
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith("--") || token === "--") throw new ProbeError("only named options are accepted", "invalid-arguments");
    const name = token.slice(2);
    if (!ALLOWED.includes(name) || Object.hasOwn(values, name)) throw new ProbeError(`unexpected or repeated option --${name}`, "invalid-arguments");
    const value = argv[index + 1];
    if (typeof value !== "string" || value.startsWith("--")) throw new ProbeError(`missing value for --${name}`, "invalid-arguments");
    values[name] = value;
    index += 1;
  }
  for (const name of REQUIRED) if (!Object.hasOwn(values, name)) throw new ProbeError(`missing --${name}`, "invalid-arguments");
  values.serverSha256 = validateDigest(values["server-sha256"], "--server-sha256");
  values.nativeSha256 = validateDigest(values["native-sha256"], "--native-sha256");
  values.driverSha256 = validateDigest(values["driver-sha256"], "--driver-sha256");
  values.outputParent = values["output-parent"];
  const bundleCount = BUNDLE_REQUIRED.filter((name) => Object.hasOwn(values, name)).length;
  if (bundleCount !== 0 && bundleCount !== BUNDLE_REQUIRED.length) {
    throw new ProbeError("bundle mode requires --bundle-launcher and all three bundle hashes", "invalid-arguments");
  }
  values.bundleMode = bundleCount === BUNDLE_REQUIRED.length;
  if (values.bundleMode) {
    values.bundleLauncher = values["bundle-launcher"];
    values.bundleLauncherSha256 = validateDigest(values["bundle-launcher-sha256"], "--bundle-launcher-sha256");
    values.bundleManifestSha256 = validateDigest(values["bundle-manifest-sha256"], "--bundle-manifest-sha256");
    values.bundlePlistSha256 = validateDigest(values["bundle-plist-sha256"], "--bundle-plist-sha256");
  }
  return values;
}

export function assertStrictState(value, expectedOperation, expectedPID) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new ProbeError("driver output is not an object", "invalid-driver-receipt");
  const keys = Object.keys(value).sort();
  const expectedKeys = ["browser_enabled", "new_window_enabled", "ok", "operation", "pid", "windows"];
  if (keys.length !== expectedKeys.length || keys.some((key, index) => key !== expectedKeys[index])) {
    throw new ProbeError("driver output has an unexpected receipt shape", "invalid-driver-receipt");
  }
  if (value.ok !== true || value.operation !== expectedOperation || value.pid !== expectedPID ||
      !Number.isInteger(value.windows) || value.windows < 0 ||
      typeof value.new_window_enabled !== "boolean" || typeof value.browser_enabled !== "boolean") {
    throw new ProbeError("driver output has invalid state values", "invalid-driver-receipt");
  }
  return value;
}

export function parseServerDiagnostic(text) {
  const match = /^Experimental synthetic Gallery at http:\/\/127\.0\.0\.1:(\d+) \(authorization required\)$/m.exec(text);
  if (!match) throw new ProbeError("server did not emit the expected non-secret startup diagnostic", "missing-server-diagnostic");
  const port = Number(match[1]);
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new ProbeError("server diagnostic has invalid port", "invalid-server-diagnostic");
  return port;
}

function isAbsoluteNormalized(value) {
  return typeof value === "string" && path.isAbsolute(value) && path.normalize(value) === value;
}

async function requireAbsoluteDirectory(value, label) {
  if (!isAbsoluteNormalized(value)) throw new ProbeError(`${label} must be an absolute normalized path`, "invalid-path");
  const info = await lstat(value);
  if (info.isSymbolicLink() || !info.isDirectory()) throw new ProbeError(`${label} must be a non-symlink directory`, "invalid-path");
  return value;
}

async function requireExecutable(value, label) {
  if (!isAbsoluteNormalized(value)) throw new ProbeError(`${label} must be an absolute normalized path`, "invalid-path");
  const linkInfo = await lstat(value);
  const fileInfo = await stat(value);
  if (linkInfo.isSymbolicLink() || !fileInfo.isFile()) throw new ProbeError(`${label} must be a regular non-symlink file`, "invalid-path");
  await access(value, fsConstants.X_OK);
  return value;
}

async function sha256(file) {
  const data = await readFile(file);
  return createHash("sha256").update(data).digest("hex");
}

async function validatePinnedExecutable(file, digest, label) {
  await requireExecutable(file, label);
  const actual = await sha256(file);
  if (actual !== digest) throw new ProbeError(`${label} SHA256 mismatch`, "identity-mismatch");
  return actual;
}

function exactKeys(value, expected, label) {
  if (!value || typeof value !== "object" || Array.isArray(value) ||
      JSON.stringify(Object.keys(value).sort()) !== JSON.stringify([...expected].sort())) {
    throw new ProbeError(`${label} has an unexpected shape`, "invalid-bundle-check");
  }
}

export function bundleLayoutForLauncher(launcher) {
  if (!isAbsoluteNormalized(launcher) || path.basename(launcher) !== "gallery-launcher" ||
      path.basename(path.dirname(launcher)) !== "MacOS") {
    throw new ProbeError("bundle launcher must be the fixed Contents/MacOS/gallery-launcher path", "invalid-bundle-path");
  }
  const contents = path.dirname(path.dirname(launcher));
  const root = path.dirname(contents);
  if (path.basename(contents) !== "Contents" || path.extname(root) !== ".app" || path.basename(root) === ".app") {
    throw new ProbeError("bundle launcher must be inside a named .app bundle", "invalid-bundle-path");
  }
  const resources = path.join(contents, "Resources");
  return {
    root, contents, macOS: path.dirname(launcher), resources, launcher,
    server: path.join(resources, "gallery-server"), native: path.join(resources, "gallery-native"),
    manifest: path.join(resources, "bundle.json"), plist: path.join(contents, "Info.plist"),
  };
}

async function requirePrivateDirectory(value, label) {
  const info = await lstat(value);
  if (info.isSymbolicLink() || !info.isDirectory() || await realpath(value) !== value) {
    throw new ProbeError(`${label} must be a real non-symlink directory`, "invalid-bundle-path");
  }
  if ((info.mode & 0o077) !== 0) throw new ProbeError(`${label} must be private`, "invalid-bundle-mode");
}

async function pinnedFileIdentity(file, digest, label, { executable = false, maxBytes = 256 * 1024 * 1024 } = {}) {
  const before = await lstat(file);
  if (before.isSymbolicLink() || !before.isFile() || await realpath(file) !== file) {
    throw new ProbeError(`${label} must be a real regular file`, "invalid-bundle-path");
  }
  if ((before.mode & 0o077) !== 0 || (executable && (before.mode & 0o100) === 0)) {
    throw new ProbeError(`${label} has a non-private or non-executable mode`, "invalid-bundle-mode");
  }
  if (before.size <= 0 || before.size > maxBytes) throw new ProbeError(`${label} has an invalid size`, "invalid-bundle-size");
  const handle = await open(file, "r");
  try {
    const opened = await handle.stat();
    if (opened.dev !== before.dev || opened.ino !== before.ino || opened.size !== before.size) {
      throw new ProbeError(`${label} changed while opening`, "bundle-changed");
    }
    const body = await handle.readFile();
    const after = await handle.stat();
    if (body.length !== before.size || after.size !== before.size || after.dev !== before.dev || after.ino !== before.ino) {
      throw new ProbeError(`${label} changed while reading`, "bundle-changed");
    }
    const actual = createHash("sha256").update(body).digest("hex");
    if (actual !== digest) throw new ProbeError(`${label} SHA256 mismatch`, "identity-mismatch");
    return { path: file, sha256: actual, bytes: body.length, mode: before.mode & 0o777, dev: before.dev, ino: before.ino, body };
  } finally {
    await handle.close();
  }
}

async function inspectBundle(options) {
  const layout = bundleLayoutForLauncher(options.bundleLauncher);
  if (options.server !== layout.server || options.native !== layout.native) {
    throw new ProbeError("bundle mode server/native paths must be the launcher's fixed resources", "bundle-path-mismatch");
  }
  for (const [label, directory] of Object.entries({
    "bundle parent": path.dirname(layout.root), "bundle root": layout.root, Contents: layout.contents,
    MacOS: layout.macOS, Resources: layout.resources,
  })) await requirePrivateDirectory(directory, label);
  const launcher = await pinnedFileIdentity(layout.launcher, options.bundleLauncherSha256, "bundle launcher", { executable: true });
  const server = await pinnedFileIdentity(layout.server, options.serverSha256, "bundle server", { executable: true });
  const native = await pinnedFileIdentity(layout.native, options.nativeSha256, "bundle native", { executable: true });
  const manifest = await pinnedFileIdentity(layout.manifest, options.bundleManifestSha256, "bundle manifest", { maxBytes: MAX_METADATA_BYTES });
  const plist = await pinnedFileIdentity(layout.plist, options.bundlePlistSha256, "bundle plist", { maxBytes: MAX_METADATA_BYTES });
  let manifestValue;
  try { manifestValue = JSON.parse(manifest.body.toString("utf8")); } catch {
    throw new ProbeError("bundle manifest is not JSON", "invalid-bundle-manifest");
  }
  for (const identity of [launcher, server, native, manifest, plist]) delete identity.body;
  return { layout, launcher, server, native, manifest, plist, manifestValue };
}

export function assertBundleCheckReceipt(value, snapshot) {
  exactKeys(value, ["checked_only", "plan"], "bundle check receipt");
  if (value.checked_only !== true) throw new ProbeError("bundle launcher did not report checked-only", "invalid-bundle-check");
  exactKeys(value.plan, ["Executable", "Args", "Manifest"], "bundle check plan");
  const expectedArgs = ["-native-host", snapshot.layout.native, "-native-host-sha256", snapshot.native.sha256];
  if (value.plan.Executable !== snapshot.layout.server || JSON.stringify(value.plan.Args) !== JSON.stringify(expectedArgs)) {
    throw new ProbeError("bundle launcher check returned a mismatched parent/companion plan", "bundle-plan-mismatch");
  }
  if (JSON.stringify(value.plan.Manifest) !== JSON.stringify(snapshot.manifestValue)) {
    throw new ProbeError("bundle launcher check returned mismatched metadata", "bundle-plan-mismatch");
  }
  exactKeys(value.plan.Manifest, ["schema_version", "mode", "kit_revision", "wails_version", "min_macos", "files"], "bundle manifest");
  if (value.plan.Manifest.schema_version !== 1 || value.plan.Manifest.mode !== "private" ||
      value.plan.Manifest.wails_version !== "v3.0.0-beta.21" ||
      !/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(value.plan.Manifest.kit_revision) ||
      !/^(?:[1-9][0-9])\.(?:0|[1-9][0-9]?)$/.test(value.plan.Manifest.min_macos) ||
      !Array.isArray(value.plan.Manifest.files) || value.plan.Manifest.files.length !== 3) {
    throw new ProbeError("bundle manifest fixed metadata is invalid", "invalid-bundle-check");
  }
  const expectedFiles = [snapshot.launcher, snapshot.server, snapshot.native];
  for (let index = 0; index < expectedFiles.length; index += 1) {
    const record = value.plan.Manifest.files[index];
    exactKeys(record, ["path", "sha256", "size"], `bundle file record ${index}`);
    const expectedPath = ["Contents/MacOS/gallery-launcher", "Contents/Resources/gallery-server", "Contents/Resources/gallery-native"][index];
    if (record.path !== expectedPath || record.sha256 !== expectedFiles[index].sha256 || record.size !== expectedFiles[index].bytes) {
      throw new ProbeError(`bundle file record ${index} does not match checked bytes`, "bundle-plan-mismatch");
    }
  }
  return value;
}

export function bundleLaunchInvocation(options, snapshot) {
  if (!options.bundleMode) throw new ProbeError("bundle launch requested outside bundle mode", "invalid-bundle-launch");
  return { command: snapshot.layout.launcher, args: [], expectedServer: snapshot.server, expectedLauncher: snapshot.launcher };
}

export function assertBundleParentTransition(invocation, pid, observed) {
  if (!Number.isInteger(pid) || pid < 2 || observed?.pid !== pid ||
      observed.path !== invocation.expectedServer.path || observed.sha256 !== invocation.expectedServer.sha256 ||
      invocation.command !== invocation.expectedLauncher.path || invocation.args.length !== 0) {
    throw new ProbeError("bundle launcher did not transition the same PID to the pinned server", "bundle-exec-transition");
  }
  return { pid, launcher: invocation.expectedLauncher, server: invocation.expectedServer, launcher_args: [] };
}

async function copyPinnedExecutable(source, destination, digest, label) {
  await copyFile(source, destination, fsConstants.COPYFILE_EXCL);
  await chmod(destination, 0o500);
  await requireExecutable(destination, `${label} copy`);
  const actual = await sha256(destination);
  if (actual !== digest) throw new ProbeError(`${label} copy SHA256 mismatch`, "copy-identity-mismatch");
  return { path: destination, sha256: actual, post_lock_sha256: actual, mode: "0500" };
}

async function copyHarnessSnapshot(destination) {
  const source = fileURLToPath(import.meta.url);
  const linkInfo = await lstat(source);
  const fileInfo = await stat(source);
  if (linkInfo.isSymbolicLink() || !fileInfo.isFile()) throw new ProbeError("executed harness is not a regular file", "invalid-harness");
  await copyFile(source, destination, fsConstants.COPYFILE_EXCL);
  const digest = await sha256(destination);
  return { path: destination, sha256: digest };
}

function sandboxPolicy() {
  const deniedReads = FLEET_SOURCE_ROOTS.map((root) => `(deny file-read* (subpath \"${root}\"))`).join("\n");
  return `(version 1)\n(allow default)\n(deny process-exec (literal \"/usr/bin/open\"))\n${deniedReads}\n`;
}

function boundedCollector(limit = MAX_PARENT_BUFFER) {
  let text = "";
  let overflow = false;
  return {
    append(chunk) {
      if (overflow) return;
      const remaining = limit - Buffer.byteLength(text);
      if (remaining <= 0) { overflow = true; return; }
      const input = Buffer.from(chunk);
      text += input.subarray(0, remaining).toString("utf8");
      if (input.length > remaining) overflow = true;
    },
    value() { return text; },
    overflow() { return overflow; },
  };
}

function spawnBounded(command, args, { env, cwd, timeoutMs = COMMAND_TIMEOUT_MS, outputLimit = MAX_PARENT_BUFFER } = {}) {
  return new Promise((resolve, reject) => {
    let child;
    try {
      child = spawn(command, args, { cwd, env, stdio: ["ignore", "pipe", "pipe"] });
    } catch (error) { reject(error); return; }
    const stdout = boundedCollector(outputLimit);
    const stderr = boundedCollector(outputLimit);
    child.stdout.on("data", (chunk) => stdout.append(chunk));
    child.stderr.on("data", (chunk) => stderr.append(chunk));
    let settled = false;
    const finish = (result) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve({ child, stdout: stdout.value(), stderr: stderr.value(), stdoutOverflow: stdout.overflow(), stderrOverflow: stderr.overflow(), ...result });
    };
    const timer = setTimeout(() => {
      try { child.kill("SIGKILL"); } catch { /* bounded command already gone */ }
      finish({ code: null, signal: "SIGKILL", timedOut: true });
    }, timeoutMs);
    child.once("error", (error) => { clearTimeout(timer); if (!settled) { settled = true; reject(error); } });
    child.once("close", (code, signal) => finish({ code, signal, timedOut: false }));
  });
}

async function inspectNoQuarantine(target, label, context) {
  context.xattrInspections ??= [];
  context.xattrSeen ??= new Set();
  for (let current = target; ; current = path.dirname(current)) {
    if (!context.xattrSeen.has(current)) {
      const result = await spawnBounded("/usr/bin/xattr", [current], {
        env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), outputLimit: 64 * 1024,
      });
      const names = result.stdout.split("\n").map((name) => name.trim()).filter(Boolean);
      const record = { label, path: current, code: result.code, signal: result.signal, timed_out: result.timedOut,
        attribute_name_count: names.length, quarantine_present: names.includes("com.apple.quarantine"),
        stdout_bytes: Buffer.byteLength(result.stdout), stderr_bytes: Buffer.byteLength(result.stderr) };
      context.xattrInspections.push(record);
      context.xattrSeen.add(current);
      if (result.timedOut || result.stdoutOverflow || result.stderrOverflow || result.code !== 0 || result.signal || result.stderr !== "") {
        throw new ProbeError(`${label} quarantine inspection failed`, "quarantine-inspection-failed");
      }
      if (record.quarantine_present) throw new ProbeError(`${label} or ancestor is quarantined`, "quarantine-refused");
    }
    if (path.dirname(current) === current) break;
  }
}

async function verifyBundleUnchanged(options, expected) {
  const current = await inspectBundle(options);
  const select = (snapshot) => ({
    layout: snapshot.layout,
    launcher: snapshot.launcher,
    server: snapshot.server,
    native: snapshot.native,
    manifest: snapshot.manifest,
    plist: snapshot.plist,
    manifestValue: snapshot.manifestValue,
  });
  if (JSON.stringify(select(current)) !== JSON.stringify(select(expected))) {
    throw new ProbeError("bundle changed after check-only validation", "bundle-changed");
  }
  return current;
}

async function checkBundleLauncher(options, snapshot, context) {
  const result = await spawnBounded(snapshot.layout.launcher, ["-check"], {
    env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), outputLimit: MAX_PARENT_BUFFER,
  });
  context.bundleCheck = { executable: snapshot.layout.launcher, args: ["-check"], code: result.code, signal: result.signal,
    timed_out: result.timedOut, stdout: result.stdout, stderr: result.stderr,
    stdout_overflow: result.stdoutOverflow, stderr_overflow: result.stderrOverflow };
  const { body, invocation } = validatedBundleInvocation(options, snapshot, result);
  context.bundleCheck.receipt = body;
  await verifyBundleUnchanged(options, snapshot);
  return invocation;
}

export function validatedBundleInvocation(options, snapshot, result) {
  if (result.timedOut || result.stdoutOverflow || result.stderrOverflow || result.code !== 0 || result.signal || result.stderr !== "") {
    throw new ProbeError("bundle launcher check-only validation failed", "bundle-check-failed");
  }
  let body;
  try { body = JSON.parse(result.stdout); } catch {
    throw new ProbeError("bundle launcher check-only output was not JSON", "invalid-bundle-check");
  }
  assertBundleCheckReceipt(body, snapshot);
  return { body, invocation: bundleLaunchInvocation(options, snapshot) };
}

async function runSandboxed(command, args, context, options = {}) {
  return spawnBounded("/usr/bin/sandbox-exec", ["-p", context.policy, command, ...args], {
    env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), ...options,
  });
}

function remainingTimeout(context, ceiling = COMMAND_TIMEOUT_MS) {
  const remaining = context.deadline - Date.now();
  if (remaining <= 0) throw new ProbeError("native delivery probe exceeded its 90-second budget", "hard-budget-exceeded");
  return Math.min(ceiling, remaining);
}

async function requireSandboxCanaries(context) {
  const open = await runSandboxed("/usr/bin/open", ["-h"], context);
  context.canaries.push({ name: "open-denied", code: open.code, signal: open.signal, timed_out: open.timedOut, stdout: open.stdout, stderr: open.stderr });
  if (open.timedOut || open.stdoutOverflow || open.stderrOverflow || open.code === null || open.code === 0 || !/EPERM|Operation not permitted/i.test(`${open.stdout}\n${open.stderr}`)) {
    throw new ProbeError("sandbox opener canary was not denied with EPERM", "sandbox-canary-failed");
  }
  const head = await runSandboxed("/usr/bin/head", ["-c", "1", "/Volumes/m2ext/code/geoffmilleraz/app-kit/go.mod"], context);
  context.canaries.push({ name: "source-read-denied", code: head.code, signal: head.signal, timed_out: head.timedOut, stdout: head.stdout, stderr: head.stderr });
  if (head.timedOut || head.stdoutOverflow || head.stderrOverflow || head.code === null || head.code === 0 || !/EPERM|Operation not permitted/i.test(`${head.stdout}\n${head.stderr}`)) {
    throw new ProbeError("sandbox source-read canary was not denied with EPERM", "sandbox-canary-failed");
  }
}

async function commandOutput(command, args, context) {
  const result = await spawnBounded(command, args, { env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), outputLimit: 4096 });
  if (result.timedOut || result.code !== 0 || result.stdoutOverflow || result.stderrOverflow) {
    throw new ProbeError(`${path.basename(command)} failed`, "command-failed");
  }
  return result.stdout.trim();
}

async function processExecutable(pid, context, label) {
  const args = ["-p", String(pid), "-o", "comm="];
  const result = await spawnBounded("/bin/ps", args, {
    env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), outputLimit: 4096,
  });
  if (result.timedOut || result.code !== 0 || result.signal || result.stdoutOverflow || result.stderrOverflow) {
    const stderr = result.stderr.replace(/[\x00-\x1f\x7f]/g, " ").slice(0, 256);
    throw new ProbeError(`${label} ps failed for pid=${pid} exit=${result.code} signal=${result.signal ?? "none"} stderr=${JSON.stringify(stderr)}`, "command-failed");
  }
  return result.stdout.trim();
}

async function directChildPIDs(parentPID, context) {
  const result = await spawnBounded("/usr/bin/pgrep", ["-P", String(parentPID)], {
    env: context.env, cwd: context.runDir, timeoutMs: remainingTimeout(context), outputLimit: 4096,
  });
  if (result.code === 1 && !result.signal && !result.timedOut && !result.stdoutOverflow && !result.stderrOverflow &&
      result.stdout.trim() === "" && result.stderr === "") return [];
  if (result.code !== 0 || result.signal || result.timedOut || result.stdoutOverflow || result.stderrOverflow || result.stderr !== "") {
    throw new ProbeError("could not enumerate direct native children", "native-child-unknown");
  }
  const pids = result.stdout.trim().split(/\s+/).filter(Boolean).map(Number);
  if (!pids.every((pid) => Number.isInteger(pid) && pid > 1) || new Set(pids).size !== pids.length) {
    throw new ProbeError("direct native child list was malformed", "native-child-unknown");
  }
  return pids;
}

async function pinnedPrivateNative(pid, parentPID, expectedDigest, context) {
  const executable = await processExecutable(pid, context, "owned child");
  if (!isAbsoluteNormalized(executable)) throw new ProbeError("owned child executable path is unavailable", "native-child-identity");
  const directory = path.dirname(executable);
  const [fileInfo, directoryInfo] = await Promise.all([lstat(executable), lstat(directory)]);
  if (fileInfo.isSymbolicLink() || !fileInfo.isFile() || (fileInfo.mode & 0o077) !== 0 ||
      directoryInfo.isSymbolicLink() || !directoryInfo.isDirectory() || (directoryInfo.mode & 0o077) !== 0 ||
      await realpath(executable) !== executable || await realpath(directory) !== directory) {
    throw new ProbeError("native child path is not private and canonical", "native-child-identity");
  }
  if (!isAbsoluteNormalized(executable) || path.basename(executable) !== "gallery-native" ||
      !path.basename(directory).startsWith("appkit-native-companion-")) {
    throw new ProbeError("native child executable path failed its identity shape", "native-child-identity");
  }
  const actual = await validatePinnedExecutable(executable, expectedDigest, "native child");
  return { pid, executable, parentPID, sha256: actual, cleanupAuthorized: true };
}

async function directNativeChild(parentPID, expectedDigest, context) {
  const pids = await directChildPIDs(parentPID, context);
  context.nativeCandidate = await pinnedPrivateNative(requireNativeChildPID(pids), parentPID, expectedDigest, context);
  return context.nativeCandidate;
}

export function requireNativeChildPID(pids) {
  if (pids.length === 0) throw new ProbeError("native child has not started yet", "native-child-pending");
  if (pids.length !== 1) throw new ProbeError("server did not own exactly one direct native child", "native-child-missing");
  return pids[0];
}

function assertServerHealthy(context) {
  if (context.serverExit) throw new ProbeError("server exited unexpectedly", "server-exited");
  if (context.serverStdout?.overflow() || context.serverStderr?.overflow()) {
    throw new ProbeError("server diagnostics exceeded 64KiB", "server-output-overflow");
  }
}

async function waitFor(predicate, deadline, description) {
  while (Date.now() < deadline) {
    const result = await predicate();
    if (result) return result;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new ProbeError(description, "timeout");
}

async function runDriver(driver, operation, native, context, finalClose = false) {
  if (!["inspect", "new-window", "close-window"].includes(operation)) {
    throw new ProbeError(`driver operation ${operation} is not permitted by this harness`, "forbidden-driver-operation");
  }
  if (finalClose && operation !== "close-window") throw new ProbeError("only last close may race normal parent exit", "invalid-final-close");
  assertServerHealthy(context);
  const result = await runSandboxed(driver, [String(native.pid), native.executable, operation], context, { outputLimit: MAX_DRIVER_OUTPUT });
  const transcript = { operation, code: result.code, signal: result.signal, timed_out: result.timedOut, stdout: result.stdout, stderr: result.stderr };
  context.driverReceipts.push(transcript);
  if (result.timedOut || result.stdoutOverflow || result.stderrOverflow || Buffer.byteLength(result.stdout) > MAX_DRIVER_OUTPUT) {
    throw new ProbeError(`driver ${operation} exceeded its bounded contract`, "driver-failed");
  }
  let receipt;
  try { receipt = JSON.parse(result.stdout); } catch { throw new ProbeError(`driver ${operation} did not emit JSON`, "driver-failed"); }
  if (result.code !== 0) throw new ProbeError(`driver ${operation} failed: ${receipt?.error ?? "unknown"}`, "driver-failed");
  transcript.observed_state = assertStrictState(receipt, operation, native.pid);
  if (!finalClose) assertServerHealthy(context);
  return transcript.observed_state;
}

export function assertWindowState(receipt, { windows, newWindowEnabled, browserEnabled = true }) {
  if (receipt.windows !== windows || receipt.new_window_enabled !== newWindowEnabled || receipt.browser_enabled !== browserEnabled) {
    throw new ProbeError(`unexpected native state: windows=${receipt.windows}, new_window_enabled=${receipt.new_window_enabled}`, "unexpected-native-state");
  }
}

async function listenerRefuses(port, context) {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host: "127.0.0.1", port });
    const timer = setTimeout(() => { socket.destroy(); reject(new ProbeError("listener refusal probe timed out", "listener-still-live")); }, remainingTimeout(context));
    socket.once("connect", () => { clearTimeout(timer); socket.destroy(); reject(new ProbeError("listener still accepted TCP", "listener-still-live")); });
    socket.once("error", (error) => {
      clearTimeout(timer);
      if (error.code === "ECONNREFUSED") resolve(true);
      else reject(new ProbeError(`listener refusal had unexpected error ${error.code}`, "listener-refusal-error"));
    });
  });
}

export function isProcessAbsentError(error) {
  return error?.code === "ESRCH";
}

export async function processAbsent(pid) {
  try {
    process.kill(pid, 0);
    return false;
  } catch (error) {
    if (isProcessAbsentError(error)) return true;
    throw new ProbeError(`cannot establish process ${pid} absence: ${error.code ?? error.message}`, "process-absence-unknown");
  }
}

async function assertCurrentNativeIdentity(native, expectedDigest, context) {
  const executable = await processExecutable(native.pid, context, "native child cleanup");
  if (executable !== native.executable) throw new ProbeError("native PID no longer has the recorded executable path", "native-pid-reused");
  await validatePinnedExecutable(executable, expectedDigest, "native child before cleanup");
}

async function currentPinnedProcess(pid, permitted, context, label) {
  const executable = await processExecutable(pid, context, label);
  const expected = permitted.find((identity) => identity.path === executable);
  if (!expected) {
    const error = new ProbeError(`${label} PID no longer has a permitted executable path`, "process-pid-reused");
    error.observedExecutable = executable;
    throw error;
  }
  await validatePinnedExecutable(executable, expected.sha256, `${label} process`);
  return { pid, path: executable, sha256: expected.sha256 };
}

async function authorizeNativeCleanup(native, context) {
  if (await processAbsent(native.pid)) return false;
  const children = await directChildPIDs(native.parentPID, context);
  if (!children.includes(native.pid)) throw new ProbeError("native child is no longer directly owned by the launched server", "native-cleanup-unproven");
  await assertCurrentNativeIdentity(native, native.sha256, context);
  return true;
}

async function stopKnownProcess(pid, context, identity) {
  if (!pid || await processAbsent(pid)) return;
  if (identity) await currentPinnedProcess(pid, identity.permitted, context, identity.label);
  try { process.kill(pid, "SIGTERM"); } catch (error) {
    if (isProcessAbsentError(error)) return;
    throw new ProbeError(`SIGTERM failed for process ${pid}: ${error.code ?? error.message}`, "cleanup-failed");
  }
  try { await waitFor(() => processAbsent(pid), Date.now() + 2_000, `process ${pid} did not exit after SIGTERM`); return; } catch (error) {
    if (error.code !== "timeout") throw error;
  }
  if (identity) await currentPinnedProcess(pid, identity.permitted, context, identity.label);
  try { process.kill(pid, "SIGKILL"); } catch (error) {
    if (isProcessAbsentError(error)) return;
    throw new ProbeError(`SIGKILL failed for process ${pid}: ${error.code ?? error.message}`, "cleanup-failed");
  }
  await waitFor(() => processAbsent(pid), Date.now() + 2_000, `process ${pid} did not exit after SIGKILL`);
}

async function stopAuthorizedNative(native, context) {
  if (await processAbsent(native.pid)) return;
  if (!native.cleanupAuthorized) throw new ProbeError("native cleanup was not authorized by direct server ancestry", "native-cleanup-unproven");
  await assertCurrentNativeIdentity(native, native.sha256, context);
  await stopKnownProcess(native.pid, context, { label: "native child", permitted: [{ path: native.executable, sha256: native.sha256 }] });
}

// Test seam for cleanup paths that fail before ordinary native discovery.
export async function cleanupEarlyOwnedProcesses(parentIdentity, expectedNativeDigest, context) {
  if (!parentIdentity?.pid || await processAbsent(parentIdentity.pid)) {
    return { parent_absent: true, native_absent: null, native: null, reason: "parent-exited-before-child-observation" };
  }
  await currentPinnedProcess(parentIdentity.pid, parentIdentity.permitted, context, parentIdentity.label);
  const children = await directChildPIDs(parentIdentity.pid, context);
  if (children.length > 1) throw new ProbeError("early cleanup found multiple direct children", "native-child-unknown");
  const native = children.length === 1 ? await pinnedPrivateNative(children[0], parentIdentity.pid, expectedNativeDigest, context) : undefined;
  if (native) {
    const confirmed = await directChildPIDs(parentIdentity.pid, context);
    if (confirmed.length !== 1 || confirmed[0] !== native.pid) {
      throw new ProbeError("native child ancestry changed before cleanup", "native-cleanup-unproven");
    }
  }
  await stopKnownProcess(parentIdentity.pid, context, parentIdentity);
  if (native) await stopAuthorizedNative(native, context);
  return { parent_absent: await processAbsent(parentIdentity.pid), native_absent: native ? await processAbsent(native.pid) : true,
    native: native ? { pid: native.pid, executable: native.executable, sha256: native.sha256 } : null };
}

export function finalOutcome(scenarioSucceeded, cleanupError) {
  return scenarioSucceeded && !cleanupError ? "passed" : "failed";
}

async function writeJSON(file, value) {
  await writeFile(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
}

export async function run(argv = process.argv.slice(2)) {
  if (process.platform !== "darwin") throw new ProbeError("native delivery probe requires darwin", "unsupported-platform");
  const options = parseArgs(argv);
  const outputParent = await requireAbsoluteDirectory(options.outputParent, "--output-parent");
  const runDir = await mkdtemp(path.join(outputParent, "appkit-native-delivery-"));
  await mkdir(path.join(runDir, "tmp"), { mode: 0o700 });
  const context = { runDir, policy: sandboxPolicy(), env: { PATH: "/usr/bin:/bin:/usr/sbin:/sbin", TMPDIR: path.join(runDir, "tmp") }, deadline: Date.now() + HARD_BUDGET_MS, driverReceipts: [], canaries: [] };
  const receipts = { started_at: new Date().toISOString(), run_dir: runDir, outcome: "failed", events: [] };
  let serverChild;
  let verifiedNative;
  let serverResult;
  let serverStdout;
  let serverStderr;
  try {
    const bundle = options.bundleMode ? await inspectBundle(options) : undefined;
    await Promise.all(options.bundleMode ? [
      validatePinnedExecutable(options.driver, options.driverSha256, "driver"),
    ] : [
      validatePinnedExecutable(options.server, options.serverSha256, "server"),
      validatePinnedExecutable(options.native, options.nativeSha256, "native"),
      validatePinnedExecutable(options.driver, options.driverSha256, "driver"),
    ]);
    const harness = await copyHarnessSnapshot(path.join(runDir, "native_delivery_probe.mjs"));
    const inputConfig = { mode: options.bundleMode ? "bundle" : "standalone", server: { path: options.server, sha256: options.serverSha256 },
      native: { path: options.native, sha256: options.nativeSha256 }, driver: { path: options.driver, sha256: options.driverSha256 },
      ...(bundle ? { bundle: { layout: bundle.layout, launcher: bundle.launcher, manifest: bundle.manifest, plist: bundle.plist } } : {}), harness };
    await writeFile(path.join(runDir, "sandbox.sb"), context.policy, { mode: 0o600 });
    await writeJSON(path.join(runDir, "config.json"), { inputs: inputConfig, policy: "allow-default; deny-open; deny-fleet-source-reads", env: context.env });
    receipts.inputs = inputConfig;
    receipts.xattr_inspections = context.xattrInspections = [];
    await writeJSON(path.join(runDir, "receipt.json"), receipts);
    await inspectNoQuarantine(options.driver, "driver source", context);
    if (bundle) await inspectNoQuarantine(bundle.layout.launcher, "bundle launcher source", context);

    const staged = {
      server: bundle?.server ?? await copyPinnedExecutable(options.server, path.join(runDir, "gallery-server"), options.serverSha256, "server"),
      native: bundle?.native ?? await copyPinnedExecutable(options.native, path.join(runDir, "gallery-native-input"), options.nativeSha256, "native"),
      driver: await copyPinnedExecutable(options.driver, path.join(runDir, "native-ui-driver"), options.driverSha256, "driver"),
      harness,
    };
    receipts.staged = staged;
    await writeJSON(path.join(runDir, "config.json"), { inputs: inputConfig, staged, policy: "allow-default; deny-open; deny-fleet-source-reads", env: context.env });
    receipts.canaries = context.canaries;
    await requireSandboxCanaries(context);
    receipts.events.push("sandbox-canaries-refused");

    const invocation = bundle ? await checkBundleLauncher(options, bundle, context) : {
      command: staged.server.path, args: ["-native-host", staged.native.path, "-native-host-sha256", staged.native.sha256],
      expectedServer: staged.server,
    };
    if (bundle) {
      receipts.bundle_check = context.bundleCheck;
      receipts.events.push("bundle-check-only-passed");
    }
    context.serverIdentity = { label: "HTTP parent", permitted: bundle ? [invocation.expectedLauncher, invocation.expectedServer] : [invocation.expectedServer] };
    serverResult = spawn("/usr/bin/sandbox-exec", ["-p", context.policy, invocation.command, ...invocation.args], { cwd: runDir, env: context.env, stdio: ["ignore", "pipe", "pipe"] });
    serverChild = serverResult;
    serverStdout = boundedCollector(MAX_PARENT_BUFFER);
    serverStderr = boundedCollector(MAX_PARENT_BUFFER);
    context.serverStdout = serverStdout;
    context.serverStderr = serverStderr;
    serverResult.stdout.on("data", (chunk) => serverStdout.append(chunk));
    serverResult.stderr.on("data", (chunk) => serverStderr.append(chunk));
    const exit = new Promise((resolve) => serverResult.once("close", (code, signal) => { context.serverExit = { code, signal }; resolve(context.serverExit); }));
    serverResult.once("error", (error) => { context.serverExit = { code: null, signal: null, error: error.message }; });
    const deadline = context.deadline;
    if (bundle) {
      const transition = await waitFor(async () => {
        assertServerHealthy(context);
        let observed;
        try { observed = await currentPinnedProcess(serverResult.pid, context.serverIdentity.permitted, context, "bundle parent"); }
        catch (error) {
          if (error.code === "process-pid-reused" && error.observedExecutable === "/usr/bin/sandbox-exec") return null;
          throw error;
        }
        if (observed.path === invocation.expectedLauncher.path) return null;
        return assertBundleParentTransition(invocation, serverResult.pid, observed);
      }, deadline, "bundle launcher-to-server PID transition");
      receipts.bundle_parent_transition = transition;
      receipts.events.push("bundle-launcher-exec-transition-verified");
    }
    const port = await waitFor(() => {
      assertServerHealthy(context);
      try { return parseServerDiagnostic(`${serverStdout.value()}\n${serverStderr.value()}`); } catch { return null; }
    }, deadline, "server startup");
    receipts.port = port;
    verifiedNative = await waitFor(() => {
      assertServerHealthy(context);
      return directNativeChild(serverResult.pid, staged.native.sha256, context).catch((error) => {
        if (error.code === "native-child-pending") return null;
        throw error;
      });
    }, deadline, "verified native child discovery");
    verifiedNative.sha256 = staged.native.sha256;
    verifiedNative.parentPID = serverResult.pid;
    receipts.native = verifiedNative;
    receipts.events.push("server-and-native-started");
    await waitFor(() => {
      assertServerHealthy(context);
      const diagnostic = serverStderr.value();
      if (diagnostic.includes("Native companion unavailable;")) throw new ProbeError("native startup entered recovery", "native-startup-failed");
      return diagnostic.includes("Native companion navigation acknowledged (not a task or platform certification).");
    }, deadline, "native navigation acknowledgement");
    receipts.events.push("native-navigation-acknowledged");

    const waitForState = async (operation, expected, description) => waitFor(async () => {
      assertServerHealthy(context);
      const state = await runDriver(staged.driver.path, operation, verifiedNative, context);
      return state.windows === expected.windows && state.new_window_enabled === expected.newWindowEnabled && state.browser_enabled === (expected.browserEnabled ?? true) ? state : null;
    }, deadline, description);
    assertWindowState(await waitForState("inspect", { windows: 1, newWindowEnabled: true }, "initial native window"), { windows: 1, newWindowEnabled: true });
    assertWindowState(await runDriver(staged.driver.path, "new-window", verifiedNative, context), { windows: 1, newWindowEnabled: true });
    assertWindowState(await waitForState("inspect", { windows: 2, newWindowEnabled: false }, "second native window"), { windows: 2, newWindowEnabled: false });
    assertWindowState(await runDriver(staged.driver.path, "close-window", verifiedNative, context), { windows: 2, newWindowEnabled: false });
    assertWindowState(await waitForState("inspect", { windows: 1, newWindowEnabled: true }, "first close result"), { windows: 1, newWindowEnabled: true });
    assertWindowState(await runDriver(staged.driver.path, "new-window", verifiedNative, context), { windows: 1, newWindowEnabled: true });
    assertWindowState(await waitForState("inspect", { windows: 2, newWindowEnabled: false }, "second reopen"), { windows: 2, newWindowEnabled: false });
    assertWindowState(await runDriver(staged.driver.path, "close-window", verifiedNative, context), { windows: 2, newWindowEnabled: false });
    assertWindowState(await waitForState("inspect", { windows: 1, newWindowEnabled: true }, "second close result"), { windows: 1, newWindowEnabled: true });
    assertWindowState(await runDriver(staged.driver.path, "close-window", verifiedNative, context, true), { windows: 1, newWindowEnabled: true });
    const finalExit = await Promise.race([exit, new Promise((_, reject) => setTimeout(() => reject(new ProbeError("server/native did not exit after last window close", "shutdown-timeout")), remainingTimeout(context)))]);
    if (finalExit.code !== 0 || finalExit.signal || finalExit.error) throw new ProbeError("server/native did not exit cleanly after last close", "shutdown-failed");
    await waitFor(() => processAbsent(verifiedNative.pid), deadline, "native child absence after normal close");
    if (serverStdout.overflow() || serverStderr.overflow()) throw new ProbeError("server diagnostics exceeded 64KiB", "server-output-overflow");
    await listenerRefuses(port, context);
    receipts.parent_exit = finalExit;
    receipts.native_absent_before_cleanup = true;
    receipts.events.push("last-window-closed-server-exited-listener-refused");
    receipts.scenario_succeeded = true;
    return { runDir, receipts };
  } catch (error) {
    receipts.error = { code: error.code ?? "unexpected", message: error.message };
    throw error;
  } finally {
    // Cleanup has its own small bound; an exhausted task deadline must not
    // prevent identity rechecks or leave the owned processes running.
    const cleanupContext = { ...context, deadline: Date.now() + 15_000 };
    const cleanupNative = verifiedNative ?? context.nativeCandidate;
    let cleanupError;
    if (!cleanupNative && serverChild?.pid) {
      try {
        receipts.early_cleanup = await cleanupEarlyOwnedProcesses({ ...context.serverIdentity, pid: serverChild.pid }, options.nativeSha256, cleanupContext);
      } catch (error) { cleanupError = error; receipts.early_cleanup_error = error.message; }
    } else {
      try { if (cleanupNative && !cleanupNative.cleanupAuthorized && serverChild?.pid && !await processAbsent(serverChild.pid)) cleanupNative.cleanupAuthorized = await authorizeNativeCleanup(cleanupNative, cleanupContext); } catch (error) { cleanupError = error; receipts.native_cleanup_authorization_error = error.message; }
      try { if (serverChild?.pid && serverChild.exitCode === null && serverChild.signalCode === null) await stopKnownProcess(serverChild.pid, cleanupContext, context.serverIdentity); } catch (error) { cleanupError = error; receipts.server_cleanup_error = error.message; }
      try { if (cleanupNative) await stopAuthorizedNative(cleanupNative, cleanupContext); } catch (error) { cleanupError ??= error; receipts.native_cleanup_error = error.message; }
    }
    receipts.outcome = finalOutcome(receipts.scenario_succeeded === true, cleanupError);
    if (context.bundleCheck) receipts.bundle_check ??= context.bundleCheck;
    if (cleanupError) receipts.error ??= { code: cleanupError.code ?? "cleanup-failed", message: cleanupError.message };
    receipts.finished_at = new Date().toISOString();
    if (serverResult) {
      await writeFile(path.join(runDir, "server-stdout.txt"), serverStdout?.value() ?? "", { mode: 0o600 });
      await writeFile(path.join(runDir, "server-stderr.txt"), serverStderr?.value() ?? "", { mode: 0o600 });
      receipts.server_output = { stdout_overflow: serverStdout?.overflow() ?? false, stderr_overflow: serverStderr?.overflow() ?? false };
      receipts.observed_parent_exit = context.serverExit ?? null;
    }
    await writeJSON(path.join(runDir, "driver-receipts.json"), context.driverReceipts);
    await writeJSON(path.join(runDir, "receipt.json"), receipts);
    if (cleanupError) throw cleanupError;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  run().then(({ runDir }) => {
    process.stdout.write(`${JSON.stringify({ ok: true, run_dir: runDir })}\n`);
  }).catch((error) => {
    process.stderr.write(`native delivery probe failed [${error.code ?? "unexpected"}]: ${error.message}\n`);
    process.exitCode = 1;
  });
}
