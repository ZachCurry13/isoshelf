"use strict";

// The Updates group of Settings: checking, updating by itself, what happens to
// the old file, the speed limit and the archive. The table it is part of is in
// settinglist.js.

const UPDATE_SETTINGS = {
  title: "Updates",
  short: "Updates",
  settings: [
      {
        name: "Check automatically",
        fields: ["auto_check"],
        hint: "Asks each project for its newest version when the page opens, and " +
          "remembers the answer for a day.",
        words: "automatic online check for updates network offline internet",
        control: () => switchRow("Check automatically", state && state.auto_check,
          (on) => ({ auto_check: on })),
        note: () => (state && state.checked_at)
          ? `Last asked ${timeAgo(state.checked_at)}.`
          : "",
      },
      {
        name: "Update images automatically",
        fields: ["auto_update", "auto_update_every"],
        hint: "Downloads and installs updates on a schedule, with nothing to press. " +
          "Verified the same way as when you press Update yourself.",
        words: "automatic update download schedule unattended daily weekly",
        control: () => el("div", { class: "setting-controls" },
          switchRow("Update images automatically", state && state.auto_update,
            (on) => ({ auto_update: on })),
          (state && state.auto_update)
            ? choiceRow("How often", [["day", "Every day"], ["week", "Every week"]],
              (state && state.auto_update_every) || "day", (every) => ({ auto_update_every: every }))
            : null),
        note: () => (state && state.auto_update)
          ? "Changes your folder while you aren't watching. Old files go the way the setting " +
            "below says, a pinned file stays where it is, nothing is deleted that you hadn't " +
            "already chosen to lose, and it stops before the folder is full."
          : "",
      },
      {
        name: "Replaced files",
        fields: ["old_files"],
        hint: "For every image. To keep one exact file whatever happens, pin it " +
          "in its own panel.",
        words: "what happens to the file an update replaces replace archive delete keep both old copies updates pin pinned",
        control: () => choiceRow("Replaced files", [
          ["replace", "Replace it"],
          ["archive", "Move it to the archive"],
          ["keep", "Keep both"],
        ], (state && state.old_files) || "replace", (old_files) => ({ old_files })),
      },
      {
        name: "Download speed limit",
        fields: ["download_limit"],
        hint: "The most of your internet connection isoshelf may use. Choose No limit " +
          "to use all you have; with a limit it also downloads more than one image at once when there is room.",
        words: "download speed limit bandwidth megabits mbit throttle slow internet parallel several at once",
        control: () => {
          // A pick-one list, so it saves the way every other setting here does.
          // A value set some other way (an older copy, the config file) is
          // kept in the list rather than quietly replaced.
          const now = (state && state.download_limit) || 0;
          const speeds = [10, 25, 50, 100, 200, 300, 500, 1000];
          if (now && !speeds.includes(now)) speeds.push(now);
          speeds.sort((a, b) => a - b);
          return choiceRow("Download speed limit",
            [["0", "No limit"], ...speeds.map((n) => [String(n), `${n.toLocaleString()} Mbit/s`])],
            String(now), (value) => ({ download_limit: Number(value) }));
        },
        note: () => "Only downloads from the internet are limited. Copying from another isoshelf on your network is not.",
      },
      {
        name: "Dismissed updates",
        fields: ["dismissed"],
        hint: "Updates you asked not to be shown, for a while or for good. Undo brings one back.",
        words: "dismiss dismissed hide ignore skip never update snooze undo",
        available: () => dismissedEntries().length > 0,
        control: () => dismissedControl(),
      },
      {
        name: "Empty archive automatically",
        fields: ["archive_after"],
        hint: "Delete archived files once they have waited this long. Off unless " +
          "you choose a number: the archive is how a removal is undone.",
        words: "archive empty the archive by itself delete timer days automatic old removed space free",
        control: () => choiceRow("Empty archive automatically", [
          [0, "Never"],
          [7, "After 7 days"],
          [30, "After 30 days"],
          [90, "After 90 days"],
        ], (state && state.archive_after) || 0,
          (archive_after) => ({ archive_after: Number(archive_after) })),
        // Said before it happens, not after: this is the one thing isoshelf
        // does by itself that throws something away.
        note: () => {
          if (!state || !state.archive_after) return "";
          if (!state.archive_due) {
            return `Nothing in the archive has waited ${state.archive_after} days yet.`;
          }
          return `${plural(state.archive_due, "file")} in the archive ` +
            `${state.archive_due === 1 ? "has" : "have"} waited longer than ` +
            `${state.archive_after} days and will be deleted, freeing ` +
            `${formatBytes(state.archive_due_bytes)}. Restore anything you want to keep first.`;
        },
      },
  ],
};

