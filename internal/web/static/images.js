"use strict";

// Your images: the statuses in plain words, the filters and their chips,
// and the rows themselves. The line saying what wants doing is summary.js;
// the panel that opens when a row is clicked is details.js.

// ---- Statuses in plain words ----------------------------------------------

// What the page calls each status, and what it means. The report keeps its
// own words for the command line and for scripts; only the page speaks
// plainly, and every status explains itself when you hover or tap it.
const STATUS_WORDS = {
  "update available": ["Update available", "A newer version is published. isoshelf can download it, verify it, and put it in place."],
  "up to date": ["Up to date", "This is the newest version the project publishes."],
  "EOL": ["End of life", "This release stopped getting security fixes. Fine to keep for a virtual machine, an old PC, or tinkering."],
  "not bootable": ["Won't boot from here", "An image, but not in a format this folder's boot menu can use."],
  "manual": ["Download manually", "This project publishes no checksum, so isoshelf can't verify a download of it. Get this one from its download page, which is one click away."],
  "missing": ["Missing", "You usually keep this image here, but the file isn't in the folder."],
  "unrecognized": ["Unrecognized", "isoshelf doesn't know what this file is. It can try to identify it, or you can name it yourself."],
  "checksum mismatch": ["Checksum mismatch", "The file doesn't match the checksum the project publishes - usually a broken download, sometimes a changed file."],
  "unverified": ["Unverified", "The project publishes no checksum for this file, so nothing can prove it arrived intact."],
  "check failed": ["Check failed", "isoshelf couldn't reach the project this time. The note says what went wrong."],
  "unknown": ["Not identified", "isoshelf hasn't worked out enough about this file to say what it is."],
  "not checked": ["Not checked yet", "Press Check now and isoshelf will ask each project what the newest version is."],
};

function statusWord(status) {
  return (STATUS_WORDS[status] || [status])[0];
}

function statusHelp(status) {
  return (STATUS_WORDS[status] || [])[1] || "";
}

// showMeanings lists every status and what it means, for anyone who wants
// the whole key rather than one explanation at a time.
function showMeanings() {
  const dialog = $("ask");
  $("ask-title").textContent = "What the statuses mean";
  const text = $("ask-text");
  text.replaceChildren(el("dl", { class: "meanings" },
    Object.entries(STATUS_WORDS).flatMap(([status, [word, help]]) => [
      el("dt", {}, el("span", { class: `pill ${STATUS_CLASS[status] || "s-muted"}` }, word)),
      el("dd", {}, help),
    ])));
  const buttons = $("ask-buttons");
  buttons.replaceChildren(el("button", { type: "button", class: "btn primary", onclick: () => dialog.close() }, "Got it"));
  dialog.showModal();
}

// ---- Filters ---------------------------------------------------------------

const KINDS = [
  ["desktop", "Desktop"],
  ["gaming", "Gaming and handhelds"],
  ["server", "Server and homelab"],
  ["boards", "Raspberry Pi and other boards"],
  ["security", "Security and privacy"],
  ["rescue", "Rescue and tools"],
  ["windows", "Windows"],
  ["other", "Other"],
];

const ARCHES = [
  ["x86_64", "64-bit (x86_64)"],
  ["x86", "32-bit (x86)"],
  ["arm64", "ARM 64-bit"],
  ["arm", "ARM 32-bit"],
  ["multi", "Multi"],
];

// What an architecture is called on a badge, and what it is called in full.
//
// The badge used to show the catalog's own word for it, which for 32-bit
// images is "x86" - and "x86" reads to most people as the ordinary kind,
// which is to say 64-bit. The catalog was never wrong (every x86 entry says
// "(32-bit)" in its name); the badge was. So the badge says the bit width and
// the full name waits in the tooltip.
const ARCH_BADGE = {
  "x86_64": "64-bit",
  "x86": "32-bit",
  "arm64": "ARM64",
  "arm": "ARM32",
  "multi": "Multi",
};
const ARCH_FULL = Object.fromEntries(ARCHES);

// archBadge draws one, or nothing when there is no architecture to show.
//
// Ordinary 64-bit PC images get none (v0.7.0): nearly every row said
// "64-bit", which told nobody anything and hid the rows where it matters.
// always is for the few places choosing between the two is the point, like
// naming a file.
function archBadge(arch, always) {
  if (!arch || (arch === "x86_64" && !always)) return null;
  return el("span", { class: "arch", title: ARCH_FULL[arch] || arch },
    ARCH_BADGE[arch] || arch);
}

// pinIcon is the small pushpin a pinned row carries, drawn rather than an
// emoji so it looks the same on every system.
function pinIcon() {
  const ns = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  svg.classList.add("pin-icon");
  for (const d of ["M15 4.5l-4 4l-4 1.5l-1.5 1.5l7 7l1.5-1.5l1.5-4l4-4", "M9 15l-4.5 4.5", "M14.5 4l5.5 5.5"]) {
    const path = document.createElementNS(ns, "path");
    path.setAttribute("d", d);
    svg.append(path);
  }
  return svg;
}

const SHOW = [
  ["updates", "Updates available"],
  ["duplicates", "Possible duplicates"],
  ["favorites", "Favorites"],
  ["older", "Older versions"],
  ["caution", "Has a caution (⚠)"],
];

// renderFilters fills the Filter menu with what this folder actually holds,
// and shows every filter that is on as a chip above the list.
function renderFilters() {
  const box = $("filter-items");
  box.replaceChildren();
  if (!state.report) return;
  const items = state.report.items;
  const has = (pick) => items.some(pick);

  const group = (title, boxes) => boxes.length
    ? el("div", { class: "filter-group" }, el("div", { class: "filter-title" }, title), boxes)
    : null;

  // A group with nothing in it is null, and append would write that as a word.
  box.append(...[
    group("Show", SHOW.filter(([key]) => {
      if (key === "updates") return has((it) => it.status === "update available" && !isDismissed(it));
      if (key === "favorites") return has((it) => it.entry && (state.tracks[it.entry] || {}).starred);
      if (key === "older") return has(isOlder);
      if (key === "duplicates") return has(isDuplicate);
      return has((it) => cautionOf(it));
    }).map(([key, label]) => filterBox(label, view.show[key], (on) => {
      view.show[key] = on;
    }))),
    group("Kind", KINDS.filter(([kind]) => has((it) => (it.category || "other") === kind))
      .map(([kind, label]) => filterBox(label, view.kinds.includes(kind), (on) => {
        view.kinds = on ? [...view.kinds, kind] : view.kinds.filter((k) => k !== kind);
      }))),
    group("Architecture", ARCHES.filter(([arch]) => has((it) => it.arch === arch))
      .map(([arch, label]) => filterBox(label, view.arches.includes(arch), (on) => {
        view.arches = on ? [...view.arches, arch] : view.arches.filter((a) => a !== arch);
      }))),
  ].filter(Boolean));

  const count = activeFilters().length;
  const badge = $("filter-count");
  badge.hidden = count === 0;
  badge.textContent = count;
  $("filter-clear").disabled = count === 0;
  renderChips();
}

function filterBox(label, checked, set) {
  return el("label", { class: "check" },
    el("input", {
      type: "checkbox",
      checked: checked || undefined,
      onchange: (e) => {
        set(e.target.checked);
        saveView();
        renderFilters();
        renderRows();
      },
    }),
    label);
}

// activeFilters lists what is filtering the list right now: each one gets a
// chip, so a short list always says why it is short.
function activeFilters() {
  const chips = [];
  if (view.status) {
    chips.push({ label: statusWord(view.status), off: () => { view.status = ""; } });
  }
  for (const [key, label] of SHOW) {
    if (view.show[key]) chips.push({ label, off: () => { view.show[key] = false; } });
  }
  for (const [kind, label] of KINDS) {
    if (view.kinds.includes(kind)) chips.push({ label, off: () => { view.kinds = view.kinds.filter((k) => k !== kind); } });
  }
  for (const [arch, label] of ARCHES) {
    if (view.arches.includes(arch)) chips.push({ label, off: () => { view.arches = view.arches.filter((a) => a !== arch); } });
  }
  const query = $("search").value.trim();
  if (query) chips.push({ label: `“${query}”`, off: () => { $("search").value = ""; } });
  return chips;
}

function renderChips() {
  const box = $("chips");
  const chips = activeFilters();
  box.hidden = chips.length === 0;
  // Nothing may be handed to replaceChildren in place of a node: unlike el,
  // it doesn't skip a null but writes the word "null" on the page.
  box.replaceChildren(...[
    ...chips.map((chip) => el("button", {
      type: "button", class: "chip-off", title: "Stop filtering by this",
      onclick: () => {
        chip.off();
        saveView();
        renderFilters();
        renderRows();
      },
    }, chip.label, el("span", { class: "x", "aria-hidden": "true" }, "✕"))),
    // Missing images are shown to be got back, so the list offers that.
    view.status === "missing" && downloadableMissing().length
      ? el("button", {
          type: "button", class: "btn small", onclick: downloadMissing,
        }, "Download all…")
      : null,
    // Older versions are shown to be cleared, so the list offers that.
    view.show.older && state.report
      ? el("button", {
          type: "button", class: "btn small", disabled: scanning(),
          onclick: () => reviewOlder(state.report.items.filter(shown)),
        }, "Review and clear…")
      : null,
    chips.length > 1
      ? el("button", { type: "button", class: "linkish", onclick: clearFilters }, "Clear all")
      : null,
  ].filter(Boolean));
}

function clearFilters() {
  resetFilters();
  saveView();
  renderFilters();
  renderRows();
}

function resetFilters() {
  view.status = "";
  view.kinds = [];
  view.arches = [];
  for (const [key] of SHOW) view.show[key] = false;
  $("search").value = "";
}

// shown says whether the search and the filters leave this image in the list.
function shown(item) {
  const query = $("search").value.trim().toLowerCase();
  const haystack = `${item.name} ${item.path || ""} ${item.entry || ""} ${item.family || ""}`.toLowerCase();
  const starred = item.entry && (state.tracks[item.entry] || {}).starred;
  return (!view.status || item.status === view.status) &&
    (!query || haystack.includes(query)) &&
    (!view.kinds.length || view.kinds.includes(item.category || "other")) &&
    (!view.arches.length || view.arches.includes(item.arch)) &&
    (!view.show.updates || (item.status === "update available" && !isDismissed(item))) &&
    (!view.show.favorites || starred) &&
    (!view.show.older || isOlder(item)) &&
    (!view.show.duplicates || isDuplicate(item)) &&
    (!view.show.caution || cautionOf(item));
}

// isOlder is a file you could clear because a newer version of the same
// image is here too. A pinned one isn't: somebody decided to keep it.
function isOlder(item) {
  return Boolean(item.older && item.path && !isPinned(item));
}

function renderRows() {
  const rows = $("rows");
  rows.replaceChildren();
  const empty = $("empty");
  if (!state.report) {
    empty.hidden = false;
    empty.textContent = state.target
      ? "Scanning will list the images in this folder."
      : "Choose a folder to see its images.";
    $("shown").textContent = "";
    return;
  }

  const items = sortItems(state.report.items.filter(shown));

  for (const item of items) rows.append(renderRow(item));
  // Like the key on a menu: only there when something in the list has the mark.
  $("legend").hidden = !items.some((item) => cautionOf(item));

  const total = state.report.items.length;
  $("shown").textContent = items.length === total ? plural(total, "image") : `${items.length} of ${plural(total, "image")}`;

  empty.hidden = items.length > 0;
  empty.replaceChildren();
  if (items.length === 0) {
    if (total === 0) {
      empty.append("No images found in this folder.");
    } else {
      empty.append(
        `None of the ${plural(total, "image")} here match ${activeFilters().map((c) => c.label).join(", ")}.`,
        el("div", {},
          el("button", { type: "button", class: "btn small", onclick: clearFilters }, "Clear filters")));
    }
  }
}

const STATUS_ORDER = STATUSES.map(([status]) => status);

// compareVersions reads versions the way people do: 22.10 is newer than 22.4.
function compareVersions(a, b) {
  const partsOf = (v) => (v || "").split(/[^a-zA-Z0-9]+/).flatMap((part) => part.match(/\d+|[a-zA-Z]+/g) || []);
  const left = partsOf(a), right = partsOf(b);
  for (let i = 0; i < Math.max(left.length, right.length); i++) {
    const x = left[i], y = right[i];
    if (x === undefined) return -1;
    if (y === undefined) return 1;
    const bothNumbers = /^\d+$/.test(x) && /^\d+$/.test(y);
    if (bothNumbers && Number(x) !== Number(y)) return Number(x) - Number(y);
    if (!bothNumbers && x !== y) return x < y ? -1 : 1;
  }
  return 0;
}

// Each sorter puts the most useful end first, so "descending" means the
// reverse of what the column's name suggests: largest, newest, most urgent.
function sorters() {
  const byName = (a, b) => a.name.localeCompare(b.name) || (a.path || "").localeCompare(b.path || "");
  const starred = (item) => (item.entry && (state.tracks[item.entry] || {}).starred ? 0 : 1);
  // A dismissed update sorts with the ones that are up to date (#56).
  const rank = (item) => STATUS_ORDER.indexOf(isDismissed(item) ? "up to date" : item.status);
  const text = (value) => (value || "").toLowerCase();
  return {
    attention: (a, b) => rank(a) - rank(b) || byName(a, b),
    favorites: (a, b) => starred(a) - starred(b) || rank(a) - rank(b) || byName(a, b),
    name: byName,
    size: (a, b) => (b.size || 0) - (a.size || 0) || byName(a, b),
    version: (a, b) => compareVersions(b.version, a.version) || byName(a, b),
    latest: (a, b) => compareVersions(b.latest, a.latest) || byName(a, b),
    file: (a, b) => text(a.path).localeCompare(text(b.path)) || byName(a, b),
    added: (a, b) => new Date(b.added || 0) - new Date(a.added || 0) || byName(a, b),
  };
}

// blank says whether a row has nothing to compare in this column: an image
// that isn't in the folder has no file, size or version. Those always sort
// last, whichever way round the column is, because a list that starts with
// blanks is a list you have to scroll past.
const BLANK = {
  size: (item) => !item.path || !item.size,
  version: (item) => !item.version,
  latest: (item) => !item.latest,
  file: (item) => !item.path,
  added: (item) => !item.added,
};

function sortItems(items) {
  const all = sorters();
  const compare = all[view.sort] || all.attention;
  const isBlank = BLANK[view.sort] || (() => false);

  const filled = [], blanks = [];
  for (const item of items) (isBlank(item) ? blanks : filled).push(item);
  filled.sort(compare);
  if (view.desc) filled.reverse();
  blanks.sort(all.name);
  return [...filled, ...blanks];
}

// Columns that can be sorted by clicking their heading, and what each one
// means the first time it's clicked.
const COLUMN_SORTS = [
  ["col-image", "name", "By name, A to Z"],
  ["col-version", "version", "Newest version here first"],
  ["col-status", "attention", "Most urgent first"],
  ["col-added", "added", "Most recently added first"],
];

// renderHeadings makes the column titles sort the list, and shows which one
// is doing it.
function renderHeadings() {
  for (const [id, key, hint] of COLUMN_SORTS) {
    const heading = $(id);
    if (!heading) continue;
    const active = view.sort === key;
    heading.classList.toggle("sorted", active);
    heading.setAttribute("aria-sort", active ? (view.desc ? "descending" : "ascending") : "none");
    heading.title = active
      ? `Sorted by this column, ${view.desc ? "descending" : "ascending"}. Click to reverse it.`
      : hint;
    heading.onclick = () => {
      // The same column again turns it round; a different one starts fresh.
      view.desc = active ? !view.desc : false;
      view.sort = key;
      $("sort").value = key;
      saveView();
      renderHeadings();
      renderRows();
    };
  }
}

// logoTile is the project logo, or colored initials when there is none.
function logoTile(item) {
  const tile = el("span", { class: "logo", "aria-hidden": "true" });
  if (item.icon) {
    tile.classList.add("has-logo");
    tile.style.setProperty("--logo", `url("/logo/${encodeURIComponent(item.icon)}.svg")`);
    tile.style.setProperty("--brand", readableBrand(item.icon_color));
  } else {
    tile.classList.add("initials");
    tile.textContent = initials(item.name);
    tile.style.setProperty("--brand", colorFor(item.name));
  }
  return tile;
}

function initials(name) {
  const words = name.replace(/[^A-Za-z0-9 ]/g, " ").split(/\s+/).filter(Boolean);
  return ((words[0] || "?")[0] + (words[1] ? words[1][0] : "")).toUpperCase();
}

function colorFor(name) {
  let hash = 0;
  for (const ch of name) hash = (hash * 31 + ch.charCodeAt(0)) % 360;
  return `hsl(${hash} 45% 42%)`;
}

// readableBrand keeps brand colors visible: a few are nearly black, which
// disappears on a dark background.
function readableBrand(color) {
  if (!color) return "currentColor";
  const value = parseInt(color.slice(1), 16);
  const [r, g, b] = [(value >> 16) & 255, (value >> 8) & 255, value & 255];
  const luminance = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
  const dark = darkNow();
  if (dark && luminance < 0.25) return "#c9d2df";
  if (!dark && luminance > 0.9) return "#5f6878";
  return color;
}

// problemLink reports an image whose source has moved or broken. Projects
// rearrange their download folders without warning, and the person who hits
// it is the one who can say what happened.
function problemLink(item) {
  if (!state.report_url || !item.entry) return null;
  const title = `Catalog: ${item.name} ${item.status === "check failed" ? "can't be checked" : "needs fixing"}`;
  const body = [
    `**Image:** ${item.name} (\`${item.entry}\`)`,
    `**isoshelf:** ${state.version}`,
    item.version ? `**Version here:** ${item.version}` : null,
    item.latest ? `**Newest isoshelf found:** ${item.latest}` : null,
    `**Status:** ${item.status}`,
    item.note ? `**What it said:** ${item.note}` : null,
    "",
    "What is wrong? For a download that moved, the new address helps most — especially",
    "where the project publishes its checksums now.",
  ].filter((line) => line !== null).join("\n");
  return ["Report a problem", `${state.report_url}?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}&labels=catalog`];
}

// linksMenu holds the project pages, out of the way until asked for.
function linksMenu(item) {
  const links = [
    ["Download page", item.page],
    ["Website", item.site],
    ["Forum", item.forum],
    ["Release notes", item.release],
    problemLink(item),
  ].filter((link) => link && link[1]);
  if (!links.length) return null;

  const menu = el("details", { class: "menu" },
    el("summary", { title: "Links", "aria-label": `Links for ${item.name}` }, "\u2026"),
    el("div", { class: "menu-items" },
      links.map(([label, url]) => el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, label))));
  wireMenu(menu);
  return menu;
}

// wireMenu makes a menu behave: only one open at a time, and pinned to the
// window where there is room for it. Every details.menu needs this, the
// Filter menu included - it was left out, so on a full drive it hung off the
// bottom of the window with the last filters out of reach.
function wireMenu(menu) {
  menu.addEventListener("toggle", () => {
    if (!menu.open) return;
    for (const other of document.querySelectorAll("details.menu[open]")) {
      if (other !== menu) other.open = false;
    }
    placeMenu(menu);
  });
}

// placeMenu pins an open menu to the window, so the table's scroll box can't
// cut it off, and opens it upwards when there's no room below.
function placeMenu(menu) {
  const items = menu.querySelector(".menu-items");
  const anchor = menu.querySelector("summary").getBoundingClientRect();
  const floor = innerHeight - ($("dock").hidden ? 8 : $("dock").offsetHeight + 8);
  items.classList.add("pinned");
  items.style.right = `${Math.max(8, innerWidth - anchor.right)}px`;
  // On a phone a menu with a lot of filters in it is wider than the window.
  // Only the right edge was ever held inside; the left one went off the side
  // of the screen with the first filters out of reach.
  items.style.maxWidth = `${innerWidth - 16}px`;
  // Measure without the limits a previous opening may have left behind.
  items.style.maxHeight = "";
  const wanted = items.offsetHeight;
  const spread = items.getBoundingClientRect();
  if (spread.left < 8) {
    items.style.right = `${Math.max(8, innerWidth - spread.width - 8)}px`;
  }
  const below = anchor.bottom + 4;
  const roomBelow = floor - below;
  const roomAbove = anchor.top - 12;
  if (wanted <= roomBelow) {
    items.style.top = `${below}px`;
  } else if (wanted <= roomAbove) {
    items.style.top = `${anchor.top - wanted - 4}px`;
  } else {
    // A long list of filters on a short window fits neither way. Take the
    // roomier side and let the menu scroll, so the last filter is still
    // reachable instead of hanging off the screen.
    const under = roomBelow >= roomAbove;
    items.style.top = under ? `${below}px` : "8px";
    items.style.maxHeight = `${Math.max(140, under ? roomBelow : roomAbove)}px`;
  }
}

function closeMenus() {
  for (const open of document.querySelectorAll("details.menu[open]")) open.open = false;
}

function renderRow(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  const usual = item.entry && state.usual_set.includes(item.entry);

  // A filled star means the user starred the image. Images that are only in
  // the usual set because recent scans saw them keep an outline star.
  const star = el("button", {
    type: "button",
    class: "star",
    "aria-pressed": track.starred ? "true" : "false",
    title: track.starred
      ? "Starred: isoshelf reports this image as missing if it disappears from this folder. Click to unstar."
      : usual
        ? "Usually kept here (seen in recent scans). Star it to always report it if it goes missing."
        : "Star to always report this image if it goes missing from this folder",
    "aria-label": `Star ${item.name}`,
    disabled: !item.entry || scanning(),
    onclick: () => setTrack(item.entry, { starred: !track.starred }),
  }, track.starred ? "★" : "☆");

  const statusCell = el("td", {},
    isDismissed(item)
      ? el("span", { class: "pill s-muted", title: "You dismissed this update. Its details panel can bring it back." }, dismissedWords(item))
      : el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}`, title: statusHelp(item.status) },
        statusWord(item.status)),
    item.eol && item.status !== "EOL"
      ? el("span", { class: "pill s-eol", title: statusHelp("EOL") }, "End of life")
      : null);

  const name = item.page
    ? el("a", { href: item.page, target: "_blank", rel: "noopener noreferrer", title: "Open the download page" }, item.name)
    : item.name;
  const caution = cautionOf(item);
  const imageCell = el("td", {}, el("div", { class: "image-cell" },
    logoTile(item),
    el("div", { class: "image-text" },
      // The name may wrap; the badges sit on their own line underneath so a
      // wrapped name never leaves one dangling at the end of it.
      el("div", { class: "name-line" },
        el("span", { class: "name" }, name),
        caution ? el("span", { class: "caution", title: caution, "aria-label": `Worth knowing: ${caution}` }, "⚠") : null,
        isPinned(item) ? el("span", { class: "pin-mark", title: "Pinned: kept whatever updates come", role: "img", "aria-label": "Pinned" }, pinIcon()) : null),
      archBadge(item.arch) ? el("div", { class: "meta-line" }, archBadge(item.arch)) : null,
      item.note ? el("div", { class: "note" }, item.note) : null,
      duplicateNote(item))));

  // The file, its size and when it arrived go under the name: one line each
  // instead of four columns.
  const under = [];
  let added = null;
  if (item.path) {
    under.push(el("span", { class: "file", title: item.path }, breakable(item.path)));
    if (item.size) under.push(el("span", {}, formatBytes(item.size)));
    // On a phone there are no columns, so the date goes here instead; the
    // stylesheet shows whichever of the two fits the width. Saying it twice
    // at once would be worse than not saying it at all. The "·" before it is
    // part of it, so where it is hidden no "·" is left hanging at the end.
    if (item.added) {
      added = el("span", { class: "added-inline", title: new Date(item.added).toLocaleString() },
        `· ${item.placed ? "Updated" : "Added"} ${shortDate(item.added)}`);
    }
  } else {
    under.push(el("span", { class: "muted" }, "Not in this folder"));
  }
  imageCell.querySelector(".image-text").append(el("div", { class: "under" }, join(under, " · "), added));

  // "22.04 → 24.04" says more in one column than two ever did.
  const newer = item.latest && item.status === "update available";
  const versionCell = el("td", { class: "version-cell" },
    item.version || (item.latest ? "" : "–"),
    newer ? el("span", { class: "latest-new", title: item.latest_file || "" }, `${item.version ? " → " : ""}${item.latest}`) : null);

  // When the file arrived, or when an update last replaced it. An image
  // that isn't in the folder has no date and sorts last, like every other
  // blank column.
  const addedCell = el("td", { class: "col-added added-cell" },
    item.added
      ? el("span", { title: `${item.placed ? "Updated" : "Added"} ${new Date(item.added).toLocaleString()}` },
        shortDate(item.added))
      : el("span", { class: "muted" }, "–"));

  const actions = [];
  if (item.entry && item.updates === "download" && item.status === "update available" && !isDismissed(item)) {
    actions.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn small primary",
      title: `Download ${item.latest || "the newest version"} and put it in this folder`,
      onclick: () => updateItem(item),
    }, "Update"), true));
  } else if (item.status === "missing" && item.entry) {
    const back = missingAction(item, true);
    if (back) actions.push(back);
  } else if (item.path && !item.entry) {
    actions.push(el("button", {
      type: "button", class: "btn small primary", disabled: scanning(),
      title: "Let isoshelf identify this file",
      "aria-label": `Identify ${item.path}`,
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  actions.push(...duplicateActions(item));
  actions.push(el("button", {
    type: "button", class: "btn small", "aria-label": `Everything about ${item.name}`,
    title: "Links, settings and everything else about this image",
    onclick: () => openDetails(item),
  }, "Details"));

  // The whole row opens the details panel; the buttons in it don't.
  const row = el("tr", {
    class: `row${detailsKey(item) === detailsOpen ? " picked" : ""}${isDismissed(item) ? " dismissed" : ""}`,
    onclick: (e) => {
      if (e.target.closest("button, a, input, label, summary")) return;
      openDetails(item);
    },
  },
    el("td", { class: "col-star" }, star),
    imageCell,
    versionCell,
    statusCell,
    addedCell,
    el("td", { class: "row-actions" }, actions));
  return row;
}

// join puts a separator between parts, the way a sentence would.
function join(parts, separator) {
  return parts.flatMap((part, i) => (i ? [separator, part] : [part]));
}

// breakable lets a long filename wrap after its separators — the _ - and .
// between words — instead of in the middle of one.
function breakable(text) {
  const parts = [];
  for (const piece of text.split(/(?<=[_\-.])/)) parts.push(piece, el("wbr"));
  parts.pop();
  return parts;
}

// cautionOf says what's worth knowing before using an image, if anything:
// that it no longer gets security fixes, or a note from the catalog. It's a
// mark to hover over, never a block — people keep old images on purpose.
function cautionOf(item) {
  if (item.caution) return item.caution;
  if (item.eol) return "End of life: this release no longer gets security fixes.";
  return "";
}

// shortDate reads like a person: "today", "3 days ago", then "12 Mar 2025".
function shortDate(iso) {
  const then = new Date(iso);
  const days = Math.floor((Date.now() - then.getTime()) / 86400000);
  if (days < 1) return "today";
  if (days === 1) return "yesterday";
  if (days < 7) return `${days} days ago`;
  return then.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}
