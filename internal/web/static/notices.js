"use strict";

// The short confirmations and error lines shown in the page or in the dialog
// that is open.

// flash is a short confirmation that outlives the redraws after it, which
// would otherwise hide it at once.
let flash = null;

function flashNotice(message) {
  flash = { message, until: Date.now() + 10000 };
  showNotice(message, false);
  setTimeout(() => { if (flash && Date.now() >= flash.until) { flash = null; render(); } }, 10100);
}

// topDialog is the modal dialog on top, if one is open.
//
// A dialog shown with showModal() is drawn in the browser's top layer, above
// everything else on the page. A message written into the page behind it is
// therefore invisible - which is how a refused action inside the identify
// dialog came to look like a button that did nothing at all. Reported by the
// maintainer as "I clicked That's it and nothing happened", and reproduced:
// the message was there, underneath.
function topDialog() {
  const open = document.querySelectorAll("dialog[open]");
  return open.length ? open[open.length - 1] : null;
}

// noticeIn is the message slot inside a dialog, made the first time one is
// needed and taken away when the dialog closes, so it never reappears stale
// the next time the dialog is opened.
function noticeIn(dialog) {
  let box = dialog.querySelector(".dialog-notice");
  if (!box) {
    box = el("div", { class: "banner dialog-notice" });
    (dialog.querySelector("form") || dialog).prepend(box);
    dialog.addEventListener("close", () => box.remove(), { once: true });
  }
  return box;
}

// A message that was dismissed isn't shown again until it says something else.
let dismissedBanner = "";

function showNotice(message, isError, kind) {
  if (message === dismissedBanner && !isError) return hideNotice();
  const dialog = topDialog();
  // The page's own notice is always set, so the message is still there after
  // a dialog closes. What changes is whether anyone can see it right now.
  fillNotice($("notice"), message, isError, kind);
  if (dialog) fillNotice(noticeIn(dialog), message, isError, kind);
}

function hideNotice() {
  $("notice").hidden = true;
}

// The icon for each severity, as Primer's Banner draws them.
const BANNER_ICON = {
  critical: "stop-16",
  warning: "alert-16",
  success: "check-circle-16",
  info: "info-16",
};

// fillNotice draws a Banner (Primer's component for feedback about the page
// or a section): an icon for how serious it is, a short title that says it,
// and what can be done. An error stays until the next thing happens and has
// no dismiss button, as Primer asks of a critical banner, but offers a way to
// report it; anything else can be dismissed.
function fillNotice(notice, message, isError, kind) {
  const severity = isError ? "critical" : kind || "success";
  notice.classList.add("banner");
  for (const name of Object.keys(BANNER_ICON)) notice.classList.toggle(`banner--${name}`, name === severity);
  notice.setAttribute("role", isError ? "alert" : "status");
  const parts = [icon(BANNER_ICON[severity]), el("div", { class: "banner-title" }, message)];
  if (isError) {
    parts.push(el("button", {
      type: "button", class: "btn small",
      onclick: () => reportProblem("Something went wrong in isoshelf", message),
    }, "Report this"));
  } else {
    parts.push(el("button", {
      type: "button", class: "btn small icon", "aria-label": "Dismiss message", "data-tooltip": "Dismiss", "data-tooltip-end": "",
      onclick: () => {
        dismissedBanner = message;
        flash = null;
        hideNotice();
        for (const box of document.querySelectorAll(".dialog-notice")) box.remove();
      },
    }, icon("x-16")));
  }
  notice.replaceChildren(...parts);
  notice.hidden = false;
}

// fieldError says what is wrong with one field, next to it (Primer's form
// validation): the message under the field with a small alert icon, the field
// marked invalid and described by the message, and focus on the field. A
// message says what is wrong and how to put it right. clearFieldError takes
// it away again as soon as the person edits the field.
function fieldError(input, message) {
  const id = `${input.id}-error`;
  let box = document.getElementById(id);
  if (!box) {
    box = el("p", { class: "field-error", id });
    input.after(box);
    input.addEventListener("input", () => clearFieldError(input), { once: true });
  }
  box.replaceChildren(icon("alert-fill-12"), el("span", {}, message));
  input.setAttribute("aria-invalid", "true");
  input.setAttribute("aria-describedby", id);
  input.focus();
}

function clearFieldError(input) {
  input.removeAttribute("aria-invalid");
  input.removeAttribute("aria-describedby");
  const box = document.getElementById(`${input.id}-error`);
  if (box) box.remove();
}
