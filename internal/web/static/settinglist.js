"use strict";

// What each setting is: the table the Settings panel is drawn from, and the
// two small controls most of its rows use. The panel itself is settings.js.
// Groups live with the parts of the page they belong to: This folder in
// thisfolder.js, sign-in and other isoshelfs in access.js, and isoshelf
// itself in selfupdate.js; settingGroups puts them in order.

// look reads the appearance answers, with the defaults for anything unsaid.
function look() {
  return (state && state.appearance) || {};
}

// switchRow is the control most settings use: Primer's ToggleSwitch, which
// applies the setting at once and says so (nothing to press). It is labelled
// with what it controls, not with its state. Most save through /api/settings,
// which is what change describes; the few with a request of their own pass
// instead, and change is then unused.
function switchRow(label, on, change, instead) {
  return el("span", { class: "switch-row" },
    el("span", { class: "switch-label" }, label),
    el("label", { class: "switch" },
      el("input", {
        type: "checkbox", role: "switch", "aria-label": label, checked: Boolean(on),
        onchange: (e) => (instead ? instead(e.target.checked) : saveSetting(change(e.target.checked))),
      }),
      el("span", { class: "track", "aria-hidden": "true" })));
}

// choiceRow is a list to pick from, saved the moment it changes.
function choiceRow(label, options, current, change) {
  return el("select", {
    "aria-label": label,
    onchange: (e) => saveSetting(change(e.target.value)),
  }, options.map(([value, text]) => el("option", { value, selected: value === current || undefined }, text)));
}

// settingGroups is every group, in the order the panel shows them. A
// function rather than a table so that groups defined in scripts loaded
// after this one are there when it's called (v0.8.8, decided by the
// maintainer: six groups and an index along the top).
function settingGroups() {
  return [FOLDER_SETTINGS, UPDATE_SETTINGS, NETWORK_SETTINGS, LIST_SETTINGS, LOOK_SETTINGS, SELF_SETTINGS];
}

const LIST_SETTINGS = {
  title: "The list of images",
  short: "Images",
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
      {
        name: "Allow downloads",
        fields: ["allow_downloads"],
        hint: "A Download button on each image, to save it onto the computer you are using. Off unless you turn it on: on a network, everyone who can sign in could then take every image.",
        words: "download save button browser computer laptop",
        control: () => switchRow("Allow downloads", state && state.allow_downloads,
          (on) => ({ allow_downloads: on })),
      },
  ],
};

const LOOK_SETTINGS = {
  title: "How it looks",
  short: "Look",
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
        name: "Status colours",
        fields: ["colours"],
        hint: "Colours that stay apart for the common kinds of colour blindness, instead of green and red.",
        words: "accessibility colour color blind blindness colorblind red green protanopia deuteranopia tritanopia blue yellow status",
        control: () => choiceRow("Status colours", [
          ["", "Standard"],
          ["colorblind", "Red-green colour blindness"],
          ["tritanopia", "Blue-yellow colour blindness"],
        ], look().colours || "", (colours) => ({ colours })),
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
};

// catalogNote says where the list of images came from and how it's doing, in
// the same words the footer uses.
function catalogNote() {
  const cat = (state && state.catalog) || {};
  return cat.error || cat.note || "";
}

function projectURL() {
  return state.report_url.replace(/\/issues\/new$/, "");
}
