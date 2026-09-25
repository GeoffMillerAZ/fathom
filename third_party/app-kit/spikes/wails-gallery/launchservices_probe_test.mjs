import assert from "node:assert/strict";
import test from "node:test";
import {
  assertProcessGeneration,
  cleanupOwnedProcesses,
  finalOutcome,
  parseListenerFields,
  recoverCleanupBindings,
  resolveObservedExecutable,
} from "./launchservices_probe.mjs";

test("the legitimate macOS /var spelling retains observed and canonical paths", async () => {
  const observed = "/var/folders/aa/private/T/appkit-native-companion-7/gallery-native";
  const result = await resolveObservedExecutable(observed, { realpath: async (value) => {
    if (value === "/var") return "/private/var";
    if (value === observed) return `/private${observed}`;
    throw new Error(`unexpected path ${value}`);
  } });
  assert.deepEqual(result, { observed, canonical: `/private${observed}`, alias: "/var->/private/var" });
});

test("arbitrary aliases and a changed /var mapping remain refused", async () => {
  await assert.rejects(resolveObservedExecutable("/tmp/alias/gallery-native", {
    realpath: async () => "/private/tmp/alias/gallery-native",
  }), (error) => error.code === "process-identity-refused");
  const observed = "/var/folders/a/T/appkit-native-companion-1/gallery-native";
  await assert.rejects(resolveObservedExecutable(observed, {
    realpath: async (value) => value === "/var" ? "/unexpected" : `/private${value}`,
  }), (error) => error.code === "process-identity-refused");
  await assert.rejects(resolveObservedExecutable(observed, {
    realpath: async (value) => value === "/var" ? "/private/var" : "/private/var/elsewhere/gallery-native",
  }), (error) => error.code === "process-identity-refused");
});

test("listener parser accepts the exact lsof IPv4 loopback record", () => {
  assert.deepEqual(parseListenerFields("p22082\nf12\nn127.0.0.1:50593\n"),
    { address: "127.0.0.1:50593", port: 50593 });
});

test("listener parser freezes wildcard, extra, malformed, range, and escaped-regexp regressions", () => {
  for (const value of [
    "p1\nn*:50593\n",
    "p1\nn127.0.0.1:50593\nn127.0.0.1:50594\n",
    "p1\n127.0.0.1:50593\n",
    "p1\nn127.0.0.1:70000\n",
    "p1\nn127\\.0\\.0\\.1:50593\n",
    "p1\nn127x0x0x1:50593\n",
    "p1\nn127.0.0.1:0\n",
  ]) assert.throws(() => parseListenerFields(value), (error) => error.code === "listener-refused");
});

test("discovery refusal cleanup still stops the identity-bound parent and preserves native unknown", async () => {
  const parent = { pid: 41, identityBound: true, label: "synthetic parent" };
  const calls = [];
  const result = await cleanupOwnedProcesses({ parent, native: null }, {
    stop: async (identity) => calls.push(["stop", identity.pid]),
    absent: async (pid) => { calls.push(["absent", pid]); return true; },
  });
  assert.deepEqual(calls, [["stop", 41], ["absent", 41]]);
  assert.deepEqual(result, {
    parent_absent: true,
    native_absent: null,
    native_reason: "native identity was not bound; absence remains unknown",
  });
});

test("cleanup signals an identity-bound native only after the parent", async () => {
  const calls = [];
  const parent = { pid: 41, identityBound: true, label: "synthetic parent" };
  const native = { pid: 42, identityBound: true, label: "synthetic native" };
  const result = await cleanupOwnedProcesses({ parent, native }, {
    stop: async (identity) => calls.push(["stop", identity.pid]),
    absent: async (pid) => { calls.push(["absent", pid]); return true; },
  });
  assert.deepEqual(calls, [["stop", 41], ["absent", 41], ["stop", 42], ["absent", 42]]);
  assert.deepEqual(result, { parent_absent: true, native_absent: true });
});

test("post-lifecycle identity failure cannot retain a passing outcome", () => {
  assert.equal(finalOutcome(true, { code: "identity-changed" }, undefined), "failed");
  assert.equal(finalOutcome(true, undefined, { code: "cleanup-failed" }), "failed");
  assert.equal(finalOutcome(true, undefined, undefined), "passed");
});

test("same-path same-hash PID generation reuse is still refused", () => {
  const identity = { label: "synthetic parent", started: "Tue Sep 16 20:00:00 2026" };
  assert.doesNotThrow(() => assertProcessGeneration(identity, identity.started));
  assert.throws(() => assertProcessGeneration(identity, "Tue Sep 16 20:00:01 2026"),
    (error) => error.code === "process-pid-reused");
});

test("cleanup recovery rebinds a launch-owned parent even when native discovery refuses", async () => {
  const boundParent = { pid: 51, identityBound: true, label: "recovered parent" };
  const recovered = await recoverCleanupBindings({
    launchRequested: true, parent: null, native: null, snapshot: { fixed: true }, baseline: new Set(),
  }, {
    findParent: async () => ({ pid: 51 }),
    bindParent: async () => boundParent,
    absent: async () => false,
    bindNative: async () => { throw Object.assign(new Error("multiple children"), { code: "native-discovery-refused" }); },
  });
  assert.equal(recovered.parent, boundParent);
  assert.equal(recovered.native, null);
  assert.deepEqual(recovered.errors, [{ process: "native", code: "native-discovery-refused", message: "multiple children" }]);
});

test("a parent cleanup error does not suppress an independently bound native attempt", async () => {
  const calls = [];
  await assert.rejects(cleanupOwnedProcesses({
    parent: { pid: 61, identityBound: true, label: "parent" },
    native: { pid: 62, identityBound: true, label: "native" },
  }, {
    stop: async (identity) => {
      calls.push(["stop", identity.pid]);
      if (identity.pid === 61) throw Object.assign(new Error("parent changed"), { code: "process-pid-reused" });
    },
    absent: async (pid) => { calls.push(["absent", pid]); return true; },
  }), (error) => error.code === "cleanup-failed" && error.details.errors[0].process === "parent");
  assert.deepEqual(calls, [["stop", 61], ["stop", 62], ["absent", 62]]);
});

test("failed ancestry reauthorization stops the parent but never signals the native", async () => {
  const calls = [];
  await assert.rejects(cleanupOwnedProcesses({
    parent: { pid: 71, identityBound: true, label: "parent" },
    native: { pid: 72, identityBound: true, label: "native" },
  }, {
    absent: async () => false,
    authorizePair: async () => { throw Object.assign(new Error("ancestry changed"), { code: "native-cleanup-unproven" }); },
    stop: async (identity) => calls.push(["stop", identity.pid]),
  }), (error) => error.code === "cleanup-failed" && error.details.native_absent === null);
  assert.deepEqual(calls, [["stop", 71]]);
});

test("an ancestry absence-probe error cannot suppress known-parent cleanup", async () => {
  const calls = [];
  await assert.rejects(cleanupOwnedProcesses({
    parent: { pid: 81, identityBound: true, label: "parent" },
    native: { pid: 82, identityBound: true, label: "native" },
  }, {
    absent: async (pid) => {
      calls.push(["absent", pid]);
      if (pid === 82) throw Object.assign(new Error("absence observation failed"), { code: "process-inspection-failed" });
      return true;
    },
    authorizePair: async () => assert.fail("failed absence observation must prevent authorization"),
    stop: async (identity) => calls.push(["stop", identity.pid]),
  }), (error) => error.code === "cleanup-failed" && error.details.native_absent === null &&
    error.details.parent_absent === true && error.details.errors[0].code === "process-inspection-failed");
  assert.deepEqual(calls, [["absent", 82], ["stop", 81], ["absent", 81]]);
});
