import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { copyFile, mkdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const mobileRoot = path.resolve(frontendRoot, "..", "mobile");
const fileName = "kfb-invest-android.apk";
const source = path.join(mobileRoot, "artifacts", fileName);
const downloadDirectory = path.join(frontendRoot, "public", "downloads");
const destination = path.join(downloadDirectory, fileName);
const manifestPath = path.join(frontendRoot, "src", "mobile-release.json");

async function fileInfo(filePath) {
  try {
    return await stat(filePath);
  } catch (error) {
    if (error.code === "ENOENT") return null;
    throw error;
  }
}

async function releaseVersion() {
  try {
    const config = JSON.parse(await readFile(path.join(mobileRoot, "app.json"), "utf8"));
    const version = config.expo?.version;
    return typeof version === "string" && version.trim() ? version.trim() : "0.1.0";
  } catch (error) {
    if (error.code === "ENOENT") {
      // The isolated Docker context retains the version prepared by the full checkout.
      try {
        const release = JSON.parse(await readFile(manifestPath, "utf8"));
        if (typeof release.version === "string" && release.version.trim()) return release.version.trim();
      } catch (manifestError) {
        if (manifestError.code !== "ENOENT") throw manifestError;
      }
      return "0.1.0";
    }
    throw error;
  }
}

async function sha256(filePath) {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(filePath)) hash.update(chunk);
  return hash.digest("hex");
}

const release = {
  available: false,
  url: "",
  version: await releaseVersion(),
  bytes: 0,
  sha256: "",
  channel: "test",
  requirements: "Android 7+ · ARM64",
};

await mkdir(downloadDirectory, { recursive: true });
await mkdir(path.dirname(manifestPath), { recursive: true });

const sourceInfo = await fileInfo(source);
const sourceDirectoryExists = (await fileInfo(mobileRoot))?.isDirectory() ?? false;

if (sourceInfo?.isFile() && sourceInfo.size > 0) {
  await copyFile(source, destination);
} else if (sourceDirectoryExists) {
  // A full checkout owns the source APK. Never retain a removed release copy.
  await rm(destination, { force: true });
}

// A frontend-only Docker context has no ../mobile directory. The prepared public
// file is then the release input; its size and checksum are always recalculated.
const downloadInfo = await fileInfo(destination);
if (downloadInfo?.isFile() && downloadInfo.size > 0) {
  release.available = true;
  release.url = `/downloads/${fileName}`;
  release.bytes = downloadInfo.size;
  release.sha256 = await sha256(destination);
} else if (downloadInfo?.isFile()) {
  await rm(destination, { force: true });
}

await writeFile(manifestPath, `${JSON.stringify(release, null, 2)}\n`, "utf8");

if (release.available) {
  console.log(`Android APK ready: ${release.url} (${release.bytes} bytes; SHA-256 ${release.sha256})`);
} else {
  console.log("Android APK unavailable: download link disabled.");
}
