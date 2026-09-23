"use strict";

// Where the open folder's records are kept. The one setting that owns files:
// changing it picks them up and puts them down somewhere else.

// saveRecords moves the open folder's records. It is not /api/settings: this
// one picks up files and puts them down somewhere else, so it has its own
// request, and it can say no - while a scan runs, or to a folder that would
// put the records on the same drive as the images.
async function saveRecords(change) {
  try {
    state = await api("POST", "/api/records", change);
    recordsChoice = null;
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

// recordsChoice is the answer being chosen right now, while "a folder you
// pick" is showing its box and nothing has been saved yet. Null means the
// panel shows what isoshelf actually does.
let recordsChoice = null;

// recordsControl is the three answers, plus the box for the third one. The
// first two save themselves; the third waits until a folder has been typed,
// because half a path is not a folder.
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
        recordsChoice = null;
        saveRecords({ location: value });
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
        if (e.key === "Enter") saveRecords({ location: "custom", dir: e.target.value });
      },
    });
    rows.push(box, el("button", {
      type: "button", class: "btn small",
      onclick: () => saveRecords({ location: "custom", dir: box.value }),
    }, "Use this folder"));
  }
  return el("div", { class: "setting-controls" }, rows);
}
