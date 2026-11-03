"use strict";

// The style guide's own few lines (a page that is not part of the app, so it
// is not in index.html): the theme switch, and the table of tokens. The table
// is read from css/tokens.css itself, so a token added there appears here
// without anyone remembering to list it.

const root = document.documentElement;

function applyGuideTheme() {
  const [mode, light, dark] = document.getElementById("g-theme").value.split("|");
  root.setAttribute("data-color-mode", mode);
  root.setAttribute("data-light-theme", light);
  root.setAttribute("data-dark-theme", dark);
  if (document.getElementById("g-large").checked) root.setAttribute("data-text", "large");
  else root.removeAttribute("data-text");
  if (document.getElementById("g-contrast").checked) root.setAttribute("data-contrast", "high");
  else root.removeAttribute("data-contrast");
  // Custom properties resolve after the style recalculation, not before.
  setTimeout(fillSwatches, 60);
}

// tokenNames reads the names tokens.css defines on :root.
async function tokenNames() {
  const css = await (await fetch("/static/css/tokens.css")).text();
  const body = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const names = [];
  for (const m of body.matchAll(/(--[A-Za-z0-9_-]+)\s*:/g)) {
    if (!names.includes(m[1])) names.push(m[1]);
  }
  return names;
}

let names = [];

function fillSwatches() {
  const box = document.getElementById("g-swatches");
  const style = getComputedStyle(root);
  const cells = names.map((name) => {
    const value = style.getPropertyValue(name).trim();
    const cell = document.createElement("div");
    cell.className = "swatch";
    const chip = document.createElement("span");
    chip.className = "chipc";
    if (name.startsWith("--shadow")) {
      chip.style.boxShadow = `var(${name})`;
    } else if (name.startsWith("--radius")) {
      chip.style.borderRadius = `var(${name})`;
      chip.style.background = "var(--surface-2)";
    } else if (name.startsWith("--text-") || name === "--leading" || name === "--font" || name === "--mono") {
      chip.style.border = "0";
      chip.style.display = "grid";
      chip.style.placeItems = "center";
      chip.textContent = "Aa";
      if (name.startsWith("--text-")) chip.style.fontSize = `var(${name})`;
      if (name === "--font") chip.style.fontFamily = "var(--font)";
      if (name === "--mono") chip.style.fontFamily = "var(--mono)";
    } else if (name.endsWith("-fg") || name === "--star" || name === "--text" || name === "--muted") {
      chip.style.display = "grid";
      chip.style.placeItems = "center";
      chip.style.background = "var(--surface)";
      chip.style.color = `var(${name})`;
      chip.style.fontWeight = "600";
      chip.textContent = "Aa";
    } else {
      chip.style.background = `var(${name})`;
    }
    const label = document.createElement("div");
    const code = document.createElement("code");
    code.textContent = name;
    const val = document.createElement("span");
    val.className = "val";
    val.textContent = value.length > 34 ? value.slice(0, 33) + "…" : value;
    label.append(code, val);
    cell.append(chip, label);
    return cell;
  });
  box.replaceChildren(...cells);
}

document.addEventListener("DOMContentLoaded", async () => {
  // A tile's colour and logo are set here, the way the app's own scripts do it,
  // because the page's policy refuses a style attribute in the markup.
  for (const tile of document.querySelectorAll("[data-brand]")) {
    tile.style.setProperty("--brand", tile.dataset.brand);
    if (tile.dataset.logo) tile.style.setProperty("--logo", `url("${tile.dataset.logo}")`);
  }
  for (const id of ["g-theme", "g-large", "g-contrast"]) {
    document.getElementById(id).addEventListener("change", applyGuideTheme);
  }
  try {
    names = await tokenNames();
  } catch (err) {
    document.getElementById("g-swatches").textContent = `Couldn't read tokens.css: ${err.message}`;
    return;
  }
  fillSwatches();
});
