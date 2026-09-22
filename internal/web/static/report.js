"use strict";

// Reporting a problem.
//
// Nothing is ever sent from here. Report a problem opens GitHub's own bug
// form in a new tab with the details already typed in, so the report is read,
// edited and sent by the person whose problem it is. Closing the tab sends
// nothing.
//
// The dialog below shows every line that will be in the form before the tab
// opens, and lets the system details be left out. What is never included at
// all: the folder's path, any file name, or anything else that could carry
// somebody's name. A bug report is a public page.

let reportFacts = null;
let includeSystem = true;

// reportProblem shows what would be sent, then opens the form.
// what is a one-line description; detail is the message isoshelf showed.
async function reportProblem(what, detail) {
  if (!reportFacts) {
    try {
      reportFacts = await api("GET", "/api/report");
    } catch (err) {
      showNotice(`Couldn't gather the details for a report: ${err.message}`, true);
      return;
    }
  }

  // Built fresh each time rather than living in the page: it only exists
  // while the dialog is open.
  const body = el("div", { class: "report" });

  // One box holding exactly what would be sent. It is what the Copy button
  // copies and what you would paste, so there is nothing shown here that
  // isn't in it and nothing in it that isn't shown.
  const box = el("textarea", {
    class: "report-text", readonly: true, rows: 10, spellcheck: "false",
    "aria-label": "The details of this report",
  });
  const fill = () => { box.value = reportText(what, detail); };
  fill();

  const said = el("div", { class: "report-said muted" });
  const copy = el("button", {
    type: "button", class: "btn small",
    onclick: async () => {
      said.textContent = (await copyText(box))
        ? "Copied. Paste it into the box on GitHub."
        : "Couldn't copy it here. Select the text above and copy it yourself.";
    },
  }, "Copy the details");

  const toggle = el("label", { class: "check" },
    el("input", {
      type: "checkbox", checked: includeSystem,
      onchange: (e) => { includeSystem = e.target.checked; fill(); },
    }),
    " Include which system I'm on");

  body.append(
    el("p", { class: "muted" },
      "This opens GitHub's bug form in a new tab with the boxes below already " +
      "filled in. Nothing is sent until you press submit there, and you can " +
      "change or delete any of it first."),
    box,
    el("div", { class: "setting-controls" }, copy, toggle),
    said,
    el("p", { class: "muted" },
      "If the form opens empty - GitHub's phone app does that, because it " +
      "ignores anything filled in from a link - copy the details first and " +
      "paste them in."),
    el("p", { class: "muted" },
      "Your folder's location and the names of your files are never included."));

  const go = await ask(
    "Report a problem",
    "",
    [
      { label: "Open the bug form", value: "go", primary: true },
      { label: "Cancel", value: "" },
    ],
    body);
  if (go) window.open(reportURL(what, detail), "_blank", "noopener,noreferrer");
}

// reportText is the whole report as plain words: what goes in the box, on
// the clipboard, and into GitHub's form. One thing to keep true instead of
// three that drift apart.
function reportText(what, detail) {
  const lines = [`What happened: ${what || "(describe it in your own words)"}`];
  if (detail) lines.push("", "What isoshelf said:", detail);
  lines.push("", `isoshelf: ${reportFacts.version || "unknown"}`);
  if (reportFacts.folder) lines.push(`Kind of folder: ${reportFacts.folder}`);
  if (includeSystem && reportFacts.system) lines.push(`System: ${reportFacts.system}`);
  return lines.join("\n");
}

// copyText copies the box's contents. navigator.clipboard only exists on
// https and on localhost, and isoshelf on a NAS is neither - so the old way
// of doing it is the one that works there, and is tried second rather than
// not at all.
async function copyText(box) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(box.value);
      return true;
    }
  } catch {
    // Refused, or no permission. Fall through and try the other way.
  }
  try {
    box.focus();
    box.select();
    box.setSelectionRange(0, box.value.length);
    return document.execCommand("copy");
  } catch {
    return false;
  }
}

// reportURL fills GitHub's bug form by its own field names. The form is
// .github/ISSUE_TEMPLATE/bug.yml; its ids are what these must match.
function reportURL(what, detail) {
  const said = [detail, includeSystem ? reportFacts.system : null]
    .filter(Boolean).join("\n\n");
  const fields = {
    template: reportFacts.template,
    title: what ? `${what}` : "",
    what: what || "",
    version: reportFacts.version || "",
    os: reportFacts.os || "",
    folder: reportFacts.folder || "",
    messages: said,
  };
  const query = Object.entries(fields)
    .filter(([, v]) => v)
    .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
    .join("&");
  return `${reportFacts.url}?${query}`;
}
