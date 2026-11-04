"use strict";

// The small menus: the "…" links menu on a row, how they open, close and sit
// on the window, and the link that reports a project's moved download.

// problemLink reports an image whose source has moved or broken. Projects
// rearrange their download folders without warning, and the person who hits
// it is the one who can say what happened.
function problemLink(item) {
  if (!state.report_url || !item.entry) return null;
  const title = `Catalog: ${item.name} ${item.status === "check failed" ? "can't be checked" : "needs fixing"}`;
  const body = [
    `**Image:** ${item.name} (\`${item.entry}\`)`,
    `**isoshelf:** ${state.version}`,
    item.version ? `**Version here:** ${item.version}` : null,
    item.latest ? `**Newest isoshelf found:** ${item.latest}` : null,
    `**Status:** ${item.status}`,
    item.note ? `**What it said:** ${item.note}` : null,
    "",
    "What is wrong? For a download that moved, the new address helps most — especially",
    "where the project publishes its checksums now.",
  ].filter((line) => line !== null).join("\n");
  return ["Report a problem", `${state.report_url}?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}&labels=catalog`];
}

// linksMenu holds the project pages, out of the way until asked for.
function linksMenu(item) {
  const links = [
    ["Download page", item.page],
    ["Website", item.site],
    ["Forum", item.forum],
    ["Release notes", item.release],
    problemLink(item),
  ].filter((link) => link && link[1]);
  if (!links.length) return null;

  const menu = el("details", { class: "menu" },
    el("summary", { title: "Links", "aria-label": `Links for ${item.name}` }, "\u2026"),
    el("div", { class: "menu-items" },
      links.map(([label, url]) => el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, label))));
  wireMenu(menu);
  return menu;
}

// wireMenu makes a menu behave: only one open at a time, and pinned to the
// window where there is room for it. Every details.menu needs this, the
// Filter menu included - it was left out, so on a full drive it hung off the
// bottom of the window with the last filters out of reach.
function wireMenu(menu) {
  menu.addEventListener("toggle", () => {
    if (!menu.open) return;
    for (const other of document.querySelectorAll("details.menu[open]")) {
      if (other !== menu) other.open = false;
    }
    placeMenu(menu);
  });
}

// placeMenu pins an open menu to the window, so the table's scroll box can't
// cut it off, and opens it upwards when there's no room below.
function placeMenu(menu) {
  const items = menu.querySelector(".menu-items");
  const anchor = menu.querySelector("summary").getBoundingClientRect();
  const floor = innerHeight - ($("dock").hidden ? 8 : $("dock").offsetHeight + 8);
  items.classList.add("pinned");
  items.style.right = `${Math.max(8, innerWidth - anchor.right)}px`;
  // On a phone a menu with a lot of filters in it is wider than the window.
  // Only the right edge was ever held inside; the left one went off the side
  // of the screen with the first filters out of reach.
  items.style.maxWidth = `${innerWidth - 16}px`;
  // Measure without the limits a previous opening may have left behind.
  items.style.maxHeight = "";
  const wanted = items.offsetHeight;
  const spread = items.getBoundingClientRect();
  if (spread.left < 8) {
    items.style.right = `${Math.max(8, innerWidth - spread.width - 8)}px`;
  }
  const below = anchor.bottom + 4;
  const roomBelow = floor - below;
  const roomAbove = anchor.top - 12;
  if (wanted <= roomBelow) {
    items.style.top = `${below}px`;
  } else if (wanted <= roomAbove) {
    items.style.top = `${anchor.top - wanted - 4}px`;
  } else {
    // A long list of filters on a short window fits neither way. Take the
    // roomier side and let the menu scroll, so the last filter is still
    // reachable instead of hanging off the screen.
    const under = roomBelow >= roomAbove;
    items.style.top = under ? `${below}px` : "8px";
    items.style.maxHeight = `${Math.max(140, under ? roomBelow : roomAbove)}px`;
  }
}

function closeMenus() {
  for (const open of document.querySelectorAll("details.menu[open]")) open.open = false;
}
