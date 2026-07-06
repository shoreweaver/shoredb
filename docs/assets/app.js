(function () {
  "use strict";

  /* ---------------- Mobile nav ---------------- */
  var menuToggle = document.getElementById("menuToggle");
  var scrim = document.getElementById("sidebarScrim");

  function closeNav() {
    document.body.classList.remove("nav-open");
    if (menuToggle) menuToggle.setAttribute("aria-expanded", "false");
  }
  function toggleNav() {
    var open = document.body.classList.toggle("nav-open");
    if (menuToggle) menuToggle.setAttribute("aria-expanded", open ? "true" : "false");
  }
  if (menuToggle) menuToggle.addEventListener("click", toggleNav);
  if (scrim) scrim.addEventListener("click", closeNav);
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeNav();
  });

  /* Close mobile nav after navigating */
  var sidebar = document.getElementById("sidebar");
  if (sidebar) {
    sidebar.addEventListener("click", function (e) {
      if (e.target.tagName === "A") closeNav();
    });
  }

  /* ---------------- Scroll-spy for the "on this page" TOC ---------------- */
  var tocLinks = Array.prototype.slice.call(document.querySelectorAll(".page-toc a"));
  if (tocLinks.length) {
    var headingEls = tocLinks
      .map(function (a) {
        var id = a.getAttribute("href").slice(1);
        return document.getElementById(id);
      })
      .filter(Boolean);

    var setActive = function (id) {
      tocLinks.forEach(function (a) {
        a.classList.toggle("active", a.getAttribute("href") === "#" + id);
      });
    };

    if ("IntersectionObserver" in window && headingEls.length) {
      var visible = new Set();
      var observer = new IntersectionObserver(
        function (entries) {
          entries.forEach(function (entry) {
            if (entry.isIntersecting) visible.add(entry.target.id);
            else visible.delete(entry.target.id);
          });
          if (visible.size) {
            // pick the topmost visible heading, in document order
            for (var i = 0; i < headingEls.length; i++) {
              if (visible.has(headingEls[i].id)) {
                setActive(headingEls[i].id);
                break;
              }
            }
          }
        },
        { rootMargin: "-72px 0px -70% 0px", threshold: 0 }
      );
      headingEls.forEach(function (el) { observer.observe(el); });
    }
  }

  /* ---------------- Search ---------------- */
  var searchInput = document.getElementById("searchInput");
  var searchResults = document.getElementById("searchResults");
  var searchIndex = null;
  var indexPromise = null;

  function loadIndex() {
    if (!indexPromise) {
      indexPromise = fetch("search-index.json")
        .then(function (r) { return r.json(); })
        .then(function (data) { searchIndex = data; return data; })
        .catch(function () { searchIndex = []; return []; });
    }
    return indexPromise;
  }

  function renderResults(items, query) {
    if (!items.length) {
      searchResults.innerHTML = '<div class="result-empty">No results for "' + escapeHtml(query) + '"</div>';
      searchResults.hidden = false;
      return;
    }
    searchResults.innerHTML = items
      .slice(0, 8)
      .map(function (p) {
        return (
          '<a href="' + p.url + '">' +
          '<span class="result-title">' + escapeHtml(p.title) + "</span>" +
          '<span class="result-meta">' + escapeHtml(p.section) + "</span>" +
          "</a>"
        );
      })
      .join("");
    searchResults.hidden = false;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  function runSearch(query) {
    if (!query) {
      searchResults.hidden = true;
      searchResults.innerHTML = "";
      return;
    }
    var q = query.toLowerCase();
    var scored = searchIndex
      .map(function (p) {
        var haystack = (p.title + " " + p.section + " " + p.excerpt + " " + p.headings.join(" ")).toLowerCase();
        var titleHit = p.title.toLowerCase().indexOf(q) !== -1;
        var headingHit = p.headings.join(" ").toLowerCase().indexOf(q) !== -1;
        var hit = haystack.indexOf(q) !== -1;
        if (!hit) return null;
        var score = (titleHit ? 3 : 0) + (headingHit ? 2 : 0) + 1;
        return { p: p, score: score };
      })
      .filter(Boolean)
      .sort(function (a, b) { return b.score - a.score; })
      .map(function (x) { return x.p; });
    renderResults(scored, query);
  }

  if (searchInput && searchResults) {
    searchInput.addEventListener("focus", loadIndex);
    searchInput.addEventListener("input", function () {
      loadIndex().then(function () { runSearch(searchInput.value.trim()); });
    });
    document.addEventListener("click", function (e) {
      if (!searchResults.contains(e.target) && e.target !== searchInput) {
        searchResults.hidden = true;
      }
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "/" && document.activeElement !== searchInput) {
        e.preventDefault();
        searchInput.focus();
      }
    });
  }
})();
