"use strict";

// The moving parts of the downloads: the summary on the bar, a row for each
// download while more than one runs, and the buttons that show progress.

// renderDockProgress updates the moving parts of the downloads: the summary
// on the bar, the running download's line, and the buttons that show it.
function renderDockProgress() {
  const d = downloads();
  if ($("dock").hidden) return;
  const many = d.running.length > 1;
  const first = d.running[0];
  const running = first ? downloadProgress(first) : null;
  const count = (outcome) => d.finished.filter((job) => job.outcome === outcome).length;
  let text;
  if (many) {
    const total = d.running.reduce((sum, run) => sum + downloadRate(run), 0);
    text = `${d.running.length} at once`;
    if (total > 0) text += ` · ${formatBytes(total)}/s`;
    if (state.download_limit > 0) text += ` · limit ${state.download_limit} Mbit/s`;
  } else if (first) {
    text = `${first.update ? "Updating" : "Adding"} ${first.name} — ${running.text}`;
  } else if (d.queued.length) {
    text = "Starting the next one…";
  } else {
    const parts = [
      count("done") && `${count("done")} done`,
      count("failed") && `${count("failed")} failed`,
      count("stopped") && `${count("stopped")} stopped`,
    ].filter(Boolean);
    text = `Finished: ${parts.join(", ")}.`;
    if (scanning()) text += " Looking at the folder again…";
  }
  $("dock-text").textContent = text;
  // With thirty images queued, "30 waiting" doesn't answer the question
  // anyone actually has, which is how long and how much room.
  const left = pendingBytes();
  $("dock-waiting").textContent = d.queued.length
    ? `${d.queued.length} waiting${left ? ` · ${formatBytes(left)} to download` : ""}`
    : "";

  // The bar at the bottom: one download's own, or the whole lot's together.
  const bar = $("dock-bar");
  bar.hidden = !first || many;
  if (running && !many) {
    bar.classList.toggle("indeterminate", running.fraction === null);
    $("dock-fill").style.width = running.fraction === null ? "" : `${Math.round(running.fraction * 100)}%`;
  }
  renderRunRows(d.running, many);
  for (const [id, going] of dockRuns) {
    const run = d.running.find((job) => job.id === id);
    if (!run) continue;
    const p = downloadProgress(run);
    going.text.textContent = p.text;
    going.fill.style.width = p.fraction === null ? "0" : `${Math.round(p.fraction * 100)}%`;
  }
  for (const button of document.querySelectorAll("[data-downloading]")) {
    const run = d.running.find((job) => job.entry === button.dataset.downloading);
    button.textContent = run ? downloadProgress(run).short : "Starting";
  }
  fitDock();
}

// renderRunRows keeps a row on the bar for each download while more than one
// is running: its name, how far it is, and its own Stop. The rows are only
// rebuilt when the set of downloads changes, so a click isn't lost to a redraw.
function renderRunRows(running, many) {
  const list = $("dock-runs");
  list.hidden = !many;
  const key = many ? running.map((run) => run.id).join(",") : "";
  if (key !== barRows.key) {
    barRows = { key, rows: new Map() };
    list.replaceChildren();
    if (many) {
      for (const run of running) {
        const row = {
          text: el("span", { class: "run-text" }),
          fill: el("div", { class: "bar-fill" }),
        };
        barRows.rows.set(run.id, row);
        list.append(el("li", { class: "dock-run" },
          el("span", { class: "run-name" }, run.name),
          row.text,
          el("div", { class: "bar" }, row.fill),
          el("button", {
            type: "button", class: "btn small",
            title: "Stop this one. What has downloaded so far is kept.",
            "aria-label": `Stop downloading ${run.name}`,
            onclick: () => dropJob(run),
          }, "Stop")));
      }
    }
  }
  for (const run of running) {
    const row = barRows.rows.get(run.id);
    if (!row) continue;
    const p = downloadProgress(run);
    row.text.textContent = p.text;
    row.fill.style.width = p.fraction === null ? "0" : `${Math.round(p.fraction * 100)}%`;
  }
}


// fitDock tells the stylesheet how tall the bar is, so the end of the page can
// always be scrolled clear of it however many downloads have a row.
function fitDock() {
  const dock = $("dock");
  document.documentElement.style.setProperty("--dock-height", `${dock.hidden ? 0 : dock.offsetHeight}px`);
}

window.addEventListener("resize", fitDock);
