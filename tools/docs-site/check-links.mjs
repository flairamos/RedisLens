import fs from "node:fs";
import path from "node:path";

const docs = path.resolve(import.meta.dirname, "..", "..", "docs");
const files = fs.readdirSync(docs).filter((f) => f.endsWith(".html"));
const pages = new Map();
for (const f of files) {
  pages.set(f, fs.readFileSync(path.join(docs, f), "utf8"));
}

const problems = [];
const idsByPage = new Map();
for (const [f, html] of pages) {
  const ids = new Set();
  for (const m of html.matchAll(/\bid="([^"]+)"/g)) ids.add(m[1]);
  idsByPage.set(f, ids);
}

for (const [f, html] of pages) {
  const refs = [];
  for (const m of html.matchAll(/\b(?:href|src)="([^"]+)"/g)) refs.push(m[1]);
  for (const ref of refs) {
    if (/^(https?:|mailto:|data:|#|\/\/)/.test(ref)) {
      if (ref.startsWith("#") && ref.length > 1) {
        const id = decodeURIComponent(ref.slice(1));
        if (!idsByPage.get(f).has(id)) problems.push(`${f}: 页内锚点不存在 ${ref}`);
      }
      continue;
    }
    const [rawPathWithQuery, hash] = ref.split("#");
    const rawPath = rawPathWithQuery.split("?")[0];
    const target = path.resolve(docs, decodeURIComponent(rawPath));
    if (!fs.existsSync(target)) {
      problems.push(`${f}: 本地文件不存在 ${ref}`);
      continue;
    }
    if (hash) {
      const targetName = path.basename(target);
      const ids = idsByPage.get(targetName);
      if (ids && !ids.has(decodeURIComponent(hash))) {
        problems.push(`${f}: 目标锚点不存在 ${ref}`);
      }
    }
  }
}

// 检查标题 id 重复
for (const [f, html] of pages) {
  const seen = new Set();
  for (const m of html.matchAll(/<h[23][^>]*\bid="([^"]+)"/g)) {
    if (seen.has(m[1])) problems.push(`${f}: 重复标题 id ${m[1]}`);
    seen.add(m[1]);
  }
}

console.log(`检查 ${files.length} 个页面，问题 ${problems.length} 个`);
problems.forEach((p) => console.log("  " + p));
