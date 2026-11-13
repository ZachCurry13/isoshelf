"use strict";

// Octicons, GitHub's icon set, drawn from the sprite static/octicons.svg
// (docs/design-system.md, "Icons"). An icon is used at the size it is drawn -
// 16 px, or 12 px where the name ends in -12 - and takes the colour of the text
// around it. Which names exist is internal/web/octicons.

const SVG_NS = "http://www.w3.org/2000/svg";

// icon makes one icon. A decorative icon, the usual case, is hidden from
// assistive technology; pass a label only when the icon alone says something,
// and then it is announced as an image with that name.
function icon(name, label) {
  const size = name.endsWith("-12") ? 12 : 16;
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("class", "octicon");
  svg.setAttribute("width", String(size));
  svg.setAttribute("height", String(size));
  svg.setAttribute("viewBox", `0 0 ${size} ${size}`);
  svg.setAttribute("focusable", "false");
  if (label) {
    svg.setAttribute("role", "img");
    svg.setAttribute("aria-label", label);
  } else {
    svg.setAttribute("aria-hidden", "true");
  }
  const use = document.createElementNS(SVG_NS, "use");
  use.setAttribute("href", `/static/octicons.svg#${name}`);
  svg.append(use);
  return svg;
}

// setAvailable says whether a control can be used right now. Primer asks that
// a control that can't be used is not `disabled` (a disabled button can't be
// reached with the keyboard and can't say why), so it is marked
// aria-disabled, stays in the tab order, and says why; a click on it does
// nothing (app.js swallows it). Pass no reason when the control is available.
function setAvailable(node, available, why) {
  if (available) {
    node.removeAttribute("aria-disabled");
    node.removeAttribute("aria-description");
    return;
  }
  node.setAttribute("aria-disabled", "true");
  if (why) node.setAttribute("aria-description", why);
}

// Escape hides a tooltip (css/controls.css) without moving focus or the
// pointer, as Primer's Tooltip must; it comes back once both have left.
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  const focused = document.activeElement && document.activeElement.closest("[data-tooltip]");
  for (const t of [...document.querySelectorAll("[data-tooltip]:hover"), focused]) {
    if (t) t.setAttribute("data-tooltip-off", "");
  }
});
// On blur the pointer must have gone too; on mouseout, the focus.
document.addEventListener("blur", (e) => {
  const t = e.target instanceof Element && e.target.closest("[data-tooltip-off]");
  if (t && !t.matches(":hover")) t.removeAttribute("data-tooltip-off");
}, true);
document.addEventListener("mouseout", (e) => {
  const t = e.target instanceof Element && e.target.closest("[data-tooltip-off]");
  if (t && !t.matches(":hover") && document.activeElement !== t) t.removeAttribute("data-tooltip-off");
}, true);
