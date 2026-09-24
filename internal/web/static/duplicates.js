"use strict";

// Duplicate copies (v0.8.4, #57): the same image and version in the folder
// twice. The cheap look is the image, version and size; Make sure reads the
// files and compares their hashes, only when pressed (the maintainer: "A lot
// of times I just trust it"). Removing a copy goes through the archive, like
// every removal.

let dupCache = { report: null, groups: [] };

// duplicateGroups is every set of files that look like one another's
// copies. Files that have all been read are grouped by their hashes, so two
// that turn out to differ drop out; otherwise the group is only possible.
function duplicateGroups() {
  if (!state.report) return [];
  if (dupCache.report === state.report) return dupCache.groups;
  const byLook = new Map();
  for (const it of state.report.items) {
    if (!it.path || !it.entry || !it.version || !it.size) continue;
    const key = `${it.entry}\u0000${it.version}\u0000${it.size}`;
    if (!byLook.has(key)) byLook.set(key, []);
    byLook.get(key).push(it);
  }
  const groups = [];
  for (const look of byLook.values()) {
    if (look.length < 2) continue;
    if (look.every((it) => it.sha256)) {
      const byHash = new Map();
      for (const it of look) byHash.set(it.sha256, [...(byHash.get(it.sha256) || []), it]);
      for (const same of byHash.values()) if (same.length > 1) groups.push({ items: same, sure: true });
    } else {
      groups.push({ items: look, sure: false });
    }
  }
  dupCache = { report: state.report, groups };
  return groups;
}

function duplicateGroupOf(item) {
  return duplicateGroups().find((g) => g.items.includes(item));
}

function isDuplicate(item) {
  return Boolean(duplicateGroupOf(item));
}

// duplicateNote is the line under a copy's name.
function duplicateNote(item) {
  const group = duplicateGroupOf(item);
  if (!group) return null;
  const others = group.items.filter((it) => it !== item).map((it) => it.path).join(", ");
  return el("div", { class: "note" }, group.sure
    ? `The same file as ${others}, byte for byte.`
    : `Same image, version and size as ${others}. Make sure reads both to tell.`);
}

// duplicateActions are the row's buttons while the list shows duplicates.
function duplicateActions(item) {
  const group = duplicateGroupOf(item);
  if (!group || !view.show.duplicates) return [];
  const out = [];
  if (!group.sure) {
    out.push(el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      title: "Read these files and compare them. Minutes for big images on a slow drive.",
      onclick: () => makeSure(group),
    }, "Make sure"));
  }
  out.push(el("button", {
    type: "button", class: "btn small", disabled: scanning(),
    onclick: () => removeItem(item),
  }, "Remove this copy…"));
  return out;
}

let makingSure = null;

async function makeSure(group) {
  const paths = group.items.map((it) => it.path);
  try {
    await api("POST", "/api/hash", { paths });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  makingSure = paths;
  await refresh();
}

// sayWhetherTheyMatched is called on every draw; once the reading is done it
// says what it found, once.
function sayWhetherTheyMatched() {
  if (!makingSure || scanning() || !state.report) return;
  const items = makingSure.map((p) => state.report.items.find((it) => it.path === p)).filter(Boolean);
  makingSure = null;
  const hashes = new Set(items.map((it) => it.sha256).filter(Boolean));
  if (hashes.size === 1 && items.every((it) => it.sha256)) {
    flashNotice(`${items.map((it) => it.path).join(" and ")} are the same file. Remove the copy you don't need.`);
  } else if (hashes.size > 1) {
    flashNotice("They're different files after all, so neither is a copy of the other.");
  }
}
