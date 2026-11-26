/* RedisLens 文档站点交互：主题、移动端导航、目录、代码复制、全文搜索。 */
(function () {
  "use strict";

  var THEME_KEY = "redislens-docs-theme";
  var doc = document;

  /* ------------------------------------------------------------------ */
  /* 主题                                                                */
  /* ------------------------------------------------------------------ */

  var SUN =
    '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4"></circle><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"></path></svg>';
  var MOON =
    '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>';

  function currentTheme() {
    return doc.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
  }

  function applyTheme(theme) {
    doc.documentElement.setAttribute("data-theme", theme);
    var btn = doc.querySelector("[data-theme-toggle]");
    if (btn) {
      btn.innerHTML = theme === "dark" ? SUN : MOON;
      btn.setAttribute("aria-label", theme === "dark" ? "切换到浅色主题" : "切换到深色主题");
      btn.setAttribute("title", theme === "dark" ? "切换到浅色主题" : "切换到深色主题");
    }
  }

  function initTheme() {
    var stored = null;
    try {
      stored = localStorage.getItem(THEME_KEY);
    } catch (e) {
      /* localStorage 不可用时忽略 */
    }
    var initial = stored || currentTheme();
    applyTheme(initial);

    var btn = doc.querySelector("[data-theme-toggle]");
    if (btn) {
      btn.addEventListener("click", function () {
        var next = currentTheme() === "dark" ? "light" : "dark";
        applyTheme(next);
        try {
          localStorage.setItem(THEME_KEY, next);
        } catch (e) {
          /* ignore */
        }
      });
    }

    if (!stored && window.matchMedia) {
      var mq = window.matchMedia("(prefers-color-scheme: dark)");
      var onChange = function (e) {
        applyTheme(e.matches ? "dark" : "light");
      };
      if (mq.addEventListener) mq.addEventListener("change", onChange);
      else if (mq.addListener) mq.addListener(onChange);
    }
  }

  /* ------------------------------------------------------------------ */
  /* 移动端导航                                                          */
  /* ------------------------------------------------------------------ */

  function initNav() {
    var menu = doc.querySelector("[data-menu-toggle]");
    var backdrop = doc.querySelector("[data-backdrop]");
    var sidebar = doc.querySelector(".sidebar");

    function close() {
      doc.body.classList.remove("nav-open");
      if (menu) menu.setAttribute("aria-expanded", "false");
    }

    if (menu) {
      menu.addEventListener("click", function () {
        var open = doc.body.classList.toggle("nav-open");
        menu.setAttribute("aria-expanded", open ? "true" : "false");
      });
    }
    if (backdrop) backdrop.addEventListener("click", close);
    if (sidebar) {
      sidebar.addEventListener("click", function (e) {
        if (e.target.closest("a")) close();
      });
    }
    doc.addEventListener("keydown", function (e) {
      if (e.key === "Escape") close();
    });
  }

  /* ------------------------------------------------------------------ */
  /* 标题锚点与目录                                                      */
  /* ------------------------------------------------------------------ */

  function slugify(text) {
    return (
      String(text)
        .trim()
        .toLowerCase()
        .replace(/&[a-z]+;/g, "")
        .replace(/[^\w\u4e00-\u9fa5]+/g, "-")
        .replace(/^-+|-+$/g, "") || "section"
    );
  }

  function initHeadings() {
    var content = doc.querySelector(".content");
    if (!content) return;
    var used = {};
    content.querySelectorAll("h2, h3").forEach(function (h) {
      if (!h.id) {
        var base = slugify(h.textContent);
        var id = base;
        var n = 2;
        while (used[id] || doc.getElementById(id)) {
          id = base + "-" + n++;
        }
        h.id = id;
      }
      used[h.id] = true;
      if (!h.querySelector(".anchor")) {
        var a = doc.createElement("a");
        a.className = "anchor";
        a.href = "#" + h.id;
        a.setAttribute("aria-label", "此节的链接");
        a.textContent = "#";
        h.appendChild(a);
      }
    });
  }

  function initToc() {
    var toc = doc.querySelector("[data-toc]");
    var content = doc.querySelector(".content");
    if (!toc || !content) return;

    var headings = Array.prototype.filter.call(
      content.querySelectorAll("h2, h3"),
      function (h) {
        return h.id && !h.closest(".card, .no-toc");
      }
    );
    if (headings.length < 2) {
      var aside = toc.closest(".toc");
      if (aside) aside.style.display = "none";
      return;
    }

    var nav = doc.createElement("nav");
    headings.forEach(function (h) {
      var a = doc.createElement("a");
      a.href = "#" + h.id;
      a.textContent = h.textContent.replace(/#\s*$/, "").trim();
      a.className = h.tagName === "H3" ? "depth-3" : "depth-2";
      nav.appendChild(a);
    });
    toc.appendChild(nav);

    var links = Array.prototype.slice.call(nav.querySelectorAll("a"));
    var ticking = false;

    function update() {
      ticking = false;
      var offset = 110;
      var active = links[0];
      for (var i = 0; i < headings.length; i++) {
        if (headings[i].getBoundingClientRect().top <= offset) {
          active = links[i];
        } else {
          break;
        }
      }
      if (window.scrollY + window.innerHeight >= doc.documentElement.scrollHeight - 4) {
        active = links[links.length - 1];
      }
      links.forEach(function (a) {
        a.classList.toggle("active", a === active);
      });
    }

    function onScroll() {
      if (!ticking) {
        ticking = true;
        window.requestAnimationFrame(update);
      }
    }

    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    update();
  }

  /* ------------------------------------------------------------------ */
  /* 代码复制                                                            */
  /* ------------------------------------------------------------------ */

  function initCopy() {
    doc.querySelectorAll(".content pre").forEach(function (pre) {
      var block = doc.createElement("div");
      block.className = "code-block";
      pre.parentNode.insertBefore(block, pre);
      block.appendChild(pre);

      var btn = doc.createElement("button");
      btn.type = "button";
      btn.className = "copy-btn";
      btn.textContent = "复制";
      btn.setAttribute("aria-label", "复制代码");
      block.appendChild(btn);

      btn.addEventListener("click", function () {
        var text = pre.innerText;
        var done = function () {
          btn.textContent = "已复制";
          btn.classList.add("copied");
          window.setTimeout(function () {
            btn.textContent = "复制";
            btn.classList.remove("copied");
          }, 1600);
        };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(done, fallback);
        } else {
          fallback();
        }
        function fallback() {
          var ta = doc.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed";
          ta.style.opacity = "0";
          doc.body.appendChild(ta);
          ta.select();
          try {
            doc.execCommand("copy");
            done();
          } catch (e) {
            /* ignore */
          }
          doc.body.removeChild(ta);
        }
      });
    });
  }

  /* ------------------------------------------------------------------ */
  /* 外链                                                                */
  /* ------------------------------------------------------------------ */

  function initExternalLinks() {
    doc.querySelectorAll('a[href^="http"]').forEach(function (a) {
      if (a.host && a.host !== window.location.host) {
        a.target = "_blank";
        a.rel = "noopener noreferrer";
      }
    });
  }

  /* ------------------------------------------------------------------ */
  /* 搜索                                                                */
  /* ------------------------------------------------------------------ */

  var search = {
    overlay: null,
    input: null,
    results: null,
    items: [],
    active: 0,
    loaded: false,
  };

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function highlight(text, tokens) {
    var safe = escapeHtml(text);
    if (!tokens.length) return safe;
    var pattern = tokens
      .filter(function (t) {
        return t.length > 0;
      })
      .map(function (t) {
        return t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      })
      .join("|");
    if (!pattern) return safe;
    return safe.replace(new RegExp("(" + pattern + ")", "gi"), "<mark>$1</mark>");
  }

  function buildOverlay() {
    var el = doc.createElement("div");
    el.className = "search-overlay";
    el.hidden = true;
    el.innerHTML =
      '<div class="search-backdrop" data-search-close></div>' +
      '<div class="search-panel" role="dialog" aria-modal="true" aria-label="搜索文档">' +
      '  <div class="search-head">' +
      '    <svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"></circle><path d="m20 20-3.2-3.2"></path></svg>' +
      '    <input id="docs-search-input" type="search" placeholder="搜索功能、配置、命令或错误…" autocomplete="off" spellcheck="false" aria-label="搜索文档">' +
      "    <kbd>ESC</kbd>" +
      "  </div>" +
      '  <div class="search-results" id="docs-search-results"></div>' +
      '  <div class="search-foot"><span><kbd>↑</kbd> <kbd>↓</kbd> 选择</span><span><kbd>Enter</kbd> 打开</span><span><kbd>Ctrl</kbd> + <kbd>K</kbd> 随时唤起</span></div>' +
      "</div>";
    doc.body.appendChild(el);

    search.overlay = el;
    search.input = el.querySelector("#docs-search-input");
    search.results = el.querySelector("#docs-search-results");

    el.addEventListener("click", function (e) {
      if (e.target.hasAttribute("data-search-close")) closeSearch();
    });
    search.input.addEventListener("input", function () {
      renderSearch(search.input.value);
    });
    search.input.addEventListener("keydown", onSearchKey);
    el.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        e.preventDefault();
        closeSearch();
      }
    });
  }

  function openSearch() {
    if (!search.overlay) buildOverlay();
    search.overlay.hidden = false;
    doc.body.style.overflow = "hidden";
    search.input.value = "";
    search.active = 0;
    renderSearch("");
    window.setTimeout(function () {
      search.input.focus();
    }, 20);
  }

  function closeSearch() {
    if (!search.overlay) return;
    search.overlay.hidden = true;
    doc.body.style.overflow = "";
  }

  function searchData() {
    return Array.isArray(window.REDISLENS_SEARCH) ? window.REDISLENS_SEARCH : [];
  }

  function scoreEntry(entry, tokens, phrase) {
    var h = (entry.h + " " + (entry.k || "")).toLowerCase();
    var body = (entry.x || "").toLowerCase();
    var title = (entry.t || "").toLowerCase();
    var score = 0;
    for (var i = 0; i < tokens.length; i++) {
      var t = tokens[i];
      var inH = h.indexOf(t);
      var inB = body.indexOf(t);
      var inT = title.indexOf(t);
      if (inH < 0 && inB < 0 && inT < 0) return -1;
      if (inH === 0) score += 80;
      else if (inH > 0) score += 50;
      if (inT >= 0) score += 18;
      if (inB >= 0) score += 8;
    }
    if (phrase && h.indexOf(phrase) >= 0) score += 40;
    if (entry.a === "") score += 4; // 页面概览稍优先
    return score;
  }

  function renderSearch(query) {
    if (!search.results) return;
    var q = String(query || "").trim();
    var tokens = q.toLowerCase().split(/\s+/).filter(Boolean);
    var list = [];

    if (!tokens.length) {
      // 空查询：展示常用入口
      list = searchData()
        .filter(function (e) {
          return e.a === "";
        })
        .slice(0, 8);
      search.items = list;
      search.active = 0;
      var html = '<div class="search-group-label">文档入口</div>';
      list.forEach(function (e, i) {
        html += renderItem(e, [], i);
      });
      search.results.innerHTML =
        html +
        '<div class="search-empty">输入<strong>键空间</strong>、<strong>SSH</strong>、<strong>危险命令</strong>等关键词开始搜索</div>';
      return;
    }

    var phrase = q.toLowerCase();
    var scored = [];
    searchData().forEach(function (e) {
      var s = scoreEntry(e, tokens, phrase);
      if (s >= 0) scored.push({ e: e, s: s });
    });
    scored.sort(function (a, b) {
      return b.s - a.s;
    });
    list = scored.slice(0, 24).map(function (x) {
      return x.e;
    });
    search.items = list;
    search.active = list.length ? 0 : -1;

    if (!list.length) {
      search.results.innerHTML =
        '<div class="search-empty">没有找到与 <strong>' +
        escapeHtml(q) +
        "</strong> 相关的内容。<br>试试更短的关键词，或检查拼写。</div>";
      return;
    }

    var out = '<div class="search-group-label">' + list.length + " 条结果</div>";
    list.forEach(function (e, i) {
      out += renderItem(e, tokens, i);
    });
    search.results.innerHTML = out;
  }

  function renderItem(e, tokens, index) {
    var title = e.a ? e.h : e.t;
    var meta = e.a ? e.t + " · " + (e.k || "") : e.k || "文档";
    var snippet = e.x ? e.x.slice(0, 150) : "";
    return (
      '<a class="search-item' +
      (index === search.active ? " active" : "") +
      '" href="' +
      escapeHtml(e.p) +
      (e.a ? "#" + escapeHtml(e.a) : "") +
      '" data-index="' +
      index +
      '">' +
      '<div class="search-item-title">' +
      highlight(title, tokens) +
      "</div>" +
      '<div class="search-item-meta">' +
      highlight(meta, tokens) +
      "</div>" +
      (snippet
        ? '<div class="search-item-snippet">' + highlight(snippet, tokens) + "</div>"
        : "") +
      "</a>"
    );
  }

  function setActive(index) {
    if (!search.items.length) return;
    if (index < 0) index = search.items.length - 1;
    if (index >= search.items.length) index = 0;
    search.active = index;
    var els = search.results.querySelectorAll(".search-item");
    els.forEach(function (el, i) {
      el.classList.toggle("active", i === index);
    });
    var activeEl = els[index];
    if (activeEl && activeEl.scrollIntoView) {
      activeEl.scrollIntoView({ block: "nearest" });
    }
  }

  function onSearchKey(e) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive(search.active + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive(search.active - 1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      var item = search.items[search.active];
      if (item) {
        window.location.href = item.p + (item.a ? "#" + item.a : "");
      }
    }
  }

  function initSearch() {
    buildOverlay();

    doc.querySelectorAll("[data-search-open]").forEach(function (btn) {
      btn.addEventListener("click", openSearch);
    });

    doc.addEventListener("click", function (e) {
      var link = e.target.closest(".search-item");
      if (link) closeSearch();
    });

    doc.addEventListener("keydown", function (e) {
      var tag = (e.target.tagName || "").toLowerCase();
      var typing =
        tag === "input" ||
        tag === "textarea" ||
        tag === "select" ||
        e.target.isContentEditable;

      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        if (search.overlay && !search.overlay.hidden) closeSearch();
        else openSearch();
        return;
      }
      if (e.key === "/" && !typing && (!search.overlay || search.overlay.hidden)) {
        e.preventDefault();
        openSearch();
      }
    });
  }

  /* ------------------------------------------------------------------ */
  /* 启动                                                                */
  /* ------------------------------------------------------------------ */

  function init() {
    initTheme();
    initNav();
    initHeadings();
    initToc();
    initCopy();
    initExternalLinks();
    initSearch();
  }

  if (doc.readyState === "loading") {
    doc.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
