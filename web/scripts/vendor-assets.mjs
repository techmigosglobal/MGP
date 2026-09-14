import { copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const assets = [
  ["node_modules/htmx.org/dist/htmx.min.js", "static/js/htmx.min.js"],
  ["node_modules/@alpinejs/csp/dist/cdn.min.js", "static/js/alpine.min.js"]
];

for (const [source, target] of assets) {
  const output = resolve(root, target);
  await mkdir(dirname(output), { recursive: true });
  await copyFile(resolve(root, source), output);
}
