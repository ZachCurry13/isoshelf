"use strict";

// Working out what an unrecognized file is: isoshelf's guesses, the whole
// catalog to choose from, or a name of your own. Split out of actions.js in
// v0.7.0.

// ---- What is this file? ----------------------------------------------------

// How sure isoshelf is, in words. A score above 80 rests on evidence: the
// same checksum, or the same file under another name.
const SURENESS = [[95, "Almost certain"], [80, "Very likely"], [60, "Likely"], [45, "Possible"], [0, "A guess"]];

function sureness(score) {
  for (const [least, label] of SURENESS) {
    if (score >= least) return label;
  }
  return "A guess";
}

// identifyPath is the file the dialog is about, so a slow answer for one file
// never lands in the dialog for another.
let identifyPath = null;
let identifyItem = null;

// suggestedName turns a filename into a first guess at a title, so the user
// edits rather than types: "WinServer_2022_x64.iso" -> "WinServer 2022".
function suggestedName(path) {
  const file = path.split("/").pop().replace(/\.[a-z0-9]{1,4}$/i, "");
  return file
    .replace(/[_.+]+/g, " ")
    .replace(/\b(x86[-_]?64|amd64|x64|i386|i686|arm64|aarch64|iso|img)\b/gi, "")
    .replace(/\s{2,}/g, " ")
    .trim()
    .slice(0, 80);
}

// renderReportLink offers to tell the project about an image its catalog is
// missing. It opens a prefilled report the user reads and sends themselves;
// isoshelf sends nothing on its own.
function renderReportLink(item, label) {
  const box = $("identify-report");
  box.replaceChildren();
  if (!state.report_url) return;
  const title = `Catalog: ${item.path.split("/").pop()}`;
  const body = [
    "An image isoshelf didn't recognize.",
    "",
    `- File: ${item.path.split("/").pop()}`,
    `- Size: ${formatBytes(item.size)}`,
    item.kind ? `- Content: ${item.kind}` : null,
    label ? `- The disc calls itself: ` : null,
    "",
    "What is it, and where is it published?",
  ].filter(Boolean).join("\n");
  const url = `${state.report_url}?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}&labels=catalog`;

  box.append(
    "Should isoshelf know this image? ",
    el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, "Suggest it for the catalog"),
    " — this opens a report you can read and edit before sending. isoshelf sends nothing by itself.");
}

// saveMyName writes the user's own name for a file into their own catalog.
async function saveMyName() {
  const name = $("mine-name").value.trim();
  if (!name) {
    showNotice("Give the image a name first.", true);
    return;
  }
  let result;
  try {
    result = await api("POST", "/api/catalog/mine", {
      path: identifyPath,
      name,
      arch: $("mine-arch").value,
      category: $("mine-category").value,
      page: $("mine-page").value.trim(),
    });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  $("identify").close();
  showNotice(result.message);
  catalog = null;
  await refresh();
}

async function openIdentify(item) {
  identifyPath = item.path;
  identifyItem = item;
  $("identify-file").textContent = item.path;
  $("identify-forget").hidden = !item.assigned;
  $("identify-search").value = "";
  $("identify-list").replaceChildren();
  $("identify-all").open = false;
  $("identify-mine").open = false;
  $("mine-name").value = suggestedName(item.path);
  $("mine-page").value = "";
  $("identify-report").replaceChildren();
  $("identify-guesses").replaceChildren(el("p", { class: "muted" }, "Looking at what the file says about itself…"));
  $("identify").showModal();

  let data;
  try {
    data = await api("GET", `/api/guesses?path=${encodeURIComponent(item.path)}`);
  } catch (err) {
    $("identify-guesses").replaceChildren(el("p", { class: "muted" }, err.message));
    return;
  }
  if (data.path !== identifyPath) return;
  renderGuesses(data.guesses);
  renderIdentifyCatalog();
  renderReportLink(item, data.label);
}

function renderGuesses(guesses) {
  const box = $("identify-guesses");
  box.replaceChildren();
  if (!guesses.length) {
    box.append(el("p", { class: "muted" },
      "isoshelf can't identify this one: nothing in the catalog looks like it, and there is no other copy of it in this folder. You can say what it is yourself."));
    $("identify-all").open = true;
    return;
  }
  box.append(el("p", { class: "muted" },
    guesses.length === 1 ? "This is probably:" : "This is probably one of these:"));

  const list = el("ul", { class: "guesses" });
  for (const guess of guesses) {
    list.append(el("li", {},
      logoTile(guess),
      el("div", { class: "info" },
        el("div", {},
          el("span", { class: "name" }, guess.name),
          archBadge(guess.arch, true),
          guess.version ? el("span", { class: "arch" }, guess.version) : null,
          el("span", { class: `pill ${guess.sure ? "s-ok" : "s-muted"}` }, sureness(guess.score))),
        el("div", { class: "kind" }, `Because ${guess.reason}.`)),
      el("button", {
        type: "button", class: "btn small primary",
        onclick: () => confirmIdentity(guess.entry, guess.version),
      }, "Confirm")));
  }
  box.append(list);
}

// renderIdentifyCatalog lists the catalog, so a file isoshelf can't place can
// still be identified by hand.
async function renderIdentifyCatalog() {
  if (!catalog) {
    try {
      catalog = (await api("GET", "/api/catalog")).entries;
    } catch {
      return;
    }
  }
  const query = $("identify-search").value.trim().toLowerCase();
  const list = $("identify-list");
  list.replaceChildren();
  let shown = 0;
  for (const entry of catalog) {
    if (query && !`${entry.name} ${entry.id} ${entry.family || ""}`.toLowerCase().includes(query)) continue;
    if (++shown > 40) {
      list.append(el("li", { class: "muted more-hint" }, "More images match. Keep typing to narrow the list."));
      break;
    }
    list.append(el("li", {},
      logoTile(entry),
      el("div", { class: "info" },
        el("div", {}, el("span", { class: "name" }, entry.name), archBadge(entry.arch, true)),
        el("div", { class: "kind" }, UPDATES_LABEL[entry.updates] || entry.updates)),
      el("button", {
        type: "button", class: "btn small",
        onclick: () => confirmIdentity(entry.id, ""),
      }, "Select")));
  }
  if (!shown) list.append(el("li", { class: "muted more-hint" }, "Nothing in the catalog matches that."));
}

// confirmIdentity records the user's answer. An empty entry forgets an
// earlier one. Nothing on disk is renamed or moved.
async function confirmIdentity(entry, version) {
  let result;
  try {
    result = await api("POST", "/api/identify", { path: identifyPath, entry, version: version || "" });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  $("identify").close();
  showNotice(result.message);
  // The file's version is known now but not what the newest one is, so pick
  // the online check up again if it had already run.
  if (result.recheck) {
    await start("check");
  } else {
    await refresh();
  }
}
