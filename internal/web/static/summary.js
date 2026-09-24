"use strict";

// What wants doing, as one line above the list (v0.7.0, #54):
//
//   41 updates · 13 older · 3 unrecognized · 5 won't boot · Archive 20.9 GB   [Update 31]
//
// Each count shows exactly the images it counted, and the one thing worth a
// button of its own - updating everything isoshelf can - sits at the end. It
// used to be a card for each, which on a busy drive was a screenful before
// the list began. The detail those cards spelled out is in each part's
// tooltip; the list it opens onto says the rest.

function renderTodo() {
  const bar = $("todo");
  if (!state.report) {
    bar.replaceChildren();
    return;
  }
  const items = state.report.items;
  const parts = [];

  const updates = updatable();
  const byHand = items.filter((it) => it.status === "update available" && it.updates !== "download").length;
  const all = updates.length + byHand;
  const waiting = updates.filter((it) => inDownloads(it.entry)).length;
  if (all) {
    const bytes = updates.reduce((sum, it) => sum + downloadSize(it), 0);
    parts.push(summaryPart(all, all === 1 ? "update" : "updates", "s-update", showUpdates, [
      bytes ? `about ${formatBytes(bytes)} for isoshelf to download` : "",
      byHand ? `${byHand} to download yourself` : "",
      waiting ? `${waiting} already in the downloads` : "",
    ]));
  }

  const older = items.filter(isOlder);
  if (older.length) {
    const bytes = older.reduce((sum, it) => sum + (it.size || 0), 0);
    parts.push(summaryPart(older.length, "older", "", () => showJust(() => { view.show.older = true; }), [
      "You already have a newer version of each of these",
      bytes ? `${formatBytes(bytes)} you could free` : "",
    ]));
  }

  const count = (status) => items.filter((it) => it.status === status).length;
  const missing = count("missing");
  if (missing) {
    parts.push(summaryPart(missing, "missing", "s-missing", () => showOnly("missing"),
      ["You usually keep these in this folder"]));
  }
  const unknown = count("unrecognized");
  if (unknown) {
    parts.push(summaryPart(unknown, "unrecognized", "", () => showOnly("unrecognized"),
      ["isoshelf can try to identify them"]));
  }
  const stuck = count("not bootable");
  if (stuck) {
    parts.push(summaryPart(stuck, "won't boot", "s-warn", () => showOnly("not bootable"),
      ["Images, but not in a format this folder's boot menu can use"]));
  }

  if (state.removed && state.removed.files) {
    parts.push(el("button", {
      type: "button", class: "summary-part",
      title: `${plural(state.removed.files, "file")} removed but kept, still using space in this folder`,
      onclick: () => jumpTo("archive"),
    }, "Archive ", el("b", {}, formatBytes(state.removed.bytes))));
  }

  if (state.folder_changed) {
    parts.push(el("button", {
      type: "button", class: "summary-part", disabled: scanning() || downloading(),
      title: "Files were added or removed outside isoshelf. Scan again to catch up.",
      onclick: () => start("scan"),
    }, "Changed outside isoshelf: ", el("b", {}, "scan again")));
  }

  const line = parts.flatMap((part, i) => i ? [el("span", { class: "summary-sep", "aria-hidden": "true" }, "·"), part] : [part]);
  if (updates.length) {
    const left = updates.length - waiting;
    line.push(el("button", {
      type: "button", class: "btn primary small summary-go", disabled: left === 0,
      onclick: updateAll,
    }, left === 0 ? "All queued" : left === all ? "Update all" : `Update ${left}`));
  }
  bar.replaceChildren(...line);
}

// summaryPart is one count: the number stands out, and the tooltip says what
// the old card used to.
function summaryPart(n, word, tone, onclick, notes) {
  return el("button", {
    type: "button", class: `summary-part ${tone}`,
    title: notes.filter(Boolean).join(". ") + ". Click to show them.",
    onclick,
  }, el("b", {}, String(n)), ` ${word}`);
}

// jumpTo scrolls to a part of the page. It used to open the first <details>
// in it as well, from when the archive folded away; the archive doesn't any
// more, and the first one in the list is the Filter menu, which it opened
// every time a count was clicked.
function jumpTo(id) {
  $(id).scrollIntoView({ behavior: "smooth", block: "start" });
}

// showOnly filters the list down to one status, as a chip you can remove.
function showOnly(status) {
  showJust(() => { view.status = status; });
}

// showUpdates is the same for the updates, whether or not isoshelf can
// download them: the count says all of them, so the list shows all of them.
function showUpdates() {
  showJust(() => { view.show.updates = true; });
}

// showJust is what clicking a count does: exactly what it counted. Any
// other filter goes first - one left over would show fewer than the count
// said, which reads as the count being wrong.
function showJust(set) {
  resetFilters();
  set();
  saveView();
  renderFilters();
  renderRows();
  jumpTo("images");
}

// ---- Asked once, in a container -------------------------------------------

// renderFirstRun asks whether the images should update by themselves, in a
// container, until somebody answers (v0.7.0). An app on a NAS is left running
// for months and nobody goes looking for a switch they weren't told about;
// until the answer it is off, like everywhere else.
function renderFirstRun() {
  const box = $("first-run");
  box.hidden = !state.ask_auto_update;
  if (box.hidden) return;
  // None of the three is the one to pick, so none looks like it.
  const answer = (label, change, said) => el("button", {
    type: "button", class: "btn",
    onclick: async () => {
      try {
        state = await api("POST", "/api/settings", change);
      } catch (err) {
        showNotice(err.message, true);
        return;
      }
      flashNotice(`${said} Settings can change that whenever you like.`);
      render();
    },
  }, label);
  box.replaceChildren(
    el("div", { class: "first-run-title", id: "first-run-title" }, "Update the images by themselves?"),
    el("p", {}, "isoshelf can look for updates on a schedule, then download each one, verify it " +
      "and put it in place with nothing to press. A pinned file stays where it is."),
    el("p", {}, firstRunNow()),
    el("div", { class: "first-run-buttons" },
      answer("Every day", { auto_update: true, auto_update_every: "day" }, "The images will update every day."),
      answer("Every week", { auto_update: true, auto_update_every: "week" }, "The images will update every week."),
      answer("No, I'll press Update", { auto_update: false }, "Updates will wait for you to press Update.")));
}

// firstRunNow says what yes does straight away, because it does: the first
// run starts the moment it is turned on. Found by answering it on a test
// folder and watching it set off to update everything in it.
function firstRunNow() {
  const updates = state.report ? updatable() : [];
  const gone = {
    replace: "each old file is deleted once its update is verified (Settings can archive them instead)",
    archive: "each old file moves to the archive",
    keep: "the old files stay beside the new ones",
  }[state.old_files || "replace"];
  if (!updates.length) return `Yes also runs once now, then on the schedule. When updates come, ${gone}.`;
  const bytes = updates.reduce((sum, it) => sum + downloadSize(it), 0);
  return `Yes starts the first run now: ${plural(updates.length, "update")}` +
    `${bytes ? `, about ${formatBytes(bytes)} to download` : ""}, and ${gone}.`;
}
