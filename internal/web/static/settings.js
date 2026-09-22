"use strict";

// Settings: one place for everything isoshelf lets you change, kept apart
// from the page itself so neither file grows past reading.
//
// Each setting is written down once, in SETTING_GROUPS: its name, a line
// saying what it does, the words someone might search for, and the control
// that changes it. The panel is drawn from that list, so a new setting is one
// entry rather than markup in three places.

let settingsOpen = false;
// lastLook is what was last put on the page, so the look isn't reapplied
// twice a second while a download runs.
let lastLook = "";
// drawnSettings is a fingerprint of what the panel last drew, for the same
// reason: redrawing while someone is using a control takes their focus away.
let drawnSettings = "";

// ---- The look of the page ------------------------------------------------

// The answers are kept by isoshelf, but a copy lives in the browser as well,
// so the page opens in the right colors instead of changing under the
// reader a moment later.
function savedLook() {
  try {
    return JSON.parse(localStorage.getItem("isoshelf.look") || "{}");
  } catch {
    return {};
  }
}

// applyAppearance puts the answers on the page: the theme, the contrast, the
// text size and whether anything moves. The stylesheet does the rest.
function applyAppearance(look) {
  look = look || {};
  const key = JSON.stringify(look);
  if (key === lastLook) return;
  lastLook = key;
  const root = document.documentElement;
  const set = (name, on, value) => {
    if (on) root.setAttribute(name, value);
    else root.removeAttribute(name);
  };
  set("data-theme", look.theme === "light" || look.theme === "dark", look.theme);
  set("data-contrast", look.high_contrast, "high");
  set("data-text", look.larger_text, "large");
  set("data-motion", look.reduce_motion, "reduce");
  try {
    localStorage.setItem("isoshelf.look", key);
  } catch {
    // A browser that won't remember is fine: isoshelf remembers anyway, and
    // the page catches up the moment it has the answer.
  }
}

applyAppearance(savedLook());

// darkNow says whether the page is dark at this moment, whichever way it got
// there: the theme chosen in Settings, or the computer's own setting.
function darkNow() {
  const chosen = document.documentElement.getAttribute("data-theme");
  if (chosen) return chosen === "dark";
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

// ---- Saving --------------------------------------------------------------

// saveSetting sends one answer and keeps whatever comes back, which is the
// whole page state: isoshelf is the one that remembers, not the browser.
async function saveSetting(change) {
  try {
    state = await api("POST", "/api/settings", change);
    applyAppearance(state.appearance);
    saved(Object.keys(change));
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

// Saying so when something saves.
//
// Everything here saves itself the moment it changes, which is the right
// behaviour and looks like nothing happening at all. So the setting that
// changed says "Saved" for a few seconds - next to that setting rather than
// somewhere general, because "saved" only reassures if you can tell what was.

// savedFields are the answers saved a moment ago, and savedAt when.
let savedFields = [];
let savedAt = 0;
let savedTimer = null;

const SAVED_FOR = 3000;

function saved(fields) {
  savedFields = fields;
  savedAt = Date.now();
  clearTimeout(savedTimer);
  // One redraw when the mark's time is up, rather than polling to notice:
  // nothing else about the panel is changing meanwhile.
  savedTimer = setTimeout(() => {
    savedFields = [];
    renderSettings(true);
  }, SAVED_FOR);
}

// justSaved says whether this setting is one that saved a moment ago.
function justSaved(setting) {
  if (!setting.fields || Date.now() - savedAt > SAVED_FOR) return false;
  return setting.fields.some((field) => savedFields.includes(field));
}

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

// ---- The settings themselves ---------------------------------------------

// look reads the appearance answers, with the defaults for anything unsaid.
function look() {
  return (state && state.appearance) || {};
}

// switchRow is the control most settings use: a tick box that saves itself.
function switchRow(label, on, change) {
  return el("label", { class: "check" },
    el("input", { type: "checkbox", checked: Boolean(on), onchange: (e) => saveSetting(change(e.target.checked)) }),
    " ", label);
}

// choiceRow is a list to pick from, saved the moment it changes.
function choiceRow(label, options, current, change) {
  return el("select", {
    "aria-label": label,
    onchange: (e) => saveSetting(change(e.target.value)),
  }, options.map(([value, text]) => el("option", { value, selected: value === current || undefined }, text)));
}

const SETTING_GROUPS = [
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
        hint: "Stronger words and firmer edges, for a bright room or tired eyes.",
        words: "accessibility contrast readable bold",
        control: () => switchRow("Higher contrast", look().high_contrast, (on) => ({ high_contrast: on })),
      },
      {
        name: "Larger text",
        fields: ["larger_text"],
        hint: "Everything on the page a size bigger, without the browser's zoom.",
        words: "accessibility big font size zoom text",
        control: () => switchRow("Larger text", look().larger_text, (on) => ({ larger_text: on })),
      },
      {
        name: "Less movement",
        fields: ["reduce_motion"],
        hint: "Stops the spinners and bars from moving while isoshelf works.",
        words: "accessibility animation motion spinner still",
        control: () => switchRow("Less movement", look().reduce_motion, (on) => ({ reduce_motion: on })),
      },
    ],
  },
  {
    title: "Checking for updates",
    settings: [
      {
        name: "Check for updates by itself",
        fields: ["auto_check"],
        hint: "Asks each project for its newest version when the page opens, and " +
          "remembers the answer for a day.",
        words: "automatic online check updates network offline internet",
        control: () => switchRow("Check for updates by itself", state && state.auto_check,
          (on) => ({ auto_check: on })),
        note: () => (state && state.checked_at)
          ? `Last asked ${timeAgo(state.checked_at)}.`
          : "",
      },
      {
        name: "Update the images by itself",
        fields: ["auto_update", "auto_update_every"],
        hint: "Downloads and installs updates on a schedule, with nothing to press. " +
          "Verified the same way as when you press Update yourself.",
        words: "automatic update download schedule unattended by itself daily weekly",
        control: () => el("div", { class: "setting-controls" },
          switchRow("Update the images by itself", state && state.auto_update,
            (on) => ({ auto_update: on })),
          (state && state.auto_update)
            ? choiceRow("How often", [["day", "Every day"], ["week", "Every week"]],
              (state && state.auto_update_every) || "day", (every) => ({ auto_update_every: every }))
            : null),
        note: () => (state && state.auto_update)
          ? "Changes your folder while you aren't watching. Each image keeps its own answer " +
            "about the file it replaces, nothing is deleted that you hadn't already chosen to " +
            "lose, and it stops before the folder is full."
          : "",
      },
      {
        name: "Tell me when a new isoshelf is out",
        fields: ["app_update_check"],
        hint: "Checks GitHub for a newer isoshelf. Nothing is installed and nothing " +
          "about you is sent - it's a link in the top bar.",
        words: "isoshelf version update release new notify github",
        control: () => switchRow("Tell me when a new isoshelf is out", state && state.app_update_check,
          (on) => ({ app_update_check: on })),
      },
    ],
  },
  {
    title: "Updates and old files",
    settings: [
      {
        name: "What happens to the file an update replaces",
        fields: ["old_files"],
        hint: "For images you haven't answered for yourself. Each one can choose " +
          "differently in its own panel.",
        words: "replace archive delete keep both old copies updates",
        control: () => choiceRow("What happens to the file an update replaces", [
          ["replace", "Replace it"],
          ["archive", "Move it to the archive"],
          ["keep", "Keep both"],
        ], (state && state.old_files) || "replace", (old_files) => ({ old_files })),
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
        name: "Who can get in",
        hint: "One username and password for this isoshelf. Once set, it's the only " +
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
        name: "Where this folder's records are kept",
        hint: "What isoshelf has worked out about this folder: its history, the images " +
          "you starred, and what each file turned out to be. Normally kept in the folder " +
          "itself, so the drive carries them with it.",
        words: "records state history stars where kept read-only folder drive",
        available: () => state && state.target,
        control: () => recordsControl(),
        note: () => {
          const r = (state && state.records) || {};
          if (!r.file) return "";
          const where = `Right now: ${r.file}`;
          if (r.location === "folder") return where;
          // Kept away from the folder, they are found by its path - which is
          // the one thing about this choice that can surprise somebody later.
          return where + " - kept away from the folder, they're found by its path, so the same " +
            "drive at a different letter or mount point starts with nothing. Your images " +
            "and the archive never move.";
        },
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
        name: "This isoshelf",
        hint: "The version you're running.",
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

// loginControl is the username and password: a button that opens the form,
// and the two ways out. Changing it is rare enough that it does not need to
// sit open in the panel taking up room.
function loginControl() {
  const login = (state && state.login) || {};
  const buttons = [
    el("button", { type: "button", class: "btn small", onclick: changeLogin },
      login.user ? "Change username or password" : "Set a username and password"),
  ];
  if (login.user) {
    buttons.push(
      el("a", { class: "btn small", href: "/login", onclick: signOut }, "Sign out"),
      el("button", { type: "button", class: "btn small", onclick: signOutEverywhere },
        "Sign out everywhere"));
  }
  return el("div", { class: "setting-controls" }, buttons);
}

// changeLogin asks for the new username and password, and the old one unless
// this browser got in with the link - in which case the old password is the
// thing that has been forgotten.
async function changeLogin() {
  const login = (state && state.login) || {};
  const body = el("div", { class: "report" });
  const field = (label, name, type, hint) => {
    const input = el("input", { type, id: `login-${name}`, class: "records-dir",
      autocomplete: type === "password" ? "new-password" : "username" });
    body.append(el("div", {},
      el("label", { for: `login-${name}` }, label),
      input,
      hint ? el("div", { class: "muted setting-note" }, hint) : null));
    return input;
  };

  let current = null;
  if (login.user) {
    current = field("Your password now", "current", "password");
    current.autocomplete = "current-password";
  }
  const user = field("Username", "user", "text");
  user.value = login.user || "";
  const password = field("Password", "password", "password");
  const again = field("Password again", "again", "password");
  body.append(el("p", { class: "muted" },
    "Every other browser is signed out when this changes, so a changed password " +
    "means a changed password everywhere."));

  const go = await ask(login.user ? "Change the login" : "Set a username and password", "", [
    { label: "Save", value: "go", primary: true },
    { label: "Cancel", value: "" },
  ], body);
  if (!go) return;
  try {
    state = await api("POST", "/api/login", {
      user: user.value,
      current: current ? current.value : "",
      password: password.value,
      again: again.value,
    });
    render();
    showNotice("Saved. Everywhere else has been signed out.", false);
  } catch (err) {
    showNotice(err.message, true);
  }
}

// signOut ends this browser's session. It is a plain link to the login page,
// which posts to sign out - so it works even if this script never ran.
function signOut(e) {
  e.preventDefault();
  const form = el("form", { method: "post", action: "/login" });
  document.body.append(form);
  form.submit();
}

async function signOutEverywhere() {
  const yes = await ask("Sign out everywhere?",
    "Every browser signed in to this isoshelf, including this one, has to sign in " +
    "again. The username and password don't change.",
    [{ label: "Sign out everywhere", value: "yes", primary: true }, { label: "Cancel", value: null }]);
  if (!yes) return;
  try {
    await api("POST", "/api/login/everywhere", {});
  } catch {
    // Signing out and then being refused is the point; either way, reload.
  }
  location.href = "/login";
}

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

// catalogNote says where the list of images came from and how it's doing, in
// the same words the footer uses.
function catalogNote() {
  const cat = (state && state.catalog) || {};
  return cat.error || cat.note || "";
}

function projectURL() {
  return state.report_url.replace(/\/issues\/new$/, "");
}

// ---- Drawing the panel ---------------------------------------------------

// settingsKey is everything the panel shows. While it stays the same the
// panel is left alone, so a download finishing doesn't take the focus out of
// the control someone is using.
function settingsKey() {
  if (!state) return "";
  return JSON.stringify([
    state.appearance, state.old_files, state.target, state.version,
    state.auto_check, state.app_update_check, state.checked_at,
    state.auto_update, state.auto_update_every,
    state.config_dir, state.catalog, state.records, recordsChoice, state.login,
    Boolean(state.app_update), $("settings-search").value,
    savedFields.join(","),
  ]);
}

function renderSettings(force) {
  if (!settingsOpen || !state) return;
  const key = settingsKey();
  if (!force && key === drawnSettings) return;
  drawnSettings = key;

  const find = $("settings-search").value.trim().toLowerCase();
  const body = $("settings-body");
  body.replaceChildren();
  let shown = 0;
  for (const group of SETTING_GROUPS) {
    const rows = group.settings
      .filter((setting) => !setting.available || setting.available())
      .filter((setting) => matches(setting, group, find))
      .map((setting) => settingRow(setting));
    if (!rows.length) continue;
    shown += rows.length;
    body.append(el("section", { class: "setting-group" },
      el("h3", {}, group.title), rows));
  }
  if (!shown) {
    body.append(el("p", { class: "muted" }, `Nothing in Settings matches “${$("settings-search").value.trim()}”.`));
  }
  $("settings-where").textContent = "Kept on this computer, not in the folder.";
}

// matches decides whether a setting belongs in a search. The group's name
// counts too, so "look" finds everything under "How it looks".
function matches(setting, group, find) {
  if (!find) return true;
  const haystack = [group.title, setting.name, setting.hint, setting.words].join(" ").toLowerCase();
  return find.split(/\s+/).every((word) => haystack.includes(word));
}

function settingRow(setting) {
  const note = setting.note ? setting.note() : "";
  return el("div", { class: "setting" },
    el("div", { class: "setting-name" },
      setting.name,
      justSaved(setting) ? el("span", { class: "saved-mark" }, "Saved") : null),
    el("div", { class: "setting-hint" }, setting.hint),
    el("div", { class: "setting-control" }, setting.control()),
    note ? el("div", { class: "muted setting-note" }, note) : null);
}

// ---- Opening, closing, resetting -----------------------------------------

function openSettings() {
  if (detailsOpen) closeDetails();
  settingsOpen = true;
  $("settings").hidden = false;
  $("settings-open").setAttribute("aria-expanded", "true");
  renderSettings(true);
  $("settings-close").focus();
}

function closeSettings() {
  settingsOpen = false;
  $("settings").hidden = true;
  $("settings-open").setAttribute("aria-expanded", "false");
  $("settings-open").focus();
}

// resetSettings puts the switches back without forgetting the folder anyone
// is working in, so it is safe to press when the page looks wrong.
async function resetSettings() {
  const answer = await ask("Put every setting back?",
    "Everything in Settings goes back to how isoshelf comes out of the box. " +
    "Your folder, your pinned folders and everything isoshelf has learned about your images stay as they are.",
    [{ label: "Cancel", value: null }, { label: "Reset to defaults", value: "reset", primary: true }]);
  if (answer !== "reset") return;
  await saveSetting({ reset: true });
  renderSettings(true);
  showNotice("Settings are back to their defaults.");
}

document.addEventListener("DOMContentLoaded", () => {
  $("settings-open").addEventListener("click", () => (settingsOpen ? closeSettings() : openSettings()));
  $("settings-close").addEventListener("click", closeSettings);
  $("settings-reset").addEventListener("click", resetSettings);
  $("settings-search").addEventListener("input", () => renderSettings(true));
});
