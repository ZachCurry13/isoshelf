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

// ---- Drawing the panel ---------------------------------------------------

// settingsKey is everything the panel shows. While it stays the same the
// panel is left alone, so a download finishing doesn't take the focus out of
// the control someone is using.
function settingsKey() {
  if (!state) return "";
  return JSON.stringify([
    state.appearance, state.old_files, state.target, state.version,
    state.auto_check, state.app_update_check, state.checked_at,
    state.auto_update, state.auto_update_every, state.peer,
    state.config_dir, state.catalog, state.records, recordsChoice, recordsResult, state.login,
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
