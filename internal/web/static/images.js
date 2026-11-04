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
  return el("span", { class: "label", title: ARCH_FULL[arch] || arch },
    ARCH_BADGE[arch] || arch);
}

const SHOW = [
  ["updates", "Updates available"],
  ["duplicates", "Possible duplicates"],
  ["favorites", "Favorites"],
  ["older", "Older versions"],
  ["caution", "Has a caution"],
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
  setAvailable($("filter-clear"), count > 0, "No filters are on.");
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
    }, chip.label, el("span", { class: "x" }, icon("x-16")))),
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
    empty.className = "empty";
    empty.replaceChildren(state.target
      ? blankslate("Nothing scanned yet", "Refresh reads the folder and lists the images in it.",
        el("button", { type: "button", class: "btn primary", onclick: () => start("check") }, "Refresh"))
      : blankslate("Choose a folder", "isoshelf lists, checks and updates the images in the folder you choose.",
        el("button", { type: "button", class: "btn primary", onclick: openPicker }, "Choose folder")));
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
      empty.append(blankslate("No images in this folder", "Add some from the catalog, or drop a file onto this page.",
        el("button", { type: "button", class: "btn primary", onclick: () => jumpTo("more") }, "Add images")));
    } else {
      empty.append(blankslate("No images match",
        `None of the ${plural(total, "image")} here match ${activeFilters().map((c) => c.label).join(", ")}.`,
        el("button", { type: "button", class: "btn primary", onclick: clearFilters }, "Clear filters")));
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


