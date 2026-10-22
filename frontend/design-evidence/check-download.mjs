import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";

const base = process.env.MARKETING_TEST_URL || "http://localhost:15173";
const release = JSON.parse(await readFile(new URL("../src/mobile-release.json", import.meta.url), "utf8"));
const response = await fetch(`${base}${release.url}`);
assert.equal(response.status, 200);
assert.match(response.headers.get("content-type"), /application\/vnd.android.package-archive/);
assert.match(response.headers.get("content-disposition"), /attachment/);
const bytes = Buffer.from(await response.arrayBuffer());
const sha256 = createHash("sha256").update(bytes).digest("hex");
assert.equal(bytes.length, release.bytes);
assert.equal(sha256, release.sha256);
assert.equal(bytes.subarray(0, 4).toString("hex"), "504b0304");
const missing = await fetch(`${base}/downloads/missing.apk`);
assert.equal(missing.status, 404);
const routes = {};
for (const route of ["/", "/welcome", "/download", "/login", "/trading"]) {
  routes[route] = (await fetch(`${base}${route}`)).status;
  assert.equal(routes[route], 200);
}
const evidence = { base, status: response.status, contentType: response.headers.get("content-type"), contentDisposition: response.headers.get("content-disposition"), bytes: bytes.length, sha256, missingStatus: missing.status, routes };
await writeFile(new URL("./download-http.json", import.meta.url), JSON.stringify(evidence, null, 2) + "\n");
console.log(JSON.stringify(evidence, null, 2));
