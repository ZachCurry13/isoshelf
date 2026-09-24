"use strict";

// Dismissing an update (v0.8.2, #56): for 7, 30 or 90 days, or for good. The
// row stays in the list, greyed and sorted with the up-to-date ones, and is
// left out of the updates count, Update all and updating by itself. The
// date holds whatever comes out meanwhile; somebody chose a date, not a
// version. Settings lists everything dismissed, with Undo.

// dismissal is the image's dismissal in force now, or null.
function dismissal(entry) {
  const t = (entry && state.tracks && state.tracks[entry]) || {};
  if (t.dismissed_forever) return { forever: true };
  if (t.dismissed_until && Date.parse(t.dismissed_until) > Date.now()) return { until: t.dismissed_until };
  return null;
}

// isDismissed is whether this row's update is dismissed.
function isDismissed(item) {
  return item.status === "update available" && Boolean(dismissal(item.entry));
}

function dismissedWords(item) {
  const d = dismissal(item.entry);
  return d.forever ? "Dismissed for good" : `Dismissed until ${untilDate(d.until)}`;
}

// dismissField is the details panel's control: dismiss, or undo.
function dismissField(item) {
  const d = dismissal(item.entry);
  if (d) {
    return el("div", {},
      el("div", {}, d.forever
        ? (item.updates === "download"
          ? "Dismissed for good: Update all and updating by itself leave this image alone."
          : "Dismissed for good.")
        : `Dismissed until ${untilDate(d.until)}. It comes back then, whatever version is out.`),
      el("button", { type: "button", class: "btn small", onclick: () => setDismiss(item.entry, "") }, "Undo"));
  }
  const pick = el("select", { "aria-label": `How long to dismiss this update to ${item.name}` },
    [["7", "For 7 days"], ["30", "For 30 days"], ["90", "For 90 days"], ["forever", "For good"]]
      .map(([value, text]) => el("option", { value }, text)));
  return el("div", {},
    el("div", { class: "version-set" }, pick,
      el("button", { type: "button", class: "btn small", onclick: () => setDismiss(item.entry, pick.value) }, "Dismiss")),
    el("div", { class: "muted" }, "Stop showing this update. The image stays in the list, greyed, and isn't counted."));
}

async function setDismiss(entry, dismiss) {
  try {
    await api("POST", "/api/track", { entry, dismiss });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  await refresh();
}

// dismissedEntries is every image with a dismissal in force, for Settings.
function dismissedEntries() {
  if (!state || !state.tracks) return [];
  return Object.keys(state.tracks).filter((entry) => dismissal(entry)).sort();
}

function imageName(entry) {
  const item = state.report && state.report.items.find((it) => it.entry === entry);
  const known = item || (catalog && catalog.find((e) => e.id === entry));
  return known ? known.name : entry;
}

// dismissedControl is the Settings row: each dismissed image with Undo, and
// Undo all.
function dismissedControl() {
  const entries = dismissedEntries();
  return el("div", { class: "setting-controls" },
    el("ul", { class: "dismissed-list" }, entries.map((entry) => {
      const d = dismissal(entry);
      return el("li", {},
        el("span", {}, imageName(entry)),
        el("span", { class: "muted" }, d.forever ? " · for good" : ` · until ${untilDate(d.until)}`),
        el("button", { type: "button", class: "linkish", onclick: () => setDismiss(entry, "") }, "Undo"));
    })),
    entries.length > 1
      ? el("button", {
          type: "button", class: "btn small",
          onclick: async () => {
            for (const entry of entries) await setDismiss(entry, "");
          },
        }, "Undo all")
      : null);
}

// untilDate is a day still to come, as "24 Oct" (with the year when it isn't
// this one). shortDate is for days gone by, and called a month away "today".
function untilDate(iso) {
  const then = new Date(iso);
  const year = then.getFullYear() === new Date().getFullYear() ? undefined : "numeric";
  return then.toLocaleDateString(undefined, { day: "numeric", month: "short", year });
}
