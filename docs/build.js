#!/usr/bin/env node
/**
 * ShoreDB docs — static site generator.
 *
 * No dependencies beyond Node's standard library. To add a page:
 *   1. Create content/your-page.md with a frontmatter block:
 *
 *        ---
 *        title: Your Page
 *        section: Getting Started | Commands | Guides | Reference
 *        order: 4
 *        ---
 *
 *   2. Write Markdown below the frontmatter (headings, code fences, tables,
 *      lists, links, bold/italic/inline code are all supported).
 *   3. Run `node build.js`. dist/your-page.html is generated automatically
 *      and the nav / search index update to include it.
 */
const fs = require("fs");
const path = require("path");

const CONTENT_DIR = path.join(__dirname, "content");
const DIST_DIR = path.join(__dirname, "dist");
const ASSETS_DIR = path.join(__dirname, "assets");

const SECTION_ORDER = ["Getting Started", "Commands", "Guides", "Reference"];

// ---------------------------------------------------------------------------
// Frontmatter
// ---------------------------------------------------------------------------
function parseFrontmatter(raw) {
  const match = raw.match(/^---\n([\s\S]*?)\n---\n?([\s\S]*)$/);
  if (!match) return { meta: {}, body: raw };
  const meta = {};
  for (const line of match[1].split("\n")) {
    const idx = line.indexOf(":");
    if (idx === -1) continue;
    const key = line.slice(0, idx).trim();
    let value = line.slice(idx + 1).trim();
    meta[key] = value;
  }
  return { meta, body: match[2] };
}

// ---------------------------------------------------------------------------
// Minimal Markdown -> HTML converter (subset sufficient for these docs)
// ---------------------------------------------------------------------------
function escapeHtml(s) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function slugify(text) {
  return text
    .toLowerCase()
    .replace(/`/g, "")
    .replace(/[^a-z0-9\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-");
}

// Renders inline markdown: **bold**, `code`, [text](url), *italic*
function renderInline(text) {
  let out = "";
  let i = 0;
  const n = text.length;
  while (i < n) {
    const ch = text[i];

    if (ch === "`") {
      const end = text.indexOf("`", i + 1);
      if (end !== -1) {
        out += `<code>${escapeHtml(text.slice(i + 1, end))}</code>`;
        i = end + 1;
        continue;
      }
    }

    if (ch === "*" && text[i + 1] === "*") {
      const end = text.indexOf("**", i + 2);
      if (end !== -1) {
        out += `<strong>${renderInline(text.slice(i + 2, end))}</strong>`;
        i = end + 2;
        continue;
      }
    }

    if (ch === "*") {
      const end = text.indexOf("*", i + 1);
      if (end !== -1) {
        out += `<em>${renderInline(text.slice(i + 1, end))}</em>`;
        i = end + 1;
        continue;
      }
    }

    if (ch === "[") {
      const closeBracket = text.indexOf("]", i + 1);
      if (closeBracket !== -1 && text[closeBracket + 1] === "(") {
        const closeParen = text.indexOf(")", closeBracket + 2);
        if (closeParen !== -1) {
          const label = text.slice(i + 1, closeBracket);
          const href = text.slice(closeBracket + 2, closeParen);
          out += `<a href="${escapeHtml(href)}">${renderInline(label)}</a>`;
          i = closeParen + 1;
          continue;
        }
      }
    }

    out += escapeHtml(ch);
    i++;
  }
  return out;
}

function renderMarkdown(md, headingCollector) {
  const lines = md.split("\n");
  const html = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Fenced code block
    if (line.trim().startsWith("```")) {
      const lang = line.trim().slice(3).trim();
      const buf = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) {
        buf.push(lines[i]);
        i++;
      }
      i++; // skip closing fence
      const langClass = lang ? ` data-lang="${escapeHtml(lang)}" class="language-${escapeHtml(lang)}"` : "";
      html.push(
        `<pre><code${langClass}>${escapeHtml(buf.join("\n"))}</code></pre>`
      );
      continue;
    }

    // Headings, e.g. "## Title" or "## `Title` {#custom-id}"
    const headingMatch = line.match(/^(#{1,3})\s+(.*)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      let text = headingMatch[2].trim();
      let customId = null;
      const idMatch = text.match(/\s*\{#([a-z0-9-]+)\}\s*$/i);
      if (idMatch) {
        customId = idMatch[1];
        text = text.slice(0, idMatch.index).trim();
      }
      const id = customId || slugify(text);
      if (headingCollector && level <= 3) {
        headingCollector.push({ level, text, id });
      }
      html.push(`<h${level} id="${id}"><a class="anchor" href="#${id}">#</a>${renderInline(text)}</h${level}>`);
      i++;
      continue;
    }

    // Blockquote
    if (line.trim().startsWith(">")) {
      const buf = [];
      while (i < lines.length && lines[i].trim().startsWith(">")) {
        buf.push(lines[i].trim().replace(/^>\s?/, ""));
        i++;
      }
      html.push(`<blockquote><p>${renderInline(buf.join(" "))}</p></blockquote>`);
      continue;
    }

    // Table
    if (line.trim().startsWith("|") && lines[i + 1] && /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1])) {
      const headerCells = line
        .trim()
        .replace(/^\||\|$/g, "")
        .split("|")
        .map((c) => c.trim());
      i += 2;
      const rows = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) {
        const cells = lines[i]
          .trim()
          .replace(/^\||\|$/g, "")
          .split("|")
          .map((c) => c.trim());
        rows.push(cells);
        i++;
      }
      let table = '<div class="table-wrap"><table><thead><tr>';
      for (const cell of headerCells) table += `<th>${renderInline(cell)}</th>`;
      table += "</tr></thead><tbody>";
      for (const row of rows) {
        table += "<tr>";
        for (const cell of row) table += `<td>${renderInline(cell)}</td>`;
        table += "</tr>";
      }
      table += "</tbody></table></div>";
      html.push(table);
      continue;
    }

    // Unordered list
    if (/^\s*-\s+/.test(line)) {
      const items = [];
      while (i < lines.length && /^\s*-\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*-\s+/, ""));
        i++;
      }
      html.push(`<ul>${items.map((it) => `<li>${renderInline(it)}</li>`).join("")}</ul>`);
      continue;
    }

    // Ordered list
    if (/^\s*\d+\.\s+/.test(line)) {
      const items = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, ""));
        i++;
      }
      html.push(`<ol>${items.map((it) => `<li>${renderInline(it)}</li>`).join("")}</ol>`);
      continue;
    }

    // Horizontal rule
    if (/^\s*---+\s*$/.test(line)) {
      html.push("<hr>");
      i++;
      continue;
    }

    // Blank line
    if (line.trim() === "") {
      i++;
      continue;
    }

    // Paragraph (collect contiguous non-blank lines)
    const buf = [line];
    i++;
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^(#{1,3})\s+/.test(lines[i]) &&
      !lines[i].trim().startsWith("```") &&
      !lines[i].trim().startsWith("|") &&
      !/^\s*-\s+/.test(lines[i]) &&
      !/^\s*\d+\.\s+/.test(lines[i]) &&
      !lines[i].trim().startsWith(">") &&
      !/^\s*---+\s*$/.test(lines[i])
    ) {
      buf.push(lines[i]);
      i++;
    }
    html.push(`<p>${renderInline(buf.join(" "))}</p>`);
  }

  return html.join("\n");
}

// ---------------------------------------------------------------------------
// Load all pages
// ---------------------------------------------------------------------------
function loadPages() {
  const files = fs.readdirSync(CONTENT_DIR).filter((f) => f.endsWith(".md"));
  const pages = files.map((file) => {
    const raw = fs.readFileSync(path.join(CONTENT_DIR, file), "utf8");
    const { meta, body } = parseFrontmatter(raw);
    const slug = file.replace(/\.md$/, "");
    const headings = [];
    const contentHtml = renderMarkdown(body, headings);
    return {
      slug,
      url: `${slug}.html`,
      title: meta.title || slug,
      section: meta.section || "Reference",
      order: Number(meta.order || 999),
      tagline: meta.tagline || "",
      headings,
      contentHtml,
      excerpt: body.replace(/[#>*`|-]/g, "").trim().slice(0, 160),
    };
  });

  pages.sort((a, b) => {
    const sa = SECTION_ORDER.indexOf(a.section);
    const sb = SECTION_ORDER.indexOf(b.section);
    if (sa !== sb) return sa - sb;
    return a.order - b.order;
  });

  return pages;
}

// ---------------------------------------------------------------------------
// Render nav / TOC / page shell
// ---------------------------------------------------------------------------
function renderNav(pages, activeSlug) {
  const bySection = {};
  for (const p of pages) {
    (bySection[p.section] = bySection[p.section] || []).push(p);
  }
  let html = "";
  for (const section of SECTION_ORDER) {
    if (!bySection[section]) continue;
    html += `<div class="nav-section"><h3>${section}</h3><ul>`;
    for (const p of bySection[section]) {
      const active = p.slug === activeSlug ? ' class="active" aria-current="page"' : "";
      html += `<li><a href="${p.url}"${active}>${escapeHtml(p.title)}</a></li>`;
    }
    html += `</ul></div>`;
  }
  return html;
}

function renderToc(headings) {
  const usable = headings.filter((h) => h.level >= 2);
  if (usable.length === 0) return "";
  let html = '<nav class="page-toc"><h4>On this page</h4><ul>';
  for (const h of usable) {
    html += `<li class="toc-level-${h.level}"><a href="#${h.id}">${renderInline(h.text)}</a></li>`;
  }
  html += "</ul></nav>";
  return html;
}

function renderPage(page, allPages, template) {
  const nav = renderNav(allPages, page.slug);
  const toc = renderToc(page.headings);
  const idx = allPages.findIndex((p) => p.slug === page.slug);
  const prev = idx > 0 ? allPages[idx - 1] : null;
  const next = idx < allPages.length - 1 ? allPages[idx + 1] : null;

  let pager = '<div class="pager">';
  pager += prev
    ? `<a class="pager-link pager-prev" href="${prev.url}"><span>Previous</span><strong>${escapeHtml(prev.title)}</strong></a>`
    : "<span></span>";
  pager += next
    ? `<a class="pager-link pager-next" href="${next.url}"><span>Next</span><strong>${escapeHtml(next.title)}</strong></a>`
    : "<span></span>";
  pager += "</div>";

  return template
    .replace(/{{TITLE}}/g, escapeHtml(page.title))
    .replace(/{{NAV}}/g, nav)
    .replace(/{{TOC}}/g, toc)
    .replace(/{{CONTENT}}/g, page.contentHtml)
    .replace(/{{PAGER}}/g, pager)
    .replace(/{{IS_HOME}}/g, page.slug === "index" ? "is-home" : "");
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
function build() {
  if (!fs.existsSync(DIST_DIR)) fs.mkdirSync(DIST_DIR, { recursive: true });

  const template = fs.readFileSync(path.join(__dirname, "template.html"), "utf8");
  const pages = loadPages();

  for (const page of pages) {
    const html = renderPage(page, pages, template);
    fs.writeFileSync(path.join(DIST_DIR, page.url), html, "utf8");
  }

  // search index
  const searchIndex = pages.map((p) => ({
    title: p.title,
    url: p.url,
    section: p.section,
    excerpt: p.excerpt,
    headings: p.headings.map((h) => h.text),
  }));
  fs.writeFileSync(
    path.join(DIST_DIR, "search-index.json"),
    JSON.stringify(searchIndex, null, 2),
    "utf8"
  );

  // copy assets
  const distAssets = path.join(DIST_DIR, "assets");
  if (!fs.existsSync(distAssets)) fs.mkdirSync(distAssets, { recursive: true });
  for (const file of fs.readdirSync(ASSETS_DIR)) {
    fs.copyFileSync(path.join(ASSETS_DIR, file), path.join(distAssets, file));
  }

  console.log(`Built ${pages.length} pages into ${path.relative(process.cwd(), DIST_DIR)}/`);
}

build();
