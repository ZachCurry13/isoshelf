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
// so the page opens in the right colours instead of changing under the
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
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

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
        hint: "Light, dark, or whatever this computer is set to.",
        words: "dark mode night light colour color appearance",
        control: () => choiceRow("Theme", [
          ["", "Match this computer"],
          ["light", "Light"],
          ["dark", "Dark"],
        ], look().theme || "", (theme) => ({ theme })),
      },
      {
        name: "Higher contrast",
        hint: "Stronger words and firmer edges, for a bright room or tired eyes.",
        words: "accessibility contrast readable bold",
        control: () => switchRow("Higher contrast", look().high_contrast, (on) => ({ high_contrast: on })),
      },
      {
        name: "Larger text",
        hint: "Everything on the page a size bigger, without the browser's zoom.",
        words: "accessibility big font size zoom text",
        control: () => switchRow("Larger text", look().larger_text, (on) => ({ larger_text: on })),
      },
      {
        name: "Less movement",
        hint: "Stops the spinners and bars from moving while isoshelf works.",
        words: "accessibility animation motion spinner still",
        control: () => switchRow("Less movement", look().reduce_motion, (on) => ({ reduce_motion: on })),
      },
    ],
  },
  {
    title: "Updates and old files",
    settings: [
      {
        name: "What happens to the file an update replaces",
        hint: "For images you haven't answered for yourself. Each image can say " +
          "otherwise in its own panel, and archived files can be put back until you empty the archive.",
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
        hint: "New images arrive without a new isoshelf. Only the project's own " +
          "repository is ever fetched, and a list that doesn't pass every check is refused.",
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
        hint: "Opens a new issue on the project with the version already filled in. " +
          "Nothing is sent until you press send yourself.",
        words: "bug problem issue report help support broken",
        available: () => state && state.report_url,
        control: () => el("div", { class: "setting-controls" },
          el("a", { class: "btn small", href: bugReportURL(), target: "_blank", rel: "noopener noreferrer" }, "Report a bug"),
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

// bugReportURL fills in what the maintainer would have to ask for anyway.
function bugReportURL() {
  const body = [
    "**What happened:**",
    "",
    "**What I expected:**",
    "",
    `**isoshelf:** ${(state && state.version) || "unknown"}`,
    `**Page:** ${navigator.userAgent}`,
  ].join("\n");
  return `${state.report_url}?title=${encodeURIComponent("Bug: ")}&body=${encodeURIComponent(body)}`;
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
    state.config_dir, state.catalog, Boolean(state.app_update), $("settings-search").value,
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
    el("div", { class: "setting-name" }, setting.name),
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
