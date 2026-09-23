"use strict";

// Where the open folder's records are kept. The one setting that owns files:
// changing it picks them up and puts them down somewhere else. So it says
// what would move, from where to where, before anything does - and afterwards
// what happened, including when records were already waiting and nothing
// moved at all. The maintainer had to ask what the dropdown did, after it had
// already done it; that was the bug.

// recordsChoice is the answer being chosen right now, while "a folder you
// pick" is showing its box and nothing has been saved yet. Null means the
// panel shows what isoshelf actually does.
let recordsChoice = null;

// recordsResult is what the last move did, and for which folder. It is said
// under the setting for as long as that folder stays open.
let recordsResult = null;

// recordsBusy is set while a change is being asked about or made. A second
// one meanwhile - Enter and the button together, say - is ignored: two
// questions share the one dialog, and the answer to the second would be lost.
let recordsBusy = false;

// chooseRecords is what the list and the folder box call. It asks isoshelf
// what the change would do, asks the user whenever records would move or a
// different set would be used, and only then makes the change.
async function chooseRecords(change) {
  if (recordsBusy) return;
  recordsBusy = true;
  try {
    let plan;
    try {
      plan = await api("POST", "/api/records/plan", change);
    } catch (err) {
      showNotice(err.message, true);
      recordsBack(change);
      return;
    }
    if (plan.what === "same") {
      recordsBack(change);
      return;
    }
    if (plan.what !== "none" && !(await askRecords(plan, change))) {
      recordsBack(change);
      return;
    }
    await saveRecords(change);
  } finally {
    recordsBusy = false;
  }
}

// saveRecords makes the change. It is not /api/settings: this one picks up
// files and puts them down somewhere else, so it has its own request, and it
// can say no - while a scan runs, or to a folder that would put the records
// on the same drive as the images.
async function saveRecords(change) {
  try {
    state = await api("POST", "/api/records", change);
    recordsResult = { ...state.records.moved, target: state.target };
    recordsChoice = null;
    render();
  } catch (err) {
    showNotice(err.message, true);
    recordsBack(change);
  }
}

// recordsBack puts the list back to what isoshelf actually does, after a no
// or a refusal. A typed folder stays in its box, one edit from being right.
function recordsBack(change) {
  if (change.location === "custom") return;
  recordsChoice = null;
  renderSettings(true);
}

// askRecords is the question before a change. It names both files and what
// stays behind - or, when records are already waiting where these would go,
// which set wins and how old each one is.
function askRecords(plan, change) {
  const cancel = { label: "Cancel", value: false };
  if (plan.what === "move") {
    // Kept away from the folder, they are found by its path - the one thing
    // about this choice that can surprise somebody later, so it is said now.
    const away = change.location === "folder" ? "" :
      " Kept away from the folder, they're found by its path, so the same drive at a " +
      "different letter or mount point starts with nothing.";
    return ask("Move this folder's records?",
      "Its history, the images you starred and what each file turned out to be move " +
      `from one file to the other. Your images, the archive and part-finished downloads ` +
      `stay in ${state.target}.` + away,
      [{ label: "Move them", value: true, primary: true }, cancel],
      el("dl", { class: "records-move" },
        el("dt", {}, "From"), el("dd", {}, plan.from),
        el("dt", {}, "To"), el("dd", {}, plan.to)));
  }
  const left = plan.what === "keep"
    ? ` The ones in ${plan.from_dir}, saved ${timeAgo(plan.from_saved)}, stay where they ` +
      "are: delete them yourself if you don't want them."
    : "";
  return ask("Use the records already there?",
    `isoshelf already has records for this folder in ${plan.to_dir}, saved ` +
    `${timeAgo(plan.to_saved)}. It will use those.` + left,
    [{ label: "Use those", value: true, primary: true }, cancel]);
}

// recordsDone says what the last move did, in a line.
function recordsDone(m) {
  switch (m.what) {
    case "move": return `Moved here from ${m.from}.`;
    case "keep": return `Using the records that were already here. The ones in ${m.from_dir} ` +
      "are still there - delete them yourself if you don't want them.";
    case "use": return "Using the records that were already here.";
    case "none": return "Nothing to move yet. This folder's records are kept here from now on.";
    default: return "";
  }
}

// recordsNote is the note under the setting: what the last move did, if it
// was for this folder, and then where the records are now.
function recordsNote() {
  const r = (state && state.records) || {};
  if (!r.file) return "";
  let where = `Right now: ${r.file}`;
  if (r.location !== "folder") {
    where += " - kept away from the folder, they're found by its path, so the same " +
      "drive at a different letter or mount point starts with nothing. Your images " +
      "and the archive never move.";
  }
  const done = recordsResult && recordsResult.target === state.target ? recordsDone(recordsResult) : "";
  return [done ? el("div", { class: "records-done", role: "status" }, done) : null, where];
}

// recordsControl is the three answers, plus the box for the third one. The
// first two ask as soon as they are picked; the third waits until a folder
// has been typed, because half a path is not a folder.
function recordsControl() {
  const now = (state && state.records) || {};
  const chosen = recordsChoice === null ? (now.location || "folder") : recordsChoice;
  const rows = [
    el("select", {
      "aria-label": "Where this folder's records are kept",
      onchange: (e) => {
        const value = e.target.value;
        if (value === "custom") {
          recordsChoice = "custom";
          renderSettings(true);
          return;
        }
        chooseRecords({ location: value });
      },
    }, [
      ["folder", "In the folder itself"],
      ["app", "In isoshelf's own folder"],
      ["custom", "In a folder you pick"],
    ].map(([value, text]) => el("option", { value, selected: value === chosen || undefined }, text))),
  ];
  if (chosen === "custom") {
    const box = el("input", {
      type: "text", class: "records-dir", value: now.dir || "",
      placeholder: "The whole path to a folder",
      "aria-label": "The folder to keep this folder's records in",
      onkeydown: (e) => {
        if (e.key === "Enter") chooseRecords({ location: "custom", dir: e.target.value });
      },
    });
    rows.push(box, el("button", {
      type: "button", class: "btn small",
      onclick: () => chooseRecords({ location: "custom", dir: box.value }),
    }, "Use this folder"));
  }
  return el("div", { class: "setting-controls" }, rows);
}
