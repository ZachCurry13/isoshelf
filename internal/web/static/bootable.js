"use strict";

// Make bootable (#3): a file the boot menu won't list, renamed or unpacked
// into a shape it will. The server works out the fix from the catalog and
// the file's own content (/api/fixup/plan); this shows it and asks. A rename
// happens at once; an unpack goes through the downloads, where its progress
// shows, and the compressed file is archived or kept, never deleted.

async function makeBootable(item) {
  let fix;
  try {
    fix = await api("POST", "/api/fixup/plan", { path: item.path });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  let original = null;
  if (fix.kind === "rename") {
    const go = await ask(
      `Rename ${baseName(fix.from)}?`,
      `It becomes ${baseName(fix.to)}. The file itself doesn't change: its content is already a disk image, and the boot menu only lists it under that name.`,
      [{ label: "Rename", value: true, primary: true }, { label: "Cancel", value: null }]);
    if (!go) return;
  } else {
    const size = fix.size ? `about ${formatBytes(fix.size)}` : "a size the file doesn't say";
    original = await ask(
      `Unpack ${baseName(fix.from)}?`,
      `The image inside, ${baseName(fix.to)}, takes ${size}, beside the compressed file until it is complete. ` +
      "What happens to the compressed file afterwards?",
      [
        { label: "Unpack and archive it", value: "move-aside", primary: true },
        { label: "Unpack and keep it", value: "keep" },
        { label: "Cancel", value: null },
      ]);
    if (!original) return;
  }
  try {
    await api("POST", "/api/fixup", { path: item.path, original });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  if (fix.kind === "rename") flashNotice(`Renamed to ${baseName(fix.to)}.`);
  await refresh();
}

function baseName(p) {
  return p.slice(p.lastIndexOf("/") + 1);
}

// bootableButton is the row's or the panel's button, for a file it can fix.
function bootableButton(item, small) {
  if (!item.fixable) return null;
  return jobButton(item.entry, el("button", {
    type: "button", class: small ? "btn small primary" : "btn primary", disabled: scanning(),
    "data-rowfocus": small ? "action" : null,
    onclick: () => makeBootable(item),
  }, "Make bootable"), true);
}
