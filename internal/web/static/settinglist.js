"use strict";

// What each setting is: the table the Settings panel is drawn from, and the
// two small controls most of its rows use. The panel itself is settings.js;
// the settings with requests of their own are access.js and records.js.

// look reads the appearance answers, with the defaults for anything unsaid.
function look() {
  return (state && state.appearance) || {};
}

// switchRow is the control most settings use: a tick box that saves itself.
// Most save through /api/settings, which is what change describes; the few
// with a request of their own pass instead, and change is then unused.
function switchRow(label, on, change, instead) {
  return el("label", { class: "check" },
    el("input", {
      type: "checkbox", checked: Boolean(on),
      onchange: (e) => (instead ? instead(e.target.checked) : saveSetting(change(e.target.checked))),
    }),
    " ", label);
}

// choiceRow is a list to pick from, saved the moment it changes.
function choiceRow(label, options, current, change) {
  return el("select", {
    "aria-label": label,
    onchange: (e) => saveSetting(change(e.target.value)),
  }, options.map(([value, text]) => el("option", { value, selected: value === current || undefined }, text)));
}

// The folder comes first where it is shown at all: see FOLDER_SETTINGS.
const SETTING_GROUPS = [
  FOLDER_SETTINGS,
  {
    title: "How it looks",
    settings: [
      {
        name: "Theme",
        fields: ["theme"],
        hint: "Light, dark, or whatever this computer is set to.",
        words: "dark mode night light color theme appearance",
        control: () => choiceRow("Theme", [
          ["", "Match this computer"],
          ["light", "Light"],
          ["dark", "Dark"],
        ], look().theme || "", (theme) => ({ theme })),
      },
      {
        name: "Higher contrast",
        fields: ["high_contrast"],
        hint: "Bolder text and stronger borders, for a bright room or tired eyes.",
        words: "accessibility contrast readable bold",
        control: () => switchRow("Higher contrast", look().high_contrast, (on) => ({ high_contrast: on })),
      },
      {
        name: "Larger text",
        fields: ["larger_text"],
        hint: "Everything on the page one size bigger, without using the browser's zoom.",
        words: "accessibility big font size zoom text",
        control: () => switchRow("Larger text", look().larger_text, (on) => ({ larger_text: on })),
      },
      {
        name: "Less movement",
        fields: ["reduce_motion"],
        hint: "Stops spinners and progress bars from animating while isoshelf works.",
        words: "accessibility animation motion spinner still",
        control: () => switchRow("Less movement", look().reduce_motion, (on) => ({ reduce_motion: on })),
      },
    ],
  },
  {
    title: "Checking for updates",
    settings: [
      {
        name: "Check for updates automatically",
        fields: ["auto_check"],
        hint: "Asks each project for its newest version when the page opens, and " +
          "remembers the answer for a day.",
        words: "automatic online check updates network offline internet",
        control: () => switchRow("Check for updates automatically", state && state.auto_check,
          (on) => ({ auto_check: on })),
        note: () => (state && state.checked_at)
          ? `Last asked ${timeAgo(state.checked_at)}.`
          : "",
      },
      {
        name: "Update images automatically",
        fields: ["auto_update", "auto_update_every"],
        hint: "Downloads and installs updates on a schedule, with nothing to press. " +
          "Verified the same way as when you press Update yourself.",
        words: "automatic update download schedule unattended daily weekly",
        control: () => el("div", { class: "setting-controls" },
          switchRow("Update images automatically", state && state.auto_update,
            (on) => ({ auto_update: on })),
          (state && state.auto_update)
            ? choiceRow("How often", [["day", "Every day"], ["week", "Every week"]],
              (state && state.auto_update_every) || "day", (every) => ({ auto_update_every: every }))
            : null),
        note: () => (state && state.auto_update)
          ? "Changes your folder while you aren't watching. Old files go the way Updates and " +
            "old files says, a pinned file stays where it is, nothing is deleted that you hadn't " +
            "already chosen to lose, and it stops before the folder is full."
          : "",
      },
      {
        name: "Tell me about new isoshelf versions",
        fields: ["app_update_check"],
        hint: "Checks GitHub for a newer isoshelf and says so in the top bar. Nothing " +
          "about you is sent, and nothing is installed until you press Update now.",
        words: "isoshelf version update release new notify github self upgrade restart",
        control: () => switchRow("Tell me about new isoshelf versions", state && state.app_update_check,
          (on) => ({ app_update_check: on })),
        note: () => selfUpdateNote(),
      },
    ],
  },
  {
    title: "Updates and old files",
    settings: [
      {
        name: "What happens to the file an update replaces",
        fields: ["old_files"],
        hint: "For every image. To keep one exact file whatever happens, pin it " +
          "in its own panel.",
        words: "replace archive delete keep both old copies updates pin pinned",
        control: () => choiceRow("What happens to the file an update replaces", [
          ["replace", "Replace it"],
          ["archive", "Move it to the archive"],
          ["keep", "Keep both"],
        ], (state && state.old_files) || "replace", (old_files) => ({ old_files })),
      },
      {
        name: "Dismissed updates",
        fields: ["dismissed"],
        hint: "Updates you asked not to be shown, for a while or for good. Undo brings one back.",
        words: "dismiss dismissed hide ignore skip never update snooze undo",
        available: () => dismissedEntries().length > 0,
        control: () => dismissedControl(),
      },
      {
        name: "Empty the archive by itself",
        fields: ["archive_after"],
        hint: "Delete archived files once they have waited this long. Off unless " +
          "you choose a number: the archive is how a removal is undone.",
        words: "archive empty delete timer days automatic old removed space free",
        control: () => choiceRow("Empty the archive by itself", [
          [0, "Never"],
          [7, "After 7 days"],
          [30, "After 30 days"],
          [90, "After 90 days"],
        ], (state && state.archive_after) || 0,
          (archive_after) => ({ archive_after: Number(archive_after) })),
        // Said before it happens, not after: this is the one thing isoshelf
        // does by itself that throws something away.
        note: () => {
          if (!state || !state.archive_after) return "";
          if (!state.archive_due) {
            return `Nothing in the archive has waited ${state.archive_after} days yet.`;
          }
          return `${plural(state.archive_due, "file")} in the archive ` +
            `${state.archive_due === 1 ? "has" : "have"} waited longer than ` +
            `${state.archive_after} days and will be deleted, freeing ` +
            `${formatBytes(state.archive_due_bytes)}. Restore anything you want to keep first.`;
        },
      },
    ],
  },
  {
    title: "The list of images",
    settings: [
      {
        name: "Keep the list of images up to date",
        fields: ["catalog_auto"],
        hint: "New images without waiting for a new isoshelf. Only ever read from " +
          "this project's own repository.",
        words: "catalog list update images automatic",
        available: () => state && state.catalog && state.catalog.can_auto,
        control: () => el("div", { class: "setting-controls" },
          switchRow("Keep the list of images up to date", state.catalog.auto, (on) => ({ catalog_auto: on })),
          el("button", { type: "button", class: "btn small", onclick: refreshCatalog }, "Check now")),
        note: () => catalogNote(),
      },
      {
        name: "Your own list of images",
        hint: "isoshelf never changes a list you wrote yourself.",
        words: "catalog own yours custom toml",
        available: () => state && state.catalog && !state.catalog.can_auto,
        control: () => el("div", { class: "muted" }, catalogNote() || "In use."),
      },
    ],
  },
  {
    title: "Where things are",
    settings: [
      {
        name: "The folder isoshelf is watching",
        hint: "The images in this folder are the ones the page is about.",
        words: "folder drive target ventoy nas path",
        control: () => el("div", { class: "setting-controls" },
          el("div", { class: "file" }, (state && state.target) || "No folder chosen yet"),
          el("button", { type: "button", class: "btn small", onclick: openPicker }, "Choose folder…")),
      },
      {
        name: "Sign-in",
        hint: "One username and password for this isoshelf. Once set, it is the only " +
          "way in - the link isoshelf prints when it starts stops working.",
        words: "login password username sign in out account security who access",
        available: () => state && state.login && state.login.can_set,
        control: () => loginControl(),
        note: () => (state && state.login && state.login.user)
          ? `Signed in as ${state.login.user}. Forgotten the password? Run \u201cisoshelf password\u201d ` +
            "on the machine isoshelf runs on, or set ISOSHELF_USERNAME and ISOSHELF_PASSWORD and restart it."
          : "Nobody has set one yet, so anyone who can reach this address can use isoshelf.",
      },
      {
        name: "Copy from another isoshelf first",
        hint: "If the isoshelf on your NAS already has an image, take it from there " +
          "instead of downloading it again over the internet.",
        words: "local network nas peer server copy share fast lan source",
        control: () => peerControl(),
        note: () => peerNote(),
      },
      {
        name: "Share this folder with other isoshelfs",
        // Only a server can be reached by another isoshelf: a desktop or
        // portable one answers this computer alone, so offering to share from
        // it was a switch that could never do anything.
        available: () => state && state.server,
        fields: ["share"],
        hint: "Offers the images in this folder to another isoshelf on your network that " +
          "signs in. Off unless you turn it on.",
        words: "share serve local network nas peer host offer",
        control: () => switchRow("Share this folder with other isoshelfs",
          state && state.peer && state.peer.sharing, null, shareImages),
        note: () => (state && state.peer && state.peer.sharing)
          ? "Anyone who can sign in to this isoshelf can copy whole images from it. Scans " +
            "also hash every image now, which the first one after turning this on will spend " +
            "time doing - that hash is how another isoshelf asks for a particular file."
          : "",
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
      {
        name: "isoshelf's own folder",
        hint: "Its settings, its copy of the image list, and its logs.",
        words: "config folder settings where files portable",
        available: () => state && state.config_dir,
        control: () => el("div", {},
          el("div", { class: "file" }, state.config_dir),
          state.portable ? el("div", { class: "muted" }, "Running portable: isoshelf keeps everything on the drive it's on.") : null),
      },
    ],
  },
  {
    title: "Help",
    settings: [
      {
        name: "Version",
        hint: "The version of isoshelf you are running.",
        words: "version about update release",
        control: () => el("div", { class: "setting-controls" },
          el("div", {}, (state && state.version) || "unknown"),
          state && state.app_update
            ? el("a", { class: "btn small", href: state.app_update.url, target: "_blank", rel: "noopener noreferrer" },
              `isoshelf ${state.app_update.latest} is available`)
            : null),
      },
      {
        name: "Report a bug",
        hint: "Shows the details, lets you copy them, and opens GitHub's form. " +
          "Nothing is sent until you press submit.",
        words: "bug problem issue report help support broken",
        available: () => state && state.report_url,
        control: () => el("div", { class: "setting-controls" },
          el("button", {
            type: "button", class: "btn small",
            onclick: () => reportProblem("", ""),
          }, "Report a bug"),
          el("a", { class: "btn small", href: projectURL(), target: "_blank", rel: "noopener noreferrer" }, "The project")),
      },
    ],
  },
];

// catalogNote says where the list of images came from and how it's doing, in
// the same words the footer uses.
function catalogNote() {
  const cat = (state && state.catalog) || {};
  return cat.error || cat.note || "";
}

function projectURL() {
  return state.report_url.replace(/\/issues\/new$/, "");
}
