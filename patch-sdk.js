const fs = require("fs");
const path = require("path");

const pkgPath = path.join(__dirname, "sdk", "nodejs", "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));

pkg.name = "@bambamboole/mattermost";
pkg.main = "index.js";
pkg.types = "index.d.ts";
pkg.publishConfig = { access: "public" };

fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
console.log("patched", pkg.name);
