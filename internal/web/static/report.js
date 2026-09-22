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
  body.append(
    el("p", { class: "muted" },
      "This opens GitHub's bug form in a new tab with the boxes below already " +
      "filled in. Nothing is sent until you press submit there, and you can " +
      "change or delete any of it first."),
    reportLine("What happened", what || "(describe it in your own words)"),
    detail ? reportLine("What isoshelf said", detail) : null,
    reportLine("isoshelf", reportFacts.version || "unknown"),
    reportLine("Kind of folder", reportFacts.folder || "(none open)"),
  );

  const systemLine = reportLine("System", reportFacts.system);
  const toggle = el("label", { class: "check" },
    el("input", {
      type: "checkbox", checked: includeSystem,
      onchange: (e) => {
        includeSystem = e.target.checked;
        systemLine.hidden = !includeSystem;
      },
    }),
    " Include which system I'm on");
  body.append(toggle, systemLine);
  systemLine.hidden = !includeSystem;
  body.append(el("p", { class: "muted" },
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

function reportLine(label, value) {
  return el("div", { class: "report-line" },
    el("span", { class: "report-label" }, label),
    el("span", { class: "report-value" }, value));
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
