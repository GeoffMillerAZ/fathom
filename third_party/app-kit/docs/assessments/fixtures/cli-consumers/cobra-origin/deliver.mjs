#!/usr/bin/env node

import { createHash } from "node:crypto";
import * as fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const RECEIPT_SHA = "4af4f21d8ee27f4bd403df24136825ac72edf995207e0c1f7b29aab942dbc7c5";
const MODULES = [
  { dir: "cobra", name: "github.com/spf13/cobra", version: "v1.10.2", files: 67 },
  { dir: "pflag", name: "github.com/spf13/pflag", version: "v1.0.10", files: 89 },
];
const MAX_FILE = 4 * 1024 * 1024;
const MAX_TOTAL = 40 * 1024 * 1024;
const MAX_FILES = 200;
const SOURCE_MOD = `module github.com/spf13/cobra

go 1.15

require (
\tgithub.com/cpuguy83/go-md2man/v2 v2.0.6
\tgithub.com/inconshreveable/mousetrap v1.1.0
\tgithub.com/spf13/pflag v1.0.9
\tgo.yaml.in/yaml/v3 v3.0.4
)
`;
const DELIVERED_MOD = SOURCE_MOD.replace(
  "github.com/spf13/pflag v1.0.9",
  "github.com/spf13/pflag v1.0.10",
) + "\nreplace github.com/spf13/pflag => ../pflag\n";

function sha(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}
function safeRelative(name) {
  if (typeof name !== "string" || !name || name.length > 1024 || name.includes("\\") ||
      name.includes("\0") || path.posix.isAbsolute(name) || path.posix.normalize(name) !== name ||
      name.split("/").some((part) => !part || part === "." || part === "..")) {
    throw new Error(`unsafe manifest path: ${JSON.stringify(name)}`);
  }
  return name;
}
async function existingRoot(raw, label) {
  if (!path.isAbsolute(raw)) throw new Error(`${label} must be absolute`);
  const resolved = path.resolve(raw);
  const info = await fs.lstat(resolved);
  if (!info.isDirectory() || info.isSymbolicLink()) throw new Error(`${label} is not a regular directory`);
  if (await fs.realpath(resolved) !== resolved) throw new Error(`${label} must be canonical`);
  return resolved;
}
async function freshRoot(raw) {
  if (!path.isAbsolute(raw)) throw new Error("target must be absolute");
  const resolved = path.resolve(raw);
  const parent = await existingRoot(path.dirname(resolved), "target parent");
  const target = path.join(parent, path.basename(resolved));
  try {
    await fs.lstat(target);
    throw new Error("target already exists");
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  return target;
}
function overlaps(a, b) {
  const rel = path.relative(a, b);
  return rel === "" || (!rel.startsWith(`..${path.sep}`) && rel !== "..");
}
async function readRegular(file, limit = MAX_FILE) {
  const linkInfo = await fs.lstat(file);
  if (!linkInfo.isFile() || linkInfo.isSymbolicLink()) throw new Error(`not a regular file: ${file}`);
  const handle = await fs.open(file, "r");
  try {
    const info = await handle.stat();
    if (!info.isFile() || info.size > limit) throw new Error(`invalid or oversized file: ${file}`);
    const bytes = await handle.readFile();
    if (bytes.length !== info.size) throw new Error(`file changed while reading: ${file}`);
    return bytes;
  } finally {
    await handle.close();
  }
}
async function exactRoot(root, expected, label) {
  const entries = await fs.readdir(root, { withFileTypes: true });
  if (entries.some((entry) => !entry.isDirectory() && !entry.isFile())) throw new Error(`${label} contains a symlink or special file`);
  exactKeys(entries.map((entry) => entry.name), expected, label);
}
async function readTree(root) {
  const found = new Map();
  let count = 0;
  let total = 0;
  async function visit(dir, prefix = "") {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      const rel = safeRelative(prefix ? `${prefix}/${entry.name}` : entry.name);
      const full = path.join(root, ...rel.split("/"));
      if (entry.isDirectory()) await visit(full, rel);
      else if (entry.isFile()) {
        if (++count > MAX_FILES) throw new Error(`too many files below ${root}`);
        const bytes = await readRegular(full);
        if ((total += bytes.length) > MAX_TOTAL) throw new Error(`tree exceeds read budget: ${root}`);
        found.set(rel, bytes);
      } else throw new Error(`symlink or special file rejected: ${full}`);
    }
  }
  await visit(root);
  return found;
}
function exactKeys(actual, expected, label) {
  const a = [...actual].sort();
  const e = [...expected].sort();
  if (a.length !== e.length || a.some((value, i) => value !== e[i])) {
    throw new Error(`${label} has missing or extra files`);
  }
}
function validateReceipt(receipt) {
  if (!Array.isArray(receipt) || receipt.length !== MODULES.length) throw new Error("invalid source receipt");
  for (let i = 0; i < MODULES.length; i++) {
    const want = MODULES[i];
    const got = receipt[i];
    if (got?.name !== want.name || got.version !== want.version ||
        typeof got.files !== "object" || got.files === null || Object.keys(got.files).length !== want.files) {
      throw new Error(`unexpected ${want.dir} receipt identity`);
    }
    for (const [name, digest] of Object.entries(got.files)) {
      safeRelative(name);
      if (!/^[0-9a-f]{64}$/.test(digest)) throw new Error(`invalid digest for ${want.dir}/${name}`);
    }
  }
  if (receipt[0].files["go.mod"] !== sha(Buffer.from(SOURCE_MOD))) {
    throw new Error("reviewed cobra go.mod identity is inconsistent");
  }
}
async function sourceReceipt(root, filename) {
  const bytes = await readRegular(path.join(root, filename), 64 * 1024);
  if (sha(bytes) !== RECEIPT_SHA) throw new Error("source receipt identity mismatch");
  const receipt = JSON.parse(bytes.toString("utf8"));
  validateReceipt(receipt);
  return { bytes, receipt };
}
function expectedFiles(receipt) {
  const files = { "source-receipt.json": RECEIPT_SHA };
  for (let i = 0; i < MODULES.length; i++) {
    for (const [name, digest] of Object.entries(receipt[i].files)) {
      files[`${MODULES[i].dir}/${name}`] = i === 0 && name === "go.mod" ? sha(Buffer.from(DELIVERED_MOD)) : digest;
    }
  }
  return Object.fromEntries(Object.entries(files).sort(([a], [b]) => a.localeCompare(b)));
}
function deliveryBytes(receipt) {
  const modules = MODULES.map((want, i) => ({
    name: want.name, version: want.version, inputSum: receipt[i].inputSum,
    candidateSum: receipt[i].candidateSum, fileCount: want.files,
  }));
  return Buffer.from(`${JSON.stringify({
    schemaVersion: 1,
    kind: "appkit-cli-origin-dependency-delivery",
    sourceReceipt: { file: "source-receipt.json", sha256: RECEIPT_SHA, modules },
    wiring: { cobraPflagVersion: "v1.0.10", cobraPflagReplace: "../pflag" },
    files: expectedFiles(receipt),
    manifestExcludes: ["delivery.json"],
  }, null, 2)}\n`);
}
async function verifiedModules(root, receipt, delivered) {
  const trees = [];
  for (let i = 0; i < MODULES.length; i++) {
    const tree = await readTree(path.join(root, MODULES[i].dir));
    exactKeys(tree.keys(), Object.keys(receipt[i].files), MODULES[i].dir);
    for (const [name, bytes] of tree) {
      const want = delivered && i === 0 && name === "go.mod"
        ? sha(Buffer.from(DELIVERED_MOD)) : receipt[i].files[name];
      if (sha(bytes) !== want) throw new Error(`content drift: ${MODULES[i].dir}/${name}`);
    }
    trees.push(tree);
  }
  return trees;
}
async function writeExclusive(file, bytes) {
  await fs.mkdir(path.dirname(file), { recursive: true, mode: 0o700 });
  await fs.writeFile(file, bytes, { flag: "wx", mode: 0o600 });
}
async function stage(sourceArg, targetArg) {
  const source = await existingRoot(sourceArg, "candidate root");
  const target = await freshRoot(targetArg);
  if (overlaps(source, target) || overlaps(target, source)) throw new Error("source and target overlap");
  await exactRoot(source, ["cobra", "pflag", "receipt.json"], "candidate root");
  const { bytes: receiptBytes, receipt } = await sourceReceipt(source, "receipt.json");
  const trees = await verifiedModules(source, receipt, false);
  await fs.mkdir(target, { mode: 0o700 });
  for (let i = 0; i < MODULES.length; i++) {
    for (const [name, bytes] of trees[i]) {
      const output = i === 0 && name === "go.mod" ? Buffer.from(DELIVERED_MOD) : bytes;
      await writeExclusive(path.join(target, MODULES[i].dir, ...name.split("/")), output);
    }
  }
  await writeExclusive(path.join(target, "source-receipt.json"), receiptBytes);
  await writeExclusive(path.join(target, "delivery.json"), deliveryBytes(receipt));
}
async function check(rootArg) {
  const root = await existingRoot(rootArg, "delivered root");
  await exactRoot(root, ["cobra", "pflag", "source-receipt.json", "delivery.json"], "delivered root");
  const { receipt } = await sourceReceipt(root, "source-receipt.json");
  await verifiedModules(root, receipt, true);
  const delivery = await readRegular(path.join(root, "delivery.json"), 256 * 1024);
  if (!delivery.equals(deliveryBytes(receipt))) throw new Error("delivery manifest drift");
}

async function main() {
  const [command, ...args] = process.argv.slice(2);
  if (command === "stage" && args.length === 2) await stage(args[0], args[1]);
  else if (command === "check" && args.length === 1) await check(args[0]);
  else throw new Error("usage: deliver.mjs stage <candidate-root> <fresh-absolute-target> | check <delivered-root>");
}

main().catch((error) => {
  process.stderr.write(`deliver: ${error.message}\n`);
  process.exitCode = 1;
});
