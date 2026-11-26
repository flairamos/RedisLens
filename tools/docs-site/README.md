# RedisLens 文档站

GitHub Pages 文档站的源码与构建脚本。线上地址：<https://flairamos.github.io/RedisLens/>

站点保持零运行时依赖：输出是纯 HTML / CSS / JS，不依赖 npm 包或前端框架。
构建脚本只使用 Node.js 内置模块。

## 构建

```bash
node tools/docs-site/build.mjs
```

构建会：

- 读取 `pages/*.html` 正文片段，套用 `site.mjs` 中的导航与布局；
- 为缺少 `id` 的 `h2` / `h3` 补齐锚点；
- 生成 `docs/assets/search-index.js`、`docs/sitemap.xml`、`docs/robots.txt`；
- 写入空的 `docs/.nojekyll`，关闭 GitHub Pages 的 Jekyll 处理。

预览（任选其一）：

```bash
# 直接打开 docs/index.html，或起一个本地静态服务器
python -m http.server 8765 --directory docs
# http://127.0.0.1:8765/
```

检查内部链接与锚点：

```bash
node tools/docs-site/check-links.mjs
```

## 目录

```text
tools/docs-site/
├── site.mjs              # 站点元信息、导航分组、页面标题与描述
├── build.mjs             # 布局、锚点、搜索索引、sitemap 生成
├── check-links.mjs       # 本地文件与页内锚点检查
└── pages/
    ├── 01-index.html     # 首页（独立全宽布局）
    ├── 02-getting-started.html
    ├── ...
    └── 15-404.html
```

页面片段只写正文，不要包含 `<html>`、顶栏、侧栏或页脚。
样式与交互在 `docs/assets/docs.css`、`docs/assets/docs.js`，由构建结果直接引用。

## 新增页面

1. 在 `pages/` 新建 HTML 片段，通常以 `<p class="lead">` 开始。
2. 在 `site.mjs` 的对应分组登记：

   ```js
   {
     file: "new-page.html",
     src: "16-new-page.html",
     label: "导航名称",
     title: "页面标题",
     description: "用于 meta description 与搜索摘要。"
   }
   ```

3. 运行 `node tools/docs-site/build.mjs`，再运行链接检查。
4. 本地预览导航、目录、搜索与窄屏布局。
5. 同时提交 `tools/docs-site/pages/`、`site.mjs` 与生成的 `docs/`。

## 部署到 GitHub Pages

仓库 **Settings → Pages**：

- **Source**：`Deploy from a branch`
- **Branch**：`main`
- **Folder**：`/docs`
- 点击 **Save**

首次发布由 GitHub 的 `pages build and deployment` 工作流完成，通常 1–3 分钟。
`docs/.nojekyll` 会让 Pages 跳过 Jekyll，直接发布静态文件。

## 注意

- `docs/` 是构建输出，不要直接编辑其中的 HTML；改源片段后重新构建。
- 站点资源全部使用相对路径，因此既可以部署在 `/<repo>/` 子路径，也可以用自定义域名。
- `site.mjs` 中的 `baseUrl` 用于 canonical、sitemap 与 robots；仓库改名或绑定自定义域名后请同步修改。
- 文档内容应与 v1.0.0 的实际行为一致。修改功能行为时，请同步更新对应页面。
