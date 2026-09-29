"use strict";

// This folder, in Settings: the folder isoshelf is watching, what kind it
// is, and where what isoshelf learns about it is kept (v0.8.8 put them in
// one group; the folder used to be listed twice on a server, once here and
// once under "Where things are"). Moved out of folders.js, which is the
// chooser itself.

const PROFILES = [
  ["folder", "Folder of images"],
  ["ventoy", "Ventoy drive"],
  ["proxmox", "Proxmox ISO storage"],
];

const FOLDER_SETTINGS = {
  title: "This folder",
  short: "Folder",
  settings: [
    {
      name: "The folder isoshelf is watching",
      fields: ["target"],
      hint: "The images in this folder are the ones the page is about.",
      words: "folder drive target ventoy nas path choose change another location",
      control: () => el("div", { class: "setting-controls" },
        el("div", { class: "file" }, (state && state.target) || "No folder chosen yet"),
        el("button", { type: "button", class: "btn small", disabled: cantSwitch(), onclick: openPicker },
          "Choose folder…")),
    },
    {
      // On a desktop the type is on the folder card; on a server the card is
      // one line and this is where it lives (v0.7.0).
      name: "Folder type",
      fields: ["profile"],
      hint: "What boots from this folder, which decides which files can.",
      words: "type ventoy proxmox profile boot menu kind",
      available: () => state && state.server && state.target,
      control: () => el("select", {
        "aria-label": "Folder type", disabled: cantSwitch(),
        onchange: (e) => changeProfile(e.target.value),
      }, PROFILES.map(([value, text]) =>
        el("option", { value, selected: value === (state.profile || "ventoy") || undefined }, text))),
    },
    {
      name: "Where this folder's records are kept",
      hint: "What isoshelf has worked out about this folder: its history, the images " +
        "you starred, and what each file turned out to be. Normally kept in the folder " +
        "itself, so the drive carries them with it.",
      words: "records state history stars where kept read-only folder drive",
      available: () => state && state.target,
      control: () => recordsControl(),
      note: () => recordsNote(),
    },
  ],
};

// cantSwitch is true while the folder mustn't change under what is running:
// a scan, a download, or a file still arriving from this computer. The
// server refuses then too.
function cantSwitch() {
  return scanning() || downloading() || uploading();
}

// changeProfile says what kind of folder this is, and reads it again as one.
async function changeProfile(profile) {
  try {
    await api("POST", "/api/target", { path: state.target, profile });
    catalog = null;
    saved(["profile"]);
    await start("scan");
  } catch (err) {
    showNotice(err.message, true);
  }
}
