"use strict";

// The list of images isoshelf knows: the Add images tab, and the line at the
// foot of the page saying where the list came from and whether it keeps
// itself up to date. Split out of actions.js in v0.7.0.

// ---- Where the list comes from ----------------------------------------------

const CATALOG_SOURCE = {
  "built-in": "the list isoshelf was built with",
  "downloaded": "kept up to date from the project",
  "yours": "your own catalog file",
};

// renderCatalogStatus shows where the list of known images comes from, and
// lets the user decide whether isoshelf keeps it current by itself.
function renderCatalogStatus(footer) {
  const cat = state.catalog;
  if (!cat || !cat.entries) return;
  const where = CATALOG_SOURCE[cat.source] || cat.source;
  const when = cat.source === "downloaded" && cat.updated_at ? `, checked ${timeAgo(cat.updated_at)}` : "";
  const mine = cat.mine ? `, plus ${plural(cat.mine, "image")} you named yourself` : "";
  const row = el("div", { class: "footer-row" },
    el("span", {}, `${plural(cat.entries, "image")} known: ${where}${when}${mine}.`),
    // The list changes separately from isoshelf, so it says when it last did
    // and where the changes are written down.
    cat.changed ? el("span", { class: "muted" },
      `List last changed ${new Date(cat.changed + "T12:00:00").toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" })} · `,
      el("a", { href: cat.changes_url, target: "_blank", rel: "noopener noreferrer" }, "What's new")) : null);

  if (cat.can_auto) {
    row.append(
      el("label", { class: "check", title: "New images arrive without a new isoshelf. Only the project's own repository is ever fetched, and a list that doesn't pass every check is refused." },
        el("input", {
          type: "checkbox", checked: cat.auto,
          onchange: (e) => setCatalogAuto(e.target.checked),
        }), " Keep this list up to date"),
      el("button", {
        type: "button", class: "btn small",
        title: "Ask the project for a newer list right now",
        onclick: refreshCatalog,
      }, "Check now"));
  } else if (cat.source === "yours") {
    row.append(el("span", { class: "muted" }, "isoshelf never changes a catalog you wrote."));
  }
  if (cat.note) row.append(el("span", { class: "muted" }, cat.note));
  if (cat.error) row.append(el("span", { class: "muted" }, cat.error));
  footer.append(row);
}

async function setCatalogAuto(on) {
  try {
    state = await api("POST", "/api/settings", { catalog_auto: on });
    catalog = null;
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

async function refreshCatalog() {
  try {
    const result = await api("POST", "/api/catalog/refresh");
    if (result.message) showNotice(result.message);
  } catch (err) {
    showNotice(err.message, true);
  }
  catalog = null;
  await refresh();
}

function renderFooter() {
  const footer = $("footer");
  footer.replaceChildren();
  renderCatalogStatus(footer);
  if (state.removed && state.removed.files > 0) {
    footer.append(el("div", { class: "footer-row" },
      `Archived files in .isoshelf/removed: ${plural(state.removed.files, "file")} using ${formatBytes(state.removed.bytes)}.`,
      el("button", { type: "button", class: "btn small", disabled: scanning() || downloading(), onclick: emptyRemoved }, "Empty it")));
  }
  if (!state.report) return;
  for (const trash of state.report.trash) {
    footer.append(el("div", {}, `Trash folder ${trash.path} uses ${formatBytes(trash.bytes)}. Emptying it frees that space.`));
  }
  for (const problem of state.report.problems) {
    footer.append(el("div", {}, `Couldn't read ${problem.path}: ${problem.error}`));
  }
}

async function renderCatalog() {
  if (!state.report) {
    $("more-count").textContent = "";
    $("catalog").replaceChildren();
    return;
  }
  if (!catalog) {
    try {
      catalog = (await api("GET", "/api/catalog")).entries;
    } catch {
      return;
    }
    // The download sizes above the list come from the catalog, and they
    // were drawn before it arrived.
    renderFirstRun();
    renderTodo();
  }
  const missing = catalog.filter((e) => !e.on_target);
  $("more-count").textContent = plural(missing.length, "image");
  // Images added from here stay listed, ticked, until the downloads are
  // cleared, so the button that was clicked says how it went.
  const listed = catalog.filter((e) => !e.on_target || (jobFor(e.id) || {}).where === "done");

  const shown = filterCatalog(listed);
  const list = $("catalog");
  list.replaceChildren();
  for (const entry of shown) {
    const room = jobFor(entry.id) ? null : fitsHere(entry);
    list.append(el("li", {},
      logoTile(entry),
      el("div", { class: "info" },
        el("div", { class: "info-line" }, el("span", { class: "name" }, entry.name)),
        el("div", { class: "meta-line" },
          archBadge(entry.arch),
          entry.popular ? el("span", { class: "pill s-ok", title: "Turns up in public round-ups of what people are running. A hand-picked hint, not a rating." }, "popular") : null,
          entry.size ? el("span", { class: "muted" }, `about ${formatBytes(entry.size)}`) : null),
        el("div", { class: "kind" },
          UPDATES_LABEL[entry.updates] || entry.updates,
          room === false
            ? el("span", { class: "wont-fit" }, fitsHere(entry, true) ? " · no space once the queued downloads are in" : " · bigger than the space left here")
            : null)),
      entry.page ? el("a", { class: "btn small", href: entry.page, target: "_blank", rel: "noopener noreferrer" }, "Page") : null,
      addButton(entry, room)));
  }
  $("more-shown").textContent = shown.length === listed.length
    ? ""
    : `showing ${shown.length} of ${listed.length}`;

  // Whatever isoshelf knows, someone's favorite image won't be in it.
  const request = $("catalog-request");
  request.replaceChildren();
  if (state.report_url) {
    request.append(
      "Not in the catalog? ",
      el("a", {
        href: `${state.report_url}?template=missing-image.yml`,
        target: "_blank", rel: "noopener noreferrer",
      }, "Request it"),
      " — the form asks where the project publishes its checksums, which is what decides whether isoshelf can download it or only link to it.");
  }
  if (!shown.length) {
    list.append(el("li", { class: "muted more-hint" }, missing.length
      ? "None of these match the filters."
      : "Everything isoshelf knows about is already in this folder."));
  }
}

// fitsHere reports whether an image would fit in the folder once the
// downloads already waiting are in: true, false, or null when either the size
// or the free space is unknown. Without the queue, it asks about the room
// there is now.
function fitsHere(entry, withoutQueue) {
  if (!entry.size || !state.space || !state.space.total) return null;
  const spare = Math.min(state.space.total / 100, 1 << 30);
  const queued = withoutQueue ? 0 : pendingBytes();
  return state.space.free - queued - entry.size >= spare;
}

// filterCatalog applies the catalog list's own filters and sort. They are
// separate from the main list's, because the two lists are read for different
// reasons: what have I got, and what could I add.
function filterCatalog(entries) {
  const query = $("more-search").value.trim().toLowerCase();
  const category = $("more-category").value;
  const arch = $("more-arch").value;
  const updates = $("more-updates").value;
  const fitsOnly = $("more-fits").checked;
  const popularOnly = $("more-popular").checked;

  const shown = entries.filter((entry) => {
    if (query && !`${entry.name} ${entry.id} ${entry.family || ""}`.toLowerCase().includes(query)) return false;
    if (category && (entry.category || "other") !== category) return false;
    if (arch && entry.arch !== arch) return false;
    if (updates && entry.updates !== updates) return false;
    if (fitsOnly && fitsHere(entry) === false) return false;
    if (popularOnly && !entry.popular) return false;
    return true;
  });

  const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  switch ($("more-sort").value) {
    case "size":
      shown.sort((a, b) => (b.size || 0) - (a.size || 0) || byName(a, b));
      break;
    case "smallest":
      // Images with no size go last either way: an unknown size is not small.
      shown.sort((a, b) => (a.size || Infinity) - (b.size || Infinity) || byName(a, b));
      break;
    case "kind":
      shown.sort((a, b) => (a.category || "other").localeCompare(b.category || "other") || byName(a, b));
      break;
    case "popular":
      shown.sort((a, b) => Number(Boolean(b.popular)) - Number(Boolean(a.popular)) || byName(a, b));
      break;
    default:
      shown.sort(byName);
  }
  return shown;
}

// addButton downloads a catalog image this folder doesn't have yet. isoshelf
// can only do that for images whose checksums it can reach; for the rest the
// download page is the way.
function addButton(entry, room) {
  if (entry.updates === "download" && room === false) {
    const queued = pendingBytes();
    return jobButton(entry.id, el("button", {
      type: "button", class: "btn small", disabled: true,
      title: `${entry.name} is about ${formatBytes(entry.size)}, and this folder has ${formatBytes(state.space.free)} left${queued ? `, with about ${formatBytes(queued)} of downloads still to come` : ""}.`,
    }, "Add"), false);
  }
  if (entry.updates !== "download") {
    return el("button", {
      type: "button", class: "btn small", disabled: true,
      title: entry.page
        ? `isoshelf can't verify a download of ${entry.name}: this project publishes no checksum. Use its download page.`
        : `isoshelf can't verify a download of ${entry.name}: this project publishes no checksum.`,
    }, "Add");
  }
  return jobButton(entry.id, el("button", {
    type: "button", class: "btn small primary",
    title: `Download the newest ${entry.name} into this folder. If something is downloading already, this one queues behind it.`,
    onclick: () => queueDownload(entry.id, "keep"),
  }, "Add"), false);
}
