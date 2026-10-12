// Checks that every literal passed to t() / tn() in web/src has a Chinese
// entry, and that translations keep the same {placeholders}.
// Usage: npm run i18n:check   (Node 23.6+ for .ts imports)
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "src",
);
const areaFiles = ["shell", "views", "project", "settings"];

const quoted = String.raw`(["'])((?:\\.|(?!\1)[^\\])*)\1`;
const tCall = new RegExp(String.raw`\bt\(\s*` + quoted, "g");
const tnCall = new RegExp(
  String.raw`\btn\(\s*[^,()]+(?:\([^()]*\))?[^,()]*,\s*(["'])((?:\\.|(?!\1)[^\\])*)\1\s*,\s*(["'])((?:\\.|(?!\3)[^\\])*)\3`,
  "g",
);
const recordBlock = /translatedRecord\(\{([\s\S]*?)\}\)/g;
const recordValue = new RegExp(String.raw`:\s*` + quoted, "g");

function unquote(quote, body) {
  const normalized =
    quote === "'" ? body.replace(/\\'/g, "'").replace(/"/g, '\\"') : body;
  return JSON.parse(`"${normalized}"`);
}

async function sourceFiles(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map((entry) => {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory())
        return entry.name === "i18n" ? [] : sourceFiles(full);
      return /\.(ts|tsx)$/.test(entry.name) ? [full] : [];
    }),
  );
  return nested.flat();
}

async function usedKeys() {
  const keys = new Map(); // key -> first file using it
  const add = (key, file) => keys.has(key) || keys.set(key, file);
  for (const file of await sourceFiles(root)) {
    const text = await readFile(file, "utf8");
    const relative = path.relative(root, file);
    for (const match of text.matchAll(tCall))
      add(unquote(match[1], match[2]), relative);
    for (const match of text.matchAll(tnCall)) {
      add(unquote(match[1], match[2]), relative);
      add(unquote(match[3], match[4]), relative);
    }
    for (const block of text.matchAll(recordBlock)) {
      for (const match of block[1].matchAll(recordValue))
        add(unquote(match[1], match[2]), relative);
    }
  }
  return keys;
}

async function catalog() {
  const merged = {};
  const conflicts = [];
  for (const area of areaFiles) {
    const module = await import(path.join(root, "i18n", "zh", `${area}.ts`));
    for (const [key, value] of Object.entries(module[area])) {
      if (key in merged && merged[key] !== value) {
        conflicts.push(`${key}: "${merged[key]}" vs "${value}" (${area})`);
      }
      merged[key] = value;
    }
  }
  return { merged, conflicts };
}

const placeholders = (text) =>
  [...text.matchAll(/\{(\w+)\}/g)]
    .map((m) => m[1])
    .sort()
    .join(",");

const keys = await usedKeys();
const { merged, conflicts } = await catalog();
const missing = [...keys].filter(([key]) => !(key in merged));
const broken = Object.entries(merged).filter(
  ([key, value]) => placeholders(key) !== placeholders(value),
);

for (const [key, file] of missing)
  console.log(`missing  ${file}: ${JSON.stringify(key)}`);
for (const [key, value] of broken)
  console.log(`vars     ${JSON.stringify(key)} -> ${JSON.stringify(value)}`);
for (const conflict of conflicts) console.log(`conflict ${conflict}`);
console.log(
  `${keys.size} keys used, ${Object.keys(merged).length} translated, ${missing.length} missing, ${broken.length} placeholder mismatches, ${conflicts.length} conflicts`,
);
process.exit(missing.length + broken.length > 0 ? 1 : 0);
