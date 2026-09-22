"use strict";

// Choosing the folder isoshelf watches. Browsers can't see the computer's
// folders, so the chooser asks isoshelf what is in each one.

// ---- Folder picker ---------------------------------------------------------

let pickerPath = "";

async function openPicker() {
  const dialog = $("picker");
  if (!dialog.open) dialog.showModal();
  await browse(state && state.target ? state.target : "");
}

async function browse(path) {
  let result;
  try {
    result = await api("GET", `/api/browse?path=${encodeURIComponent(path)}`);
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  pickerPath = result.path || "";
  $("picker-input").value = pickerPath;
  $("picker-up").disabled = !result.parent;
  $("picker-up").onclick = () => browse(result.parent);
  $("picker-use").disabled = !pickerPath || Boolean(result.error);
  if (result.suggested_profile) $("picker-profile").value = result.suggested_profile;
  showPickerError(result.error);

  // How many images a folder holds says more than its path does, and on
  // Windows a drive's name is the only way to tell E: from F:.
  const describe = (folder) => {
    const bits = [folder.label, folder.images > 0 ? plural(folder.images, "image") : null].filter(Boolean);
    return bits.length ? el("span", { class: "muted" }, ` — ${bits.join(", ")}`) : null;
  };
  $("picker-here").textContent = result.images > 0
    ? `${plural(result.images, "image")} in this folder`
    : result.path && result.images === 0 ? "No images directly in this folder" : "";

  const roots = $("picker-roots");
  roots.replaceChildren();
  const bookmarks = (state && state.bookmarks) || [];
  if (bookmarks.length) {
    roots.append(el("h3", {}, "Bookmarks"));
    for (const target of bookmarks) {
      roots.append(el("button", { type: "button", title: target, onclick: () => browse(target) }, target));
    }
  }
  const remembered = (state && state.recent_targets) || [];
  if (remembered.length) {
    roots.append(el("h3", {}, "Folders isoshelf remembers"));
    for (const folder of remembered) {
      roots.append(rememberedRow(folder, bookmarks.includes(folder.path)));
    }
  }
  roots.append(el("h3", {}, "Places"));
  for (const root of result.roots) {
    roots.append(el("button", { type: "button", title: root.path, onclick: () => browse(root.path) },
      root.name, describe(root)));
  }

  // The pin follows whichever folder is open.
  const pinned = bookmarks.some((b) => b.toLowerCase() === pickerPath.toLowerCase());
  const pin = $("picker-pin");
  pin.disabled = !pickerPath;
  pin.textContent = pinned ? "★ Bookmarked" : "☆ Bookmark";
  pin.title = pinned
    ? "Forget this folder"
    : "Keep this folder at the top of the list, so it doesn't have to be found again";
  pin.onclick = () => toggleBookmark(pickerPath, pinned);

  const folders = $("picker-folders");
  folders.replaceChildren();
  if (!pickerPath) {
    folders.append(el("li", { class: "picker-empty" }, "Pick a drive or place on the left, or type a path above."));
    return;
  }
  if (!result.error && result.folders.length === 0) {
    folders.append(el("li", { class: "picker-empty" }, "No subfolders. Use this folder, or go up."));
  }
  for (const folder of result.folders) {
    folders.append(el("li", {}, el("button", { type: "button", title: folder.path, onclick: () => browse(folder.path) }, folder.name)));
  }
}

function showPickerError(message) {
  $("picker-error").hidden = !message;
  $("picker-error").textContent = message || "";
}

async function useFolder() {
  const path = $("picker-input").value.trim();
  try {
    await api("POST", "/api/target", { path, profile: $("picker-profile").value });
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  $("picker").close();
  catalog = null;
  statusFilter = null;
  autoScanned = true;
  await start("scan");
}
// toggleBookmark pins or unpins a folder in the chooser.
async function toggleBookmark(path, pinned) {
  try {
    state = await api("POST", "/api/bookmark", { path, remove: pinned });
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  await browse(pickerPath);
}

// shortPath is the end of a path rather than the start, because the start is
// what every folder on one machine has in common: /mnt/tank/isos and
// /mnt/tank/proxmox/template/iso both read as "/mnt/tank/…" in a narrow
// column, and the ends are what tell them apart. The whole path is still the
// button's tooltip.
function shortPath(path) {
  const parts = path.split(/[\\/]+/).filter(Boolean);
  if (parts.length <= 2) return path;
  return `…/${parts.slice(-2).join("/")}`;
}

// rememberedRow is one folder isoshelf remembers: where it is, and under it
// when it was last looked at and what it held then. Those numbers are the
// last scan's, not what is there now - the drive may be in a drawer, and
// opening this list must never go looking for it.
function rememberedRow(folder, bookmarked) {
  const detail = [
    folder.last_used ? timeAgo(folder.last_used) : null,
    folder.files ? plural(folder.files, "image") : null,
    folder.bytes ? formatBytes(folder.bytes) : null,
  ].filter(Boolean).join(" · ");
  return el("div", { class: "remembered" },
    el("div", { class: "remembered-row" },
      el("button", {
        type: "button", class: "remembered-open", title: folder.path,
        onclick: () => browse(folder.path),
      }, bookmarked ? `★ ${shortPath(folder.path)}` : shortPath(folder.path)),
      el("button", {
        type: "button", class: "remembered-forget",
        "aria-label": `Forget ${folder.path}`,
        title: "Take this folder off the list. Nothing in it is touched.",
        onclick: () => forgetFolder(folder),
      }, "✕")),
    detail ? el("div", { class: "remembered-detail" }, detail) : null);
}

// forgetFolder takes a folder off the list. It is worth asking first, because
// the word sounds bigger than it is - so the question says exactly what goes.
async function forgetFolder(folder) {
  const yes = await ask(
    `Forget ${folder.path}?`,
    "This takes it off the list and throws away isoshelf's copy of what it found there. " +
    "The folder, the images in it, its archive and its own records are not touched — " +
    "open it again and isoshelf reads them back.",
    [
      { label: "Forget it", value: "yes", primary: true },
      { label: "Cancel", value: null },
    ]);
  if (!yes) return;
  try {
    state = await api("POST", "/api/folders/forget", { path: folder.path, id: folder.id });
    render();
    await browse(pickerPath);
  } catch (err) {
    showPickerError(err.message);
  }
}
