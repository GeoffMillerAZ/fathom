#!/usr/bin/env node
// Bounded normal-environment LaunchServices evidence for an already-built private bundle.
// Importing this module is inert; only the explicit CLI guard invokes /usr/bin/open.
import { createHash } from "node:crypto";
import { constants as fsConstants } from "node:fs";
import { access, chmod, copyFile, lstat, mkdtemp, open, realpath, writeFile } from "node:fs/promises";
import net from "node:net";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";
import {
  ProbeError,
  assertBundleCheckReceipt,
  assertStrictState,
  assertWindowState,
  bundleLayoutForLauncher,
  parseArgs,
  processAbsent,
} from "./native_delivery_probe.mjs";

const COMMAND_MS = 5_000;
const HARD_MS = 90_000;
const CLEANUP_MS = 15_000;
const MAX_OUTPUT = 64 * 1024;
const MAX_RECORDED_OUTPUT = 256;
const MAX_CALLS = 4096;
const MAX_STATES = 768;
const MAX_EXECUTABLE = 256 * 1024 * 1024;
const MAX_METADATA = 64 * 1024;
const MAX_SOURCE = 1024 * 1024;
const HELPER = fileURLToPath(new URL("./native_delivery_probe.mjs", import.meta.url));
const SELF = fileURLToPath(import.meta.url);
const WORKFLOW_DRIVER = fileURLToPath(new URL("./native-workflow-driver", import.meta.url));
const WORKFLOW_DRIVER_SOURCE = fileURLToPath(new URL("./native_workflow_driver.swift", import.meta.url));
const WORKFLOW_DRIVER_TEST_SOURCE = fileURLToPath(new URL("./native_workflow_driver_test.swift", import.meta.url));
const WORKFLOW_DRIVER_CHECKS = fileURLToPath(new URL("./workflow-driver-checks.json", import.meta.url));
const WORKFLOW_DRIVER_CONFIG = fileURLToPath(new URL("./workflow-driver-config.json", import.meta.url));
const WORKFLOW_DRIVER_SHA256 = "c3c8fdea26158ac2400537c91b1159745cbc7b332c8d51ea430bae7cd7873e10";
const WORKFLOW_DRIVER_SOURCE_SHA256 = "f91b08749630de5122966f4d88c5ab7378cfd539825597beba2b121386ac5fbb";
const WORKFLOW_DRIVER_TEST_SHA256 = "051df6408c8b09b95b80996039810482bfd254c3457b11ff134969fa85544e72";
const WORKFLOW_DRIVER_CHECKS_SHA256 = "3e2ec6d9f564e645fffee51afb499e8ed1a264d8897c4d0cab3f30b43ee7866c";
const WORKFLOW_DRIVER_CONFIG_SHA256 = "e0f6ed967f1465686ee7a89cc91ddb8870736efeab4f1772a14493f8dd1eeecf";
const MAX_WORKFLOW_OUTPUT = 128 * 1024;
const WORKFLOW_COMMAND_MS = 60_000;

function requireFact(value, message, code = "probe-failed") {
  if (!value) throw new ProbeError(message, code);
  return value;
}

async function digest(file, maxBytes = MAX_EXECUTABLE) {
  const before = await lstat(file);
  requireFact(before.isFile() && !before.isSymbolicLink() && before.size > 0 && before.size <= maxBytes,
    `invalid bounded file ${file}`, "invalid-file");
  const handle = await open(file, "r");
  try {
    const opened = await handle.stat();
    requireFact(opened.dev === before.dev && opened.ino === before.ino && opened.size === before.size,
      `file changed while opening ${file}`, "identity-changed");
    const body = await handle.readFile();
    const after = await handle.stat();
    requireFact(body.length === before.size && after.dev === before.dev && after.ino === before.ino && after.size === before.size,
      `file changed while reading ${file}`, "identity-changed");
    return createHash("sha256").update(body).digest("hex");
  } finally { await handle.close(); }
}

async function requirePrivateDirectory(directory, label) {
  requireFact(path.isAbsolute(directory) && path.normalize(directory) === directory, `${label} is not absolute and normalized`, "invalid-path");
  const info = await lstat(directory);
  requireFact(info.isDirectory() && !info.isSymbolicLink() && await realpath(directory) === directory,
    `${label} is not a real directory`, "invalid-path");
  requireFact(info.uid === process.getuid() && (info.mode & 0o077) === 0, `${label} is not private and owned`, "invalid-mode");
}

async function pinnedFile(file, expected, label, {
  executable = false, maxBytes = MAX_EXECUTABLE, privateMode = true, retainBody = false,
} = {}) {
  requireFact(path.isAbsolute(file) && path.normalize(file) === file, `${label} path is not absolute and normalized`, "invalid-path");
  const before = await lstat(file);
  requireFact(before.isFile() && !before.isSymbolicLink() && await realpath(file) === file, `${label} is not a real file`, "invalid-path");
  requireFact(before.uid === process.getuid(), `${label} has another owner`, "invalid-owner");
  if (privateMode) requireFact((before.mode & 0o077) === 0, `${label} is not private`, "invalid-mode");
  if (executable) await access(file, fsConstants.X_OK);
  requireFact(before.size > 0 && before.size <= maxBytes, `${label} has invalid size`, "invalid-size");
  const handle = await open(file, "r");
  try {
    const opened = await handle.stat();
    requireFact(opened.dev === before.dev && opened.ino === before.ino && opened.size === before.size,
      `${label} changed while opening`, "identity-changed");
    const body = await handle.readFile();
    const after = await handle.stat();
    requireFact(body.length === before.size && after.dev === before.dev && after.ino === before.ino && after.size === before.size,
      `${label} changed while reading`, "identity-changed");
    const actual = createHash("sha256").update(body).digest("hex");
    requireFact(actual === expected, `${label} SHA256 mismatch`, "identity-mismatch");
    const identity = { path: file, sha256: actual, bytes: body.length, mode: before.mode & 0o777,
      uid: before.uid, dev: before.dev, ino: before.ino };
    if (retainBody) identity.body = body;
    return identity;
  } finally { await handle.close(); }
}

function remaining(context, cap = COMMAND_MS) {
  return Math.max(1, Math.min(cap, context.deadline - Date.now()));
}

function command(context, file, args, { allowOne = false, inheritEnvironment = false,
  timeoutMs = COMMAND_MS, maxOutput = MAX_OUTPUT } = {}) {
  requireFact(Date.now() < context.deadline, "probe deadline exceeded", "timeout");
  const result = spawnSync(file, args, {
    cwd: context.runDir, encoding: "utf8", timeout: remaining(context, timeoutMs), maxBuffer: maxOutput,
    ...(inheritEnvironment ? {} : { env: { PATH: "/usr/bin:/bin:/usr/sbin:/sbin", LANG: "C" } }),
  });
  const item = { file, args, exit: result.status, signal: result.signal, error: result.error?.message ?? null,
    stdout: result.stdout ?? "", stderr: result.stderr ?? "" };
  requireFact(context.calls.length < MAX_CALLS, "command receipt limit exceeded", "receipt-overflow");
  const recordOutput = (value) => Buffer.byteLength(value) <= MAX_RECORDED_OUTPUT ? value : null;
  context.calls.push({ ...item, stdout: recordOutput(item.stdout), stderr: recordOutput(item.stderr),
    stdout_bytes: Buffer.byteLength(item.stdout), stderr_bytes: Buffer.byteLength(item.stderr),
    stdout_sha256: createHash("sha256").update(item.stdout).digest("hex"),
    stderr_sha256: createHash("sha256").update(item.stderr).digest("hex") });
  requireFact(!result.error && !result.signal && (result.status === 0 || (allowOne && result.status === 1)),
    `${path.basename(file)} failed`, "command-failed");
  requireFact(Buffer.byteLength(item.stdout) <= maxOutput && Buffer.byteLength(item.stderr) <= maxOutput,
    `${path.basename(file)} output overflow`, "command-output-overflow");
  return item;
}

function parsePIDs(result, label) {
  requireFact(result.stderr === "", `${label} emitted diagnostics`, "process-discovery-refused");
  if (result.exit === 1) {
    requireFact(result.stdout === "" || result.stdout.trim() === "", `${label} failure had output`, "process-discovery-refused");
    return [];
  }
  const values = result.stdout.trim().split(/\s+/).filter(Boolean).map(Number);
  requireFact(values.every((pid) => Number.isInteger(pid) && pid > 1) && new Set(values).size === values.length,
    `${label} returned invalid PIDs`, "process-discovery-refused");
  return values;
}

export function parseListenerFields(stdout) {
  requireFact(typeof stdout === "string" && Buffer.byteLength(stdout) <= MAX_OUTPUT, "invalid lsof output", "listener-refused");
  const lines = stdout.split("\n").filter(Boolean);
  requireFact(lines.every((line) => /^(?:p\d+|f\d+|n[^\n]+)$/.test(line)), "malformed lsof field output", "listener-refused");
  const names = lines.filter((line) => line.startsWith("n")).map((line) => line.slice(1));
  requireFact(names.length === 1, "expected exactly one listener", "listener-refused");
  const match = /^127\.0\.0\.1:([1-9]\d{0,4})$/.exec(names[0]);
  requireFact(match, "listener is not exact IPv4 loopback", "listener-refused");
  const port = Number(match[1]);
  requireFact(port <= 65535, "listener port is out of range", "listener-refused");
  return { address: names[0], port };
}

export async function resolveObservedExecutable(observed, adapters = { realpath }) {
  requireFact(typeof observed === "string" && path.isAbsolute(observed) && path.normalize(observed) === observed,
    "observed executable path is invalid", "process-identity-refused");
  const canonical = await adapters.realpath(observed);
  if (canonical === observed) return { observed, canonical, alias: null };
  requireFact(observed.startsWith("/var/"), "arbitrary executable alias refused", "process-identity-refused");
  requireFact(await adapters.realpath("/var") === "/private/var", "unexpected /var system alias", "process-identity-refused");
  requireFact(canonical === `/private${observed}`, "observed /var path has unexpected canonical target", "process-identity-refused");
  return { observed, canonical, alias: "/var->/private/var" };
}

async function processPath(context, pid) {
  const result = command(context, "/bin/ps", ["-p", String(pid), "-o", "comm="], { allowOne: true });
  requireFact(result.stderr === "", "ps emitted diagnostics", "process-discovery-refused");
  if (result.exit === 1) {
    requireFact(result.stdout.trim() === "", "ps failure had output", "process-discovery-refused");
    return null;
  }
  requireFact(result.stdout.trim() !== "", "ps returned an empty executable", "process-discovery-refused");
  return result.stdout.trim();
}

async function processStart(context, pid) {
  const result = command(context, "/bin/ps", ["-p", String(pid), "-o", "lstart="], { allowOne: true });
  requireFact(result.exit === 0 && result.stderr === "" && result.stdout.trim().length > 0 && result.stdout.trim().length <= 128,
    "process start stamp is unknown", "process-generation-refused");
  return result.stdout.trim();
}

export function assertProcessGeneration(identity, observedStart) {
  requireFact(typeof identity?.started === "string" && observedStart === identity.started,
    `${identity?.label ?? "process"} PID generation changed`, "process-pid-reused");
}

function namedPIDs(context, name) {
  return parsePIDs(command(context, "/usr/bin/pgrep", ["-x", name], { allowOne: true }), `pgrep ${name}`);
}

function directChildren(context, pid) {
  return parsePIDs(command(context, "/usr/bin/pgrep", ["-P", String(pid)], { allowOne: true }), `pgrep children of ${pid}`);
}

async function currentParentCandidates(context, snapshot, baseline) {
  const candidates = [];
  for (const pid of namedPIDs(context, "gallery-server")) {
    const executable = await processPath(context, pid);
    if (executable === snapshot.server.path && !baseline.has(pid)) candidates.push({ pid, observed: executable });
  }
  requireFact(candidates.length <= 1, "multiple new exact-bundle parents", "parent-discovery-refused");
  return candidates;
}

async function bindParent(context, candidate, snapshot) {
  requireFact(candidate && await processPath(context, candidate.pid) === snapshot.server.path,
    "parent executable changed", "parent-identity-refused");
  await pinnedFile(snapshot.server.path, snapshot.server.sha256, "running bundle server", { executable: true });
  return { pid: candidate.pid, observed: snapshot.server.path, canonical: snapshot.server.path,
    sha256: snapshot.server.sha256, started: await processStart(context, candidate.pid),
    label: "LaunchServices parent", identityBound: true };
}

async function bindNative(context, parent, expectedDigest) {
  const children = directChildren(context, parent.pid);
  if (children.length === 0) return null; // startup is still pending
  requireFact(children.length === 1, "multiple direct native children", "native-discovery-refused");
  const pid = children[0];
  const observed = await processPath(context, pid);
  requireFact(observed, "native child executable is unknown", "native-discovery-refused");
  const resolved = await resolveObservedExecutable(observed);
  requireFact(path.basename(observed) === "gallery-native" && path.basename(path.dirname(observed)).startsWith("appkit-native-companion-"),
    "unexpected native child path", "native-discovery-refused");
  const directory = await lstat(path.dirname(observed));
  const file = await lstat(observed);
  requireFact(directory.isDirectory() && !directory.isSymbolicLink() && directory.uid === process.getuid() && (directory.mode & 0o077) === 0,
    "native child directory is not private and owned", "native-identity-refused");
  requireFact(file.isFile() && !file.isSymbolicLink() && file.uid === process.getuid() && (file.mode & 0o077) === 0,
    "native child file is not private and owned", "native-identity-refused");
  requireFact(await digest(observed) === expectedDigest, "native child digest mismatch", "native-identity-refused");
  const ancestry = command(context, "/bin/ps", ["-p", String(pid), "-o", "ppid="]);
  requireFact(ancestry.stderr === "" && Number(ancestry.stdout.trim()) === parent.pid,
    "native child is not a direct child", "native-identity-refused");
  return { pid, ...resolved, sha256: expectedDigest, started: await processStart(context, pid), parentPID: parent.pid,
    label: "native child", identityBound: true,
    mode: file.mode & 0o777, uid: file.uid, directory_mode: directory.mode & 0o777, directory_uid: directory.uid };
}

async function waitFor(probe, context, label, end = context.deadline) {
  while (Date.now() < end) {
    const value = await probe();
    if (value) return value;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new ProbeError(`timeout waiting for ${label}`, "timeout");
}

async function assertBoundIdentity(identity, context) {
  const observed = await processPath(context, identity.pid);
  requireFact(observed === identity.observed, `${identity.label} PID changed executable spelling`, "process-pid-reused");
  assertProcessGeneration(identity, await processStart(context, identity.pid));
  const resolved = await resolveObservedExecutable(observed);
  requireFact(resolved.canonical === identity.canonical && await digest(observed) === identity.sha256,
    `${identity.label} PID changed identity`, "process-pid-reused");
}

async function stopBound(identity, context) {
  if (!identity || await processAbsent(identity.pid)) return;
  requireFact(identity.identityBound, `${identity.label} is not identity-bound`, "cleanup-unproven");
  await assertBoundIdentity(identity, context);
  process.kill(identity.pid, "SIGTERM");
  try { await waitFor(() => processAbsent(identity.pid), context, `${identity.label} SIGTERM`, Date.now() + 3_000); return; } catch (error) {
    if (error.code !== "timeout") throw error;
  }
  if (await processAbsent(identity.pid)) return;
  await assertBoundIdentity(identity, context);
  process.kill(identity.pid, "SIGKILL");
  await waitFor(() => processAbsent(identity.pid), context, `${identity.label} SIGKILL`, Date.now() + 2_000);
}

export async function cleanupOwnedProcesses({ parent, native }, adapters = {}) {
  const absent = adapters.absent ?? processAbsent;
  const stop = adapters.stop ?? stopBound;
  const context = adapters.context;
  const result = { parent_absent: parent ? null : null, native_absent: native?.identityBound ? null : null };
  const errors = [];
  let nativeAuthorized = true;
  if (adapters.authorizePair && parent?.identityBound && native?.identityBound) {
    try {
      if (!await absent(native.pid)) {
        if (await absent(parent.pid)) throw new ProbeError("parent exited before native ancestry reauthorization", "native-cleanup-unproven");
        await adapters.authorizePair(parent, native);
      }
    } catch (error) {
        nativeAuthorized = false;
        errors.push({ process: "native", code: error.code ?? "native-cleanup-unproven", message: error.message });
    }
  }
  if (parent?.identityBound) {
    try { await stop(parent, context); result.parent_absent = await absent(parent.pid); }
    catch (error) { errors.push({ process: "parent", code: error.code ?? "cleanup-failed", message: error.message }); }
  }
  if (native?.identityBound && nativeAuthorized) {
    try { await stop(native, context); result.native_absent = await absent(native.pid); }
    catch (error) { errors.push({ process: "native", code: error.code ?? "cleanup-failed", message: error.message }); }
  } else if (native?.identityBound) {
    result.native_absent = null;
    result.native_reason = "native ancestry could not be reauthorized; process was not signaled";
  } else {
    result.native_absent = null;
    result.native_reason = "native identity was not bound; absence remains unknown";
  }
  if (errors.length) {
    const error = new ProbeError("one or more identity-bound cleanup attempts failed", "cleanup-failed");
    error.details = { ...result, errors };
    throw error;
  }
  return result;
}

export async function recoverCleanupBindings(state, adapters) {
  let { parent, native } = state;
  const errors = [];
  if (state.launchRequested && !parent && state.snapshot) {
    try {
      const candidate = await adapters.findParent();
      if (candidate) parent = await adapters.bindParent(candidate);
      else errors.push({ process: "parent", code: "parent-not-observed", message: "launch-owned parent was not observed for cleanup" });
    } catch (error) { errors.push({ process: "parent", code: error.code ?? "cleanup-discovery-failed", message: error.message }); }
  }
  if (parent?.identityBound && !native) {
    try {
      if (!await adapters.absent(parent.pid)) native = await adapters.bindNative(parent);
      if (!native) errors.push({ process: "native", code: "native-not-observed", message: "native identity remains unknown" });
    } catch (error) { errors.push({ process: "native", code: error.code ?? "cleanup-discovery-failed", message: error.message }); }
  }
  return { parent, native, errors };
}

export function finalOutcome(scenarioSucceeded, scenarioError, cleanupError) {
  return scenarioSucceeded && !scenarioError && !cleanupError ? "passed" : "failed";
}

async function listenerRefuses(port, context) {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host: "127.0.0.1", port });
    const timer = setTimeout(() => { socket.destroy(); reject(new ProbeError("listener refusal timed out", "listener-still-live")); }, remaining(context, 1_000));
    socket.once("connect", () => { clearTimeout(timer); socket.destroy(); reject(new ProbeError("listener still accepted TCP", "listener-still-live")); });
    socket.once("error", (error) => { clearTimeout(timer); error.code === "ECONNREFUSED" ? resolve(true) : reject(error); });
  });
}

async function inspectQuarantine(context, files) {
  const targets = new Set();
  for (const file of files) for (let cursor = file;; cursor = path.dirname(cursor)) {
    targets.add(cursor);
    if (path.dirname(cursor) === cursor) break;
  }
  for (const target of targets) {
    const result = command(context, "/usr/bin/xattr", [target]);
    requireFact(result.stderr === "" && !result.stdout.split("\n").includes("com.apple.quarantine"),
      `quarantine refused for ${target}`, "quarantine-refused");
  }
  return { checked: [...targets], quarantine_present: false };
}

async function snapshotInputs(options, context) {
  const layout = bundleLayoutForLauncher(options.bundleLauncher);
  requireFact(options.bundleMode && options.server === layout.server && options.native === layout.native,
    "bundle paths do not match fixed launcher layout", "bundle-path-mismatch");
  for (const [label, directory] of Object.entries({ "bundle parent": path.dirname(layout.root), bundle: layout.root,
    Contents: layout.contents, MacOS: layout.macOS, Resources: layout.resources })) await requirePrivateDirectory(directory, label);
  const snapshot = { layout };
  snapshot.launcher = await pinnedFile(layout.launcher, options.bundleLauncherSha256, "bundle launcher", { executable: true });
  snapshot.server = await pinnedFile(layout.server, options.serverSha256, "bundle server", { executable: true });
  snapshot.native = await pinnedFile(layout.native, options.nativeSha256, "bundle native", { executable: true });
  snapshot.manifest = await pinnedFile(layout.manifest, options.bundleManifestSha256, "bundle manifest",
    { maxBytes: MAX_METADATA, retainBody: true });
  snapshot.plist = await pinnedFile(layout.plist, options.bundlePlistSha256, "bundle plist", { maxBytes: MAX_METADATA });
  snapshot.driver = await pinnedFile(options.driver, options.driverSha256, "AX driver", { executable: true, privateMode: false });
  snapshot.self = { path: SELF, sha256: await digest(SELF, MAX_SOURCE) };
  snapshot.helper = { path: HELPER, sha256: await digest(HELPER, MAX_SOURCE) };
  snapshot.workflowDriver = await pinnedFile(WORKFLOW_DRIVER, WORKFLOW_DRIVER_SHA256,
    "workflow driver executable", { executable: true });
  snapshot.workflowDriverSource = await pinnedFile(WORKFLOW_DRIVER_SOURCE, WORKFLOW_DRIVER_SOURCE_SHA256,
    "workflow driver source", { maxBytes: MAX_SOURCE });
  snapshot.workflowDriverTestSource = await pinnedFile(WORKFLOW_DRIVER_TEST_SOURCE, WORKFLOW_DRIVER_TEST_SHA256,
    "workflow driver test source", { maxBytes: MAX_SOURCE });
  snapshot.workflowDriverChecks = await pinnedFile(WORKFLOW_DRIVER_CHECKS, WORKFLOW_DRIVER_CHECKS_SHA256,
    "workflow driver checks", { maxBytes: MAX_METADATA });
  snapshot.workflowDriverConfig = await pinnedFile(WORKFLOW_DRIVER_CONFIG, WORKFLOW_DRIVER_CONFIG_SHA256,
    "workflow driver config", { maxBytes: MAX_METADATA });
  snapshot.manifestValue = JSON.parse(snapshot.manifest.body.toString("utf8"));
  delete snapshot.manifest.body;
  snapshot.quarantine = await inspectQuarantine(context, [layout.launcher, layout.server, layout.native, layout.manifest,
    layout.plist, options.driver, SELF, HELPER, WORKFLOW_DRIVER, WORKFLOW_DRIVER_SOURCE,
    WORKFLOW_DRIVER_TEST_SOURCE, WORKFLOW_DRIVER_CHECKS, WORKFLOW_DRIVER_CONFIG]);
  return snapshot;
}

async function assertSnapshotUnchanged(snapshot, options) {
  for (const [key, expected] of Object.entries({ launcher: options.bundleLauncherSha256, server: options.serverSha256,
    native: options.nativeSha256, manifest: options.bundleManifestSha256, plist: options.bundlePlistSha256 })) {
    await pinnedFile(snapshot[key].path, expected, `unchanged ${key}`, { executable: ["launcher", "server", "native"].includes(key),
      maxBytes: ["manifest", "plist"].includes(key) ? MAX_METADATA : MAX_EXECUTABLE });
  }
  await pinnedFile(snapshot.driver.path, options.driverSha256, "unchanged AX driver", { executable: true, privateMode: false });
  await pinnedFile(WORKFLOW_DRIVER, WORKFLOW_DRIVER_SHA256, "unchanged workflow driver executable", { executable: true });
  await pinnedFile(WORKFLOW_DRIVER_SOURCE, WORKFLOW_DRIVER_SOURCE_SHA256, "unchanged workflow driver source", { maxBytes: MAX_SOURCE });
  await pinnedFile(WORKFLOW_DRIVER_TEST_SOURCE, WORKFLOW_DRIVER_TEST_SHA256, "unchanged workflow driver test source", { maxBytes: MAX_SOURCE });
  await pinnedFile(WORKFLOW_DRIVER_CHECKS, WORKFLOW_DRIVER_CHECKS_SHA256, "unchanged workflow driver checks", { maxBytes: MAX_METADATA });
  await pinnedFile(WORKFLOW_DRIVER_CONFIG, WORKFLOW_DRIVER_CONFIG_SHA256, "unchanged workflow driver config", { maxBytes: MAX_METADATA });
  requireFact(await digest(SELF, MAX_SOURCE) === snapshot.self.sha256 && await digest(HELPER, MAX_SOURCE) === snapshot.helper.sha256,
    "executed harness/helper changed", "identity-changed");
}

async function copySnapshot(context, snapshot, options, host) {
  for (const [source, name, mode] of [[SELF, "launchservices_probe.mjs", 0o400], [HELPER, "native_delivery_probe.mjs", 0o400],
    [options.driver, "native-ui-driver", 0o500], [WORKFLOW_DRIVER, "native-workflow-driver", 0o500],
    [WORKFLOW_DRIVER_SOURCE, "native_workflow_driver.swift", 0o400],
    [WORKFLOW_DRIVER_TEST_SOURCE, "native_workflow_driver_test.swift", 0o400],
    [WORKFLOW_DRIVER_CHECKS, "workflow-driver-checks.json", 0o400],
    [WORKFLOW_DRIVER_CONFIG, "workflow-driver-config.json", 0o400]]) {
    await copyFile(source, path.join(context.runDir, name), fsConstants.COPYFILE_EXCL);
    await chmod(path.join(context.runDir, name), mode);
  }
  const copies = {
    self: await pinnedFile(path.join(context.runDir, "launchservices_probe.mjs"), snapshot.self.sha256, "snapshotted harness", { maxBytes: MAX_SOURCE }),
    helper: await pinnedFile(path.join(context.runDir, "native_delivery_probe.mjs"), snapshot.helper.sha256, "snapshotted helper", { maxBytes: MAX_SOURCE }),
    driver: await pinnedFile(path.join(context.runDir, "native-ui-driver"), options.driverSha256, "snapshotted AX driver", { executable: true }),
    workflowDriver: await pinnedFile(path.join(context.runDir, "native-workflow-driver"), WORKFLOW_DRIVER_SHA256,
      "snapshotted workflow driver executable", { executable: true }),
    workflowDriverSource: await pinnedFile(path.join(context.runDir, "native_workflow_driver.swift"),
      WORKFLOW_DRIVER_SOURCE_SHA256, "snapshotted workflow driver source", { maxBytes: MAX_SOURCE }),
    workflowDriverTestSource: await pinnedFile(path.join(context.runDir, "native_workflow_driver_test.swift"),
      WORKFLOW_DRIVER_TEST_SHA256, "snapshotted workflow driver test source", { maxBytes: MAX_SOURCE }),
    workflowDriverChecks: await pinnedFile(path.join(context.runDir, "workflow-driver-checks.json"),
      WORKFLOW_DRIVER_CHECKS_SHA256, "snapshotted workflow driver checks", { maxBytes: MAX_METADATA }),
    workflowDriverConfig: await pinnedFile(path.join(context.runDir, "workflow-driver-config.json"),
      WORKFLOW_DRIVER_CONFIG_SHA256, "snapshotted workflow driver config", { maxBytes: MAX_METADATA }),
  };
  const config = path.join(context.runDir, "config.json");
  await writeFile(config, `${JSON.stringify({
    options: { ...options, bundleMode: undefined },
    sources: { self: snapshot.self, helper: snapshot.helper },
    host,
    command: "/usr/bin/open", args: ["-n", snapshot.layout.root],
    limits: { hard_ms: HARD_MS, command_ms: COMMAND_MS, output_bytes: MAX_OUTPUT,
      workflow_command_ms: WORKFLOW_COMMAND_MS, workflow_output_bytes: MAX_WORKFLOW_OUTPUT, metadata_bytes: MAX_METADATA },
    scope: "normal local LaunchServices; no redirection, environment override, signing, install, policy change, or browser fallback pass",
  }, null, 2)}\n`, { flag: "wx", mode: 0o600 });
  copies.config = { path: config, sha256: await digest(config, MAX_METADATA) };
  return copies;
}

async function drive(context, native, operation) {
  await assertBoundIdentity(native, context);
  const driver = path.join(context.runDir, "native-ui-driver");
  const result = command(context, driver, [String(native.pid), native.observed, operation]);
  requireFact(result.stderr === "" && Buffer.byteLength(result.stdout) <= 4 * 1024, "AX driver output refused", "invalid-driver-receipt");
  let value;
  try { value = JSON.parse(result.stdout); } catch { throw new ProbeError("AX driver did not return JSON", "invalid-driver-receipt"); }
  const receipt = assertStrictState(value, operation, native.pid);
  requireFact(context.states.length < MAX_STATES, "AX receipt limit exceeded", "receipt-overflow");
  context.states.push(receipt);
  return receipt;
}

async function waitState(context, parent, native, windows, newWindowEnabled) {
  return waitFor(async () => {
    await assertBoundIdentity(parent, context);
    const receipt = await drive(context, native, "inspect");
    if (receipt.windows !== windows || receipt.new_window_enabled !== newWindowEnabled) return null;
    assertWindowState(receipt, { windows, newWindowEnabled, browserEnabled: true });
    return receipt;
  }, context, `AX state ${windows}/${newWindowEnabled}`);
}

async function action(context, parent, native, operation, windows, newWindowEnabled) {
  await assertBoundIdentity(parent, context);
  const receipt = await drive(context, native, operation);
  assertWindowState(receipt, { windows, newWindowEnabled, browserEnabled: true });
}

async function executeWorkflow(context, parent, native) {
  await assertBoundIdentity(parent, context);
  await assertBoundIdentity(native, context);
  const children = directChildren(context, parent.pid);
  requireFact(children.length === 1 && children[0] === native.pid,
    "native direct ancestry changed before workflow", "workflow-identity-refused");
  const executable = path.join(context.runDir, "native-workflow-driver");
  await pinnedFile(executable, WORKFLOW_DRIVER_SHA256, "workflow driver execution copy", { executable: true });
  const result = command(context, executable, [String(native.pid), native.observed],
    { allowOne: true, timeoutMs: WORKFLOW_COMMAND_MS, maxOutput: MAX_WORKFLOW_OUTPUT });
  const rawStdout = path.join(context.runDir, "workflow-driver.stdout");
  const rawStderr = path.join(context.runDir, "workflow-driver.stderr");
  await writeFile(rawStdout, result.stdout, { flag: "wx", mode: 0o600 });
  await writeFile(rawStderr, result.stderr, { flag: "wx", mode: 0o600 });
  await writeFile(path.join(context.runDir, "workflow-command-receipt.json"), `${JSON.stringify({
    exit: result.exit, stdout_path: rawStdout, stderr_path: rawStderr,
    stdout_bytes: Buffer.byteLength(result.stdout), stderr_bytes: Buffer.byteLength(result.stderr),
    stdout_sha256: createHash("sha256").update(result.stdout).digest("hex"),
    stderr_sha256: createHash("sha256").update(result.stderr).digest("hex"),
  }, null, 2)}\n`, { flag: "wx", mode: 0o600 });
  requireFact(result.stderr === "" && Buffer.byteLength(result.stdout) > 0 &&
    Buffer.byteLength(result.stdout) <= MAX_WORKFLOW_OUTPUT, "workflow output refused", "workflow-invalid");
  let output;
  try { output = JSON.parse(result.stdout); } catch { throw new ProbeError("workflow driver did not return JSON", "workflow-invalid"); }
  requireFact(output && output.schema === "appkit-native-workflow-ax-v1" && typeof output.workflow_passed === "boolean" &&
    output.pid === native.pid && output.observed_executable === native.observed,
  "workflow driver schema or identity refused", "workflow-invalid");
  if (result.exit === 0) requireFact(output.workflow_passed === true && Array.isArray(output.steps),
    "successful workflow lacked passed steps", "workflow-invalid");
  else requireFact(output.workflow_passed === false && typeof output.error === "string" && output.error.length > 0 &&
    Array.isArray(output.completed_and_attempted_steps), "failed workflow lacked retained evidence", "workflow-invalid");
  const workflowPath = path.join(context.runDir, "workflow-report.json");
  await writeFile(workflowPath, `${JSON.stringify(output)}\n`, { flag: "wx", mode: 0o600 });
  return { passed: output.workflow_passed, exit: result.exit, report_path: workflowPath,
    report_sha256: await digest(workflowPath, MAX_WORKFLOW_OUTPUT), error: output.error ?? null };
}

async function runLifecycle(context, parent, native, recordWorkflow) {
  // Cold windows:0 is pending, never repaired by mutating AX state.
  await waitState(context, parent, native, 1, true);
  const listening = command(context, "/usr/sbin/lsof", ["-nP", "-a", "-p", String(parent.pid), "-iTCP", "-sTCP:LISTEN", "-Fn"]);
  const listener = parseListenerFields(listening.stdout);
  await action(context, parent, native, "new-window", 1, true); await waitState(context, parent, native, 2, false);
  const workflow = await executeWorkflow(context, parent, native);
  recordWorkflow(workflow);
  await action(context, parent, native, "close-window", 2, false); await waitState(context, parent, native, 1, true);
  await action(context, parent, native, "new-window", 1, true); await waitState(context, parent, native, 2, false);
  await action(context, parent, native, "close-window", 2, false); await waitState(context, parent, native, 1, true);
  await action(context, parent, native, "close-window", 1, true);
  await waitFor(async () => await processAbsent(parent.pid) && await processAbsent(native.pid), context, "normal final-close absence");
  await listenerRefuses(listener.port, context);
  return { workflow, normal_close: { listener, parent_absent: true, native_absent: true, listener_refused: true,
    parent_exit_status: "unobserved: LaunchServices owns the parent; open exit is request status only" } };
}

export async function run(argv = process.argv.slice(2)) {
  if (process.platform !== "darwin") throw new ProbeError("LaunchServices probe requires darwin", "unsupported-platform");
  const options = parseArgs(argv);
  requireFact(options.bundleMode, "LaunchServices probe requires the complete bundle quartet", "invalid-arguments");
  await requirePrivateDirectory(options.outputParent, "--output-parent");
  const runDir = await mkdtemp(path.join(options.outputParent, "appkit-launchservices-"));
  await chmod(runDir, 0o700);
  const context = { runDir, deadline: Date.now() + HARD_MS, calls: [], states: [] };
  const report = { started_at: new Date().toISOString(), run_dir: runDir, outcome: "failed", events: [],
    authority: "owner-authorized old-bundle local AX Profile/Activity workflow and normal LaunchServices lifecycle",
    workflow: { status: "not-reached" } };
  let parent;
  let native;
  let snapshot;
  let baseline = new Set();
  let launchRequested = false;
  let scenarioSucceeded = false;
  try {
    snapshot = await snapshotInputs(options, context);
    const swVers = command(context, "/usr/bin/sw_vers", []);
    const uname = command(context, "/usr/bin/uname", ["-a"]);
    requireFact(swVers.stderr === "" && uname.stderr === "", "host identity diagnostics refused", "host-identity-refused");
    const host = { sw_vers: swVers.stdout, uname: uname.stdout };
    snapshot.copies = await copySnapshot(context, snapshot, options, host);
    const check = command(context, snapshot.layout.launcher, ["-check"]);
    requireFact(check.stderr === "", "launcher -check emitted diagnostics", "bundle-check-failed");
    const checked = assertBundleCheckReceipt(JSON.parse(check.stdout), snapshot);
    await assertSnapshotUnchanged(snapshot, options);
    report.prelaunch = { snapshot: Object.fromEntries(Object.entries(snapshot).filter(([key]) => key !== "manifestValue")),
      launcher_check: checked, identities_unchanged: true, host };
    baseline = new Set();
    for (const pid of namedPIDs(context, "gallery-server")) if (await processPath(context, pid) === snapshot.server.path) baseline.add(pid);
    requireFact(baseline.size === 0, "selected exact bundle parent is already running", "parent-already-running");
    report.baseline = { exact_parent_path: snapshot.server.path, sha256: snapshot.server.sha256, pids: [...baseline] };
    await writeFile(path.join(runDir, "prelaunch.json"), `${JSON.stringify({ ...report.prelaunch, baseline: report.baseline }, null, 2)}\n`,
      { flag: "wx", mode: 0o600 });
    report.events.push("prelaunch-snapshot-quarantine-check-launcher-check-and-baseline-passed");
    // Deliberately inherit the normal environment and pass only: open -n BUNDLE.
    launchRequested = true;
    const opened = command(context, "/usr/bin/open", ["-n", snapshot.layout.root], { inheritEnvironment: true });
    requireFact(opened.stderr === "", "LaunchServices request emitted diagnostics", "launchservices-request-failed");
    report.launchservices_request = { exit: opened.exit, parent_exit_status: "unobserved" };
    const candidate = await waitFor(async () => (await currentParentCandidates(context, snapshot, baseline))[0] ?? null,
      context, "one new exact parent", Date.now() + 30_000);
    parent = await bindParent(context, candidate, snapshot);
    report.parent = parent;
    native = await waitFor(() => bindNative(context, parent, snapshot.native.sha256), context, "one exact direct native child", Date.now() + 30_000);
    report.native = native;
    report.workflow = { status: "pending" };
    const lifecycle = await runLifecycle(context, parent, native, (workflow) => {
      report.workflow = { status: workflow.passed ? "passed" : "failed", ...workflow };
      report.events.push(`workflow-${report.workflow.status}`);
    });
    report.normal_close = lifecycle.normal_close;
    report.events.push("native-one-two-close-reopen-close-normal-absence-and-listener-refusal-passed");
    await assertSnapshotUnchanged(snapshot, options);
    await pinnedFile(path.join(context.runDir, "native-ui-driver"), options.driverSha256, "unchanged snapshotted AX driver", { executable: true });
    await pinnedFile(path.join(context.runDir, "native-workflow-driver"), WORKFLOW_DRIVER_SHA256,
      "unchanged snapshotted workflow driver executable", { executable: true });
    report.inputs_unchanged = true;
    requireFact(lifecycle.workflow.passed, `AX workflow failed: ${lifecycle.workflow.error}`, "workflow-failed");
    scenarioSucceeded = true;
  } catch (error) {
    report.error = { code: error.code ?? "unexpected", message: error.message };
  } finally {
    context.deadline = Date.now() + CLEANUP_MS;
    const recovered = await recoverCleanupBindings({ launchRequested, parent, native, snapshot, baseline }, {
      absent: processAbsent,
      findParent: async () => {
        try {
          return await waitFor(async () => (await currentParentCandidates(context, snapshot, baseline))[0] ?? null,
            context, "cleanup parent discovery", Date.now() + Math.min(3_000, remaining(context, 3_000)));
        } catch (error) {
          if (error.code === "timeout") return null;
          throw error;
        }
      },
      bindParent: (candidate) => bindParent(context, candidate, snapshot),
      bindNative: (boundParent) => bindNative(context, boundParent, snapshot.native.sha256),
    });
    parent = recovered.parent;
    native = recovered.native;
    if (recovered.errors.length) report.cleanup_discovery_errors = recovered.errors;
    try { report.cleanup = await cleanupOwnedProcesses({ parent, native }, {
      context,
      authorizePair: async (boundParent, boundNative) => {
        await assertBoundIdentity(boundParent, context);
        await assertBoundIdentity(boundNative, context);
        const children = directChildren(context, boundParent.pid);
        requireFact(children.length === 1 && children[0] === boundNative.pid,
          "native direct ancestry changed before cleanup", "native-cleanup-unproven");
      },
    }); }
    catch (error) { report.cleanup_error = { code: error.code ?? "cleanup-failed", message: error.message, details: error.details }; }
    report.calls = context.calls;
    report.states = context.states;
    report.parent_exit_status = "unobserved: LaunchServices provides no owned parent exit status";
    report.browser_fallback = "not a native pass; no browser menu or manual-key action is part of this oracle";
    report.outcome = finalOutcome(scenarioSucceeded, report.error, report.cleanup_error);
    report.finished_at = new Date().toISOString();
    await writeFile(path.join(runDir, "report.json"), `${JSON.stringify(report, null, 2)}\n`, { flag: "wx", mode: 0o600 });
  }
  if (report.outcome !== "passed") throw new ProbeError(`LaunchServices evidence failed; retained ${runDir}`, report.error?.code ?? "probe-failed");
  return { runDir, report };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  run().then(({ runDir }) => process.stdout.write(`${JSON.stringify({ ok: true, run_dir: runDir })}\n`))
    .catch((error) => { process.stderr.write(`LaunchServices probe failed [${error.code ?? "unexpected"}]: ${error.message}\n`); process.exitCode = 1; });
}
