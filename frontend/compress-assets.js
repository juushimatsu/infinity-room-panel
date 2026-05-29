const fs = require("fs");
const path = require("path");
const zlib = require("zlib");

const buildDir = path.join(__dirname, "build");

if (!fs.existsSync(buildDir)) {
  console.error("build/ directory not found. Run `npm run build` first.");
  process.exit(1);
}

function walk(dir) {
  const results = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results.push(...walk(full));
    } else if (/\.(js|css|html|svg)$/.test(entry.name)) {
      results.push(full);
    }
  }
  return results;
}

const files = walk(buildDir);
let gzCount = 0;
let brCount = 0;

for (const file of files) {
  const data = fs.readFileSync(file);

  // gzip
  const gz = zlib.gzipSync(data, { level: 9 });
  fs.writeFileSync(file + ".gz", gz);
  gzCount++;

  // brotli
  const br = zlib.brotliCompressSync(data, { params: { [zlib.constants.BROTLI_PARAM_QUALITY]: 11 } });
  fs.writeFileSync(file + ".br", br);
  brCount++;
}

console.log(`Compressed ${files.length} files: ${gzCount} .gz, ${brCount} .br`);
