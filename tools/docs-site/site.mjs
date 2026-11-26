// RedisLens 文档站点配置。修改导航或页面元信息后运行 `node tools/docs-site/build.mjs`。
export const site = {
  name: "RedisLens",
  repo: "https://github.com/flairamos/RedisLens",
  issues: "https://github.com/flairamos/RedisLens/issues",
  releases: "https://github.com/flairamos/RedisLens/releases",
  version: "1.0.0",
  license: "Apache-2.0",
  copyright: "2026 flairamos",
  updated: "2026-11-26",
  baseUrl: "https://flairamos.github.io/RedisLens/",
  prd: "RedisLens产品需求文档PRD.docx",
};

export const sections = [
  {
    title: "开始",
    items: [
      {
        file: "index.html",
        src: "01-index.html",
        label: "产品概览",
        title: "RedisLens 文档",
        h1: "RedisLens：让 Redis 数据看得见、改得动、管得住",
        description:
          "RedisLens 是基于 Go + Wails v2 + Vue 3 的 Windows 桌面版 Redis 可视化管理平台，提供连接管理、键空间浏览编辑、监控、控制台、分析、导入导出与审计能力。",
        home: true,
      },
      {
        file: "getting-started.html",
        src: "02-getting-started.html",
        label: "安装与首次连接",
        title: "安装与首次连接",
        description:
          "下载安装 RedisLens，配置第一个 Redis 连接，并了解数据目录、连接状态与常用入口。",
      },
    ],
  },
  {
    title: "使用指南",
    items: [
      {
        file: "connections.html",
        src: "03-connections.html",
        label: "连接管理",
        title: "连接管理",
        description:
          "配置单机、哨兵、集群、SSH 隧道、TLS 与只读模式，管理多实例分组、健康检查与自动重连。",
      },
      {
        file: "browser.html",
        src: "04-browser.html",
        label: "数据浏览与编辑",
        title: "数据浏览与编辑",
        description:
          "使用键空间树与 SCAN 游标浏览键，查看并编辑 String、Hash、List、Set、ZSet、Stream 与 RedisJSON，管理 TTL 与删除恢复。",
      },
      {
        file: "dashboard.html",
        src: "05-dashboard.html",
        label: "监控仪表盘",
        title: "监控仪表盘",
        description:
          "理解 RedisLens 的 INFO 采样机制、核心指标卡片、QPS / 命中率 / 内存 / 网络 IO 曲线与时间窗口。",
      },
      {
        file: "console.html",
        src: "06-console.html",
        label: "命令行控制台",
        title: "命令行控制台",
        description:
          "使用语法高亮、命令与键名补全、会话历史、危险命令拦截与二次确认执行 Redis 命令。",
      },
      {
        file: "analysis.html",
        src: "07-analysis.html",
        label: "键空间分析",
        title: "键空间分析",
        description:
          "通过渐进式 SCAN 发现大 Key，查看类型与 TTL 分布、慢查询日志和热命令统计。",
      },
      {
        file: "data-transfer.html",
        src: "08-data-transfer.html",
        label: "导入导出与批量删除",
        title: "导入导出与批量删除",
        description:
          "导出 JSON（含 DUMP 保真数据）或 CSV，跨实例导入恢复，以及在预览确认后批量删除键。",
      },
      {
        file: "pubsub.html",
        src: "09-pubsub.html",
        label: "发布订阅",
        title: "发布订阅",
        description:
          "订阅频道与模式、查看实时消息流、暂停接收与发布消息，并了解 Pub/Sub 的适用边界。",
      },
      {
        file: "topology.html",
        src: "10-topology.html",
        label: "实例拓扑",
        title: "实例拓扑",
        description:
          "自动发现单机主从、哨兵与集群分片拓扑，查看节点角色、状态、槽位与分片结构。",
      },
    ],
  },
  {
    title: "参考",
    items: [
      {
        file: "security.html",
        src: "11-security.html",
        label: "安全与审计",
        title: "安全与审计",
        description:
          "了解凭据本地加密、只读模式、危险命令拦截、审计日志、回收站与数据目录的安全边界。",
      },
      {
        file: "settings.html",
        src: "12-settings.html",
        label: "设置与快捷键",
        title: "设置与快捷键",
        description:
          "主题、列表密度、采样间隔、SCAN COUNT、日志开关等设置项，以及全局快捷键速查。",
      },
      {
        file: "faq.html",
        src: "13-faq.html",
        label: "常见问题",
        title: "常见问题与故障排查",
        description:
          "连接失败、只读拦截、集群 DB0、导入失败、回收站丢失、凭据换机等常见问题的排查思路。",
      },
      {
        file: "development.html",
        src: "14-development.html",
        label: "开发者指南",
        title: "开发者指南",
        description:
          "从源码构建 RedisLens、运行测试、了解目录结构，以及通过 docs 目录部署 GitHub Pages 的步骤。",
      },
    ],
  },
];

export const notFound = {
  file: "404.html",
  src: "15-404.html",
  label: "页面未找到",
  title: "页面未找到",
  description: "你访问的文档页面不存在或已移动。",
};
