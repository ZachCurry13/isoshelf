"use strict";

// A drive's list (#11, #12), in Settings, This folder: save the list of
// images this folder carries, to keep somewhere else, and fill a folder from
// a list - a folder this computer remembers, or a list file - when a drive
// was lost or outgrown. The backup is the list, not the files: everything on
// it comes back from its project's own site, checked as any download is.

const DRIVE_LIST_SETTING = {
  name: "Drive list",
  hint: "This folder's list of images is kept on this computer too, so if the drive is lost, isoshelf can fetch them all again. " +
    "Save the list to keep a copy somewhere else as well, or fill this folder from another folder's list: a new drive, or a bigger one.",
  words: "drive list backup rebuild restore lost wiped reformatted new bigger drive move copy export import fill",
  available: () => state && state.target,
  control: () => driveListControl(),
};

function driveListControl() {
  const others = ((state && state.recent_targets) || []).filter((f) => f.path !== state.target);
  const from = el("select", { "aria-label": "Fill this folder from" },
    others.map((f) => el("option", { value: f.id }, f.path)),
    el("option", { value: "file" }, "A list file…"));
  const file = el("input", { type: "file", accept: ".toml,text/plain", hidden: true, onchange: () => fillFromFile(file) });
  return el("div", { class: "setting-controls" },
    el("a", { class: "btn small", href: "/api/list", download: "" }, "Save list"),
    el("div", { class: "version-set" }, from,
      el("button", {
        type: "button", class: "btn small", disabled: cantSwitch(),
        onclick: () => (from.value === "file" ? file.click() : fillFrom({ folder: from.value })),
      }, "Fill from it")),
    file);
}

async function fillFromFile(input) {
  const f = input.files && input.files[0];
  input.value = "";
  if (!f) return;
  if (f.size > 512 << 10) {
    showNotice(`${f.name} is far bigger than a drive list. Choose the file isoshelf saved for you.`, true);
    return;
  }
  await fillFrom({ text: await f.text() });
}

// fillFrom shows what a list would bring, lets you untick any, and queues
// the rest. What can't be fetched is said, not left out quietly.
async function fillFrom(source) {
  let plan;
  try {
    plan = await api("POST", "/api/fill/plan", source);
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  if (!plan.download.length) {
    flashNotice(plan.here
      ? "This folder already has every image on that list that isoshelf can download."
      : "There's nothing on that list isoshelf can download.");
    return;
  }
  const cannot = plan.cannot.length
    ? ` Left out because isoshelf can't fetch them: ${plan.cannot.map((c) => `${c.name} (${c.why})`).join("; ")}.`
    : "";
  const room = plan.fits ? "" : ` They don't all fit: this folder has ${formatBytes(plan.free)} free, so untick some first.`;
  const answer = await pickFiles({
    title: `Fill this folder from ${plan.from || "the list"}?`,
    text: `Each image is downloaded from its project's own site and checked, one after another.${room}${cannot} ` +
      "Your own files, persistence files and anything else isoshelf doesn't know aren't on any list; for a byte-for-byte copy of a drive, use your own disk tools.",
    sizeLabel: "downloading about",
    rows: plan.download.map((d) => ({ id: d.id, name: d.name, size: d.size })),
    actions: [{ label: "Download images", value: "go", primary: true }],
  });
  if (!answer) return;
  try {
    const got = await api("POST", "/api/fill", { ...source, ids: answer.ids });
    flashNotice(`${plural(got.queued, "image")} added to the downloads.`);
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}
