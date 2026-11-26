// 将 tools/docs-site/pages 下的 HTML 片段套用统一布局，输出到 docs/。
// 用法：node tools/docs-site/build.mjs
import fs from "node:fs";
import path from "node:path";
import { site, sections, notFound } from "./site.mjs";

const root = path.resolve(import.meta.dirname, "..", "..");
const pagesDir = path.join(import.meta.dirname, "pages");
const outDir = path.join(root, "docs");

const pages = sections.flatMap((section) =>
  section.items.map((item) => ({ ...item, section: section.title }))
);
const allDocs = pages;

const esc = (s) =>
  String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");

const strip = (html) =>
  String(html)
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<svg[\s\S]*?<\/svg>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&middot;/g, "·")
    .replace(/&mdash;/g, "—")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, "&")
    .replace(/\s+/g, " ")
    .trim();

const slugify = (text) =>
  strip(text)
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fa5]+/g, "-")
    .replace(/^-+|-+$/g, "") || "section";

/** 给没有 id 的 h2/h3 补 id，确保目录与搜索锚点稳定。 */
function ensureHeadingIds(html) {
  const used = new Set();
  return html.replace(/<(h[23])([^>]*)>([\s\S]*?)<\/\1>/g, (match, tag, attrs, inner) => {
    let id = (attrs.match(/\bid="([^"]+)"/) || [])[1];
    if (!id) {
      const base = slugify(inner);
      id = base;
      let n = 2;
      while (used.has(id)) id = `${base}-${n++}`;
      attrs = `${attrs} id="${id}"`;
    }
    used.add(id);
    return `<${tag}${attrs}>${inner}</${tag}>`;
  });
}

const topbar = () => `
<header class="topbar">
  <div class="topbar-inner">
    <button class="icon-btn menu-btn" type="button" data-menu-toggle aria-label="打开文档导航" aria-expanded="false" aria-controls="docs-sidebar">
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M3 12h18M3 18h18"></path></svg>
    </button>
    <a class="brand" href="index.html" aria-label="RedisLens 文档首页">
      <svg class="brand-logo" viewBox="0 0 132 36" fill="none" role="img" aria-label="RedisLens">
        <g>
          <circle cx="16" cy="15" r="10.5" fill="none" stroke="currentColor" stroke-width="3"></circle>
          <path d="M16 8.2 L21.5 12.1 L16 16 L10.5 12.1 Z" fill="#DC382D"></path>
          <path d="M16 12.6 L21.5 16.5 L16 20.4 L10.5 16.5 Z" fill="#C02C22"></path>
          <path d="M16 17 L21.5 20.9 L16 24.8 L10.5 20.9 Z" fill="#A82620"></path>
          <line x1="24.2" y1="23.2" x2="30" y2="29" stroke="#DC382D" stroke-width="4.2" stroke-linecap="round"></line>
        </g>
        <text x="38" y="24" font-family="Inter, Segoe UI, Microsoft YaHei, sans-serif" font-size="17" font-weight="700" fill="currentColor" letter-spacing="0.2">Redis</text>
        <text x="86" y="24" font-family="Inter, Segoe UI, Microsoft YaHei, sans-serif" font-size="17" font-weight="700" fill="#DC382D" letter-spacing="0.2">Lens</text>
      </svg>
      <span class="brand-badge">Docs</span>
    </a>
    <nav class="topnav" aria-label="站点导航">
      <a href="index.html">文档</a>
      <a href="getting-started.html">快速开始</a>
      <a href="${site.releases}" rel="noopener">下载</a>
    </nav>
    <span class="topbar-spacer"></span>
    <button class="search-trigger" type="button" data-search-open aria-label="搜索文档（Ctrl K）">
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"></circle><path d="m20 20-3.2-3.2"></path></svg>
      <span>搜索文档</span>
      <span class="grow"></span>
      <kbd>Ctrl K</kbd>
    </button>
    <button class="icon-btn" type="button" data-theme-toggle aria-label="切换主题">
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>
    </button>
    <a class="icon-btn" href="${site.repo}" aria-label="RedisLens GitHub 仓库" title="GitHub" rel="noopener">
      <svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 19c-4.3 1.4-4.3-2.5-6-3m12 5v-3.5c0-1 .1-1.4-.5-2 2.8-.3 5.5-1.4 5.5-6a4.6 4.6 0 0 0-1.3-3.2 4.2 4.2 0 0 0-.1-3.2s-1.1-.3-3.5 1.3a12.3 12.3 0 0 0-6.4 0C6.2 2.8 5.1 3.1 5.1 3.1a4.2 4.2 0 0 0-.1 3.2A4.6 4.6 0 0 0 3.7 9.5c0 4.6 2.7 5.7 5.5 6-.6.6-.6 1.2-.5 2V21"></path></svg>
    </a>
  </div>
</header>`;

const sidebar = (activeFile) => {
  const groups = sections
    .map(
      (section) => `
    <div class="nav-section">
      <div class="nav-section-title">${esc(section.title)}</div>
      ${section.items
        .map(
          (item) => `<a class="nav-link${item.file === activeFile ? " active" : ""}" href="${item.file}"${
            item.file === activeFile ? ' aria-current="page"' : ""
          }><span class="nav-dot"></span>${esc(item.label)}</a>`
        )
        .join("\n      ")}
    </div>`
    )
    .join("");

  return `
<aside class="sidebar" id="docs-sidebar" aria-label="文档导航">
${groups}
  <div class="sidebar-footer">
    <div>RedisLens v${esc(site.version)} · ${esc(site.license)}</div>
    <div><a href="${site.repo}" rel="noopener">GitHub 仓库</a> · <a href="${site.issues}" rel="noopener">问题反馈</a></div>
  </div>
</aside>`;
};

const footer = () => `
<footer class="footer">
  <div class="footer-inner">
    <span>© ${esc(site.copyright)} · RedisLens 文档</span>
    <span>${esc(site.license)} License</span>
    <span class="grow"></span>
    <div class="footer-links">
      <a href="${site.repo}" rel="noopener">GitHub</a>
      <a href="${site.releases}" rel="noopener">Releases</a>
      <a href="${site.issues}" rel="noopener">问题反馈</a>
      <a href="${site.prd}">产品 PRD</a>
    </div>
  </div>
</footer>`;

function head(page, isHome, bodyClass = "") {
  const fullTitle = isHome
    ? `${page.title} · Redis 可视化管理平台`
    : `${page.title} · RedisLens 文档`;
  const canonical = `${site.baseUrl}${page.file}`;
  return `<!doctype html>
<html lang="zh-CN" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${esc(fullTitle)}</title>
<meta name="description" content="${esc(page.description)}">
<meta name="theme-color" content="#dc382d">
<link rel="canonical" href="${canonical}">
<link rel="icon" type="image/png" href="assets/logo.png">
<link rel="icon" type="image/svg+xml" href="assets/logo.svg">
<link rel="stylesheet" href="assets/docs.css?v=${esc(site.version)}">
<meta property="og:type" content="website">
<meta property="og:site_name" content="RedisLens 文档">
<meta property="og:title" content="${esc(fullTitle)}">
<meta property="og:description" content="${esc(page.description)}">
<meta property="og:url" content="${canonical}">
<script>try{var t=localStorage.getItem("redislens-docs-theme");if(!t)t=window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.setAttribute("data-theme",t);}catch(e){}</script>
</head>
<body${bodyClass ? ` class="${bodyClass}"` : ""}>`;
}

function scripts() {
  return `<script src="assets/search-index.js?v=${esc(site.version)}"></script>
<script src="assets/docs.js?v=${esc(site.version)}"></script>
</body>
</html>`;
}

function pageMeta(page) {
  const editUrl = `${site.repo}/edit/main/tools/docs-site/pages/${page.src}`;
  return `<div class="page-meta">
  <span>最后更新：${esc(site.updated)}</span>
  <span><a href="${editUrl}" rel="noopener">在 GitHub 上编辑此页</a></span>
  <span><a href="${site.issues}" rel="noopener">报告问题</a></span>
</div>`;
}

function pager(page) {
  const index = allDocs.findIndex((p) => p.file === page.file);
  const prev = index > 0 ? allDocs[index - 1] : null;
  const next = index >= 0 && index < allDocs.length - 1 ? allDocs[index + 1] : null;
  if (!prev && !next) return "";
  return `<nav class="pager" aria-label="文档翻页">
  ${prev ? `<a class="prev" href="${prev.file}"><span class="dir">← 上一篇</span><span class="name">${esc(prev.label)}</span></a>` : "<span></span>"}
  ${next ? `<a class="next" href="${next.file}"><span class="dir">下一篇 →</span><span class="name">${esc(next.label)}</span></a>` : "<span></span>"}
</nav>`;
}

function docLayout(page, content) {
  return `${head(page, false)}
<a class="skip-link" href="#main">跳到正文</a>
${topbar()}
<div class="backdrop" data-backdrop></div>
<div class="layout">
${sidebar(page.file)}
  <div class="content-wrap">
    <main class="content" id="main">
      <nav class="breadcrumb" aria-label="面包屑">
        <a href="index.html">文档</a><span class="sep">/</span><span>${esc(page.section || "参考")}</span><span class="sep">/</span><span>${esc(page.label)}</span>
      </nav>
      <h1>${esc(page.title)}</h1>
${content}
${pageMeta(page)}
${pager(page)}
    </main>
    <aside class="toc" aria-label="本页目录">
      <div class="toc-title">本页目录</div>
      <div data-toc></div>
    </aside>
  </div>
</div>
${footer()}
${scripts()}`;
}

function homeLayout(page, content) {
  return `${head(page, true, "page-home")}
<a class="skip-link" href="#main">跳到正文</a>
${topbar()}
<div class="backdrop" data-backdrop></div>
${sidebar(page.file)}
<main id="main">
${content}
</main>
${footer()}
${scripts()}`;
}

function notFoundLayout(content) {
  const page = { ...notFound, section: "参考" };
  return `${head(page, false)}
<a class="skip-link" href="#main">跳到正文</a>
${topbar()}
<div class="backdrop" data-backdrop></div>
<div class="layout">
${sidebar("")}
  <div class="content-wrap">
    <main class="content" id="main">
      <nav class="breadcrumb" aria-label="面包屑"><a href="index.html">文档</a><span class="sep">/</span><span>${esc(page.label)}</span></nav>
      <h1>${esc(page.title)}</h1>
${content}
    </main>
    <aside class="toc" aria-label="本页目录"><div class="toc-title">本页目录</div><div data-toc></div></aside>
  </div>
</div>
${footer()}
${scripts()}`;
}

/** 从页面片段生成搜索索引条目。 */
function indexPage(page, html) {
  const entries = [];
  const lead = (html.match(/<p class="lead">([\s\S]*?)<\/p>/) || [])[1] || "";
  entries.push({
    p: page.file,
    t: page.label,
    h: page.title,
    a: "",
    k: page.section,
    x: strip(`${page.description} ${lead}`).slice(0, 260),
  });

  const headings = [];
  const re = /<h([23])([^>]*)>([\s\S]*?)<\/h\1>/g;
  let m;
  while ((m = re.exec(html))) {
    const attrs = m[2];
    const id = (attrs.match(/\bid="([^"]+)"/) || [])[1];
    if (!id) continue;
    headings.push({
      level: Number(m[1]),
      id,
      text: strip(m[3]),
      start: m.index,
      end: re.lastIndex,
    });
  }

  let lastH2 = "";
  headings.forEach((h, i) => {
    if (h.level === 2) lastH2 = h.text;
    const next = headings[i + 1];
    const body = html.slice(h.end, next ? next.start : html.length);
    entries.push({
      p: page.file,
      t: page.label,
      h: h.text,
      a: h.id,
      k: h.level === 3 ? lastH2 || page.section : page.section,
      x: strip(body).slice(0, 320),
    });
  });
  return entries;
}

// ---------------- 构建 ----------------

fs.mkdirSync(outDir, { recursive: true });
const searchEntries = [];
const checks = [];

for (const page of pages) {
  const srcPath = path.join(pagesDir, page.src);
  if (!fs.existsSync(srcPath)) {
    throw new Error(`缺少页面片段：${srcPath}`);
  }
  let content = ensureHeadingIds(fs.readFileSync(srcPath, "utf8").trim());
  const html = page.home ? homeLayout(page, content) : docLayout(page, content);
  fs.writeFileSync(path.join(outDir, page.file), `${html}\n`, "utf8");
  searchEntries.push(...indexPage(page, content));
  checks.push(`${page.file}  ${(html.length / 1024).toFixed(1)} KB`);
}

// 404
const notFoundSrc = path.join(pagesDir, notFound.src);
let notFoundBody = ensureHeadingIds(fs.readFileSync(notFoundSrc, "utf8").trim());
fs.writeFileSync(path.join(outDir, notFound.file), `${notFoundLayout(notFoundBody)}\n`, "utf8");
checks.push(`404.html`);

// 搜索索引
fs.writeFileSync(
  path.join(outDir, "assets", "search-index.js"),
  `/* 由 tools/docs-site/build.mjs 生成，请勿手工编辑。 */\nwindow.REDISLENS_SEARCH = ${JSON.stringify(searchEntries)};\n`,
  "utf8"
);

// .nojekyll：关闭 GitHub Pages 的 Jekyll 处理，按静态文件原样发布
fs.writeFileSync(path.join(outDir, ".nojekyll"), "", "utf8");

// sitemap + robots
const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${pages
  .map(
    (p) => `  <url><loc>${site.baseUrl}${p.file}</loc><lastmod>${site.updated}</lastmod></url>`
  )
  .join("\n")}
</urlset>
`;
fs.writeFileSync(path.join(outDir, "sitemap.xml"), sitemap, "utf8");
fs.writeFileSync(
  path.join(outDir, "robots.txt"),
  `User-agent: *\nAllow: /\nSitemap: ${site.baseUrl}sitemap.xml\n`,
  "utf8"
);

console.log(`已生成 ${pages.length} 个文档页 + 404：`);
checks.forEach((line) => console.log(`  ${line}`));
console.log(`  搜索索引 ${searchEntries.length} 条`);
console.log(`输出目录：${outDir}`);
