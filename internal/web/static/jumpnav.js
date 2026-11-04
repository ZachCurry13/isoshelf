"use strict";

// The tab bar under the page header (Primer's UnderlineNav): a link to each
// part of the page with the number of things in it, and the one you are
// looking at marked as the current one. They are links to places on this one
// page, so the current one is marked aria-current="location".

// markTab re-marks the current tab; it is set once the page has loaded.
let markTab = null;

// setTab writes one tab: its words, and its count when there is one.
function setTab(id, label, count) {
  const parts = [el("span", { class: "tab-label", "data-label": label }, label)];
  if (count) parts.push(el("span", { class: "counter" }, count.toLocaleString()));
  $(id).replaceChildren(...parts);
}

// followScroll marks the tab of the part of the page nearest the top.
function followScroll() {
  const tabs = [...$("jump").querySelectorAll("a")];
  const parts = tabs.map((tab) => document.getElementById(tab.getAttribute("href").slice(1)));
  function mark() {
    const edge = $("jump").offsetHeight + 8;
    let current = null;
    parts.forEach((part, i) => {
      if (part && !part.hidden && part.getBoundingClientRect().top <= edge + 1) current = tabs[i];
    });
    // At the very end the last part may be too short to reach the top.
    if (window.innerHeight + window.scrollY >= document.body.scrollHeight - 2) {
      const last = tabs.filter((tab, i) => parts[i] && !parts[i].hidden && !tab.hidden).pop();
      if (last) current = last;
    }
    current = current || tabs[0];
    for (const tab of tabs) {
      if (tab === current) tab.setAttribute("aria-current", "location");
      else tab.removeAttribute("aria-current");
    }
  }
  window.addEventListener("scroll", mark, { passive: true });
  window.addEventListener("resize", mark);
  mark();
  return mark;
}

// folderTitle splits a path into where it is and what it is called: the last
// part is the name, and the rest (with its separator) is the context.
function folderTitle(path) {
  if (!path) return ["", "No folder chosen yet"];
  const trimmed = path.replace(/[\/]+$/, "");
  const cut = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  if (!trimmed || cut < 0 || cut === trimmed.length - 1) return ["", path];
  return [trimmed.slice(0, cut + 1), trimmed.slice(cut + 1)];
}
