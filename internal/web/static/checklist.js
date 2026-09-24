"use strict";

// The checklist, shared by "update all" and "review older versions": one
// dialog that lists files, lets you untick any, and says how much they come
// to. Split out of details.js in v0.7.0.

function pickFiles({ title, text, rows, actions, sizeLabel }) {
  const dialog = $("pick");
  $("pick-title").textContent = title;
  $("pick-text").textContent = text;
  const list = $("pick-list");
  const total = $("pick-total");
  const boxes = new Map();

  const tally = () => {
    const chosen = rows.filter((row) => boxes.get(row.id).checked);
    const bytes = chosen.reduce((sum, row) => sum + (row.size || 0), 0);
    total.textContent = `${plural(chosen.length, "image")} chosen${bytes ? ` · ${sizeLabel || "about"} ${formatBytes(bytes)}` : ""}`;
  };

  list.replaceChildren(...rows.map((row) => {
    const box = el("input", { type: "checkbox", checked: row.checked !== false || undefined, onchange: tally });
    boxes.set(row.id, box);
    return el("li", {},
      el("label", { class: "pick-item" },
        box,
        el("span", { class: "info" },
          el("span", { class: "name" }, row.name),
          row.detail ? el("span", { class: "kind" }, row.detail) : null),
        row.note ? el("span", { class: "muted pick-note" }, row.note) : null));
  }));
  tally();

  return new Promise((resolve) => {
    $("pick-actions").replaceChildren(...actions.map((action) => el("button", {
      type: "button", class: `btn ${action.primary ? "primary" : ""}`,
      onclick: () => {
        const chosen = rows.filter((row) => boxes.get(row.id).checked).map((row) => row.id);
        dialog.close();
        resolve(chosen.length ? { action: action.value, ids: chosen } : null);
      },
    }, action.label)));
    dialog.addEventListener("close", () => resolve(null), { once: true });
    dialog.showModal();
  });
}

// reviewOlder lists the older versions and clears the ones you tick.
async function reviewOlder(older) {
  const newest = {};
  for (const item of state.report.items) {
    if (item.entry && !item.older && item.path) newest[item.entry] = item.path.split("/").pop();
  }
  const answer = await pickFiles({
    title: "Older versions",
    text: "You already have a newer version of each of these. Untick anything you want to keep.",
    sizeLabel: "freeing",
    rows: older.map((item) => ({
      id: item.path,
      size: item.size,
      name: item.name,
      detail: `${item.path}${item.size ? ` · ${formatBytes(item.size)}` : ""}`,
      note: newest[item.entry] ? `newer here: ${newest[item.entry]}` : "",
    })),
    actions: [
      { label: "Archive them", value: "move-aside", primary: true },
      { label: "Delete them", value: "delete" },
    ],
  });
  if (!answer) return;
  try {
    state = await api("POST", "/api/remove", { paths: answer.ids, how: answer.action });
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  flashNotice(answer.action === "delete"
    ? `${plural(answer.ids.length, "older version")} deleted.`
    : `${plural(answer.ids.length, "older version")} archived, under Archive on this page.`);
  if (scanning()) {
    render();
    return;
  }
  await start("scan");
}
