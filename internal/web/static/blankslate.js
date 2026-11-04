"use strict";

// Primer's Blankslate: what to say where there is nothing to show yet. A
// title that names the empty place, a line that says why or what to do, and
// at most one thing to press (a second would be a link, never a second button).

function blankslate(title, text, action) {
  return el("div", { class: "blankslate" },
    el("h2", { class: "blankslate-title" }, title),
    text ? el("p", {}, text) : null,
    action ? el("div", { class: "blankslate-actions" }, action) : null);
}
