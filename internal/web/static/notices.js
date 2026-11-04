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
    box = el("div", { class: "notice dialog-notice", role: "status" });
    (dialog.querySelector("form") || dialog).prepend(box);
    dialog.addEventListener("close", () => box.remove(), { once: true });
  }
  return box;
}

function showNotice(message, isError) {
  const dialog = topDialog();
  // The page's own notice is always set, so the message is still there after
  // a dialog closes. What changes is whether anyone can see it right now.
  fillNotice($("notice"), message, isError);
  if (dialog) fillNotice(noticeIn(dialog), message, isError);
}

function fillNotice(notice, message, isError) {
  notice.replaceChildren(message);
  notice.classList.toggle("error", Boolean(isError));
  notice.hidden = false;
  // Something went wrong is exactly the moment to offer a way to say so.
  if (isError) {
    notice.append(" ", el("button", {
      type: "button", class: "linkish",
      onclick: () => reportProblem("Something went wrong in isoshelf", message),
    }, "Report this"));
  }
}
