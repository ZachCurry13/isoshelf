<!--
A design audit of the page, written by the `page` agent on 2026-09-21, before
any of the redesign was built. It is a proposal with options, not a plan that
was agreed: the maintainer asked to be consulted on anything where more than
one answer is good, and the seven decision points in section 3 are what to
ask. Kept as the critique the v0.5.0 redesign started from; the v0.5.0
section of docs/TODO.md is the brief it answered.
-->

# isoshelf web page — design audit

Read: CLAUDE.md, docs/design.md, docs/STATUS.md (decisions list), index.html,
app.css, and all eight scripts. Ran the 93-file sample drive on port 8801 and
captured screenshots at 1280×900 and 390×844, including several faked states
(a scan and a 4-item download queue running together, a stocked Archive and
History, an empty folder) via `page.route` on `/api/state` and `/api/archive`.
Screenshots referenced below were written to a scratch folder for that
session and are not kept in the repository (14 MB of them). To make them
again: `go run ./internal/sampledrive/mkdrive <folder>`, run `isoshelf ui` on
it, and drive it with Playwright - the file names below say what each one
shows.
Ignore that every row says "Couldn't check" (sandbox has no internet) and
that sizes are tiny (stand-in files) — noted as expected, not reported below.

## 1. Critical verdict

**What's actually good, and should survive.** The information design under
the surface is sound and shouldn't be thrown out with a "redo the whole
design." The slim-row idea (file/size/date folded under the name, version as
"22.04 → 24.04", one pill for status) is a genuinely good density trick —
`01-desktop-home.png` shows 70 images without feeling like a spreadsheet. The
to-do cards at the top (`3 updates ready`, `13 older versions`, `3 unknown
files`) are the right idea for a page whose main job is "tell me what needs
doing" rather than "here is everything, go find it." The details panel
keeping the row itself uncluttered, one settings panel that's searchable
rather than scattered across menus, and the phone card layout (`14c-phone-
home-rows.png`) that turns the same data into readable blocks — all of this
is competent, deliberate work, and a redesign that discards it to "look
different" would be a step backward.

**What's wrong.** The page has one register: everything is the same
weight. Open `01-desktop-home.png` full length and there is no visual
hierarchy below the to-do cards — 70 rows of near-identical pills in muted
red ("Couldn't check"), gray, and amber, each with the same 14px bold name,
the same gray meta line, the same right-aligned button. Nothing tells the
eye where to land next. The to-do cards themselves are diluted by being the
same white card, same border, same shadow as everything else on the page —
compare them to the folder card directly above, which is visually identical
in weight despite doing something far less urgent. A page whose whole
premise is "surface what needs doing" undercuts that premise by giving the
things that need doing no more visual authority than the folder path.

Color is doing too much of the differentiation work and too little of it at
once. There are eight status hues (`--ok`, `--update`, `--bad`, `--eol`,
`--warn`, `--missing`, `--manual`, `--muted`) plus brand colors on every logo
tile, plus the accent blue for buttons and links. On the real catalog most
rows are "Couldn't check" (muted) in this environment, but in normal use a
mixed folder shows five or six of those hues on screen simultaneously,
fighting the amber-bordered to-do cards for attention. The caution triangle
(⚠) adds a ninth signal. None of this is wrong per status, but the sum is a
page that never rests the eye — everything is trying to matter at once,
which functionally means nothing does.

Typography has almost no scale. Reading `app.css`, nearly all text sits
between 0.79rem and 1rem; the row name is 1rem/650, the page's biggest text
(`brand-name`) is 1.21rem, and a to-do card's title is the same weight as a
column heading. On a "read this first" screen (the folder card, the to-do
row) there is no size step that says "you are meant to read this before
that." Larger Text in Settings (`17b-phone-settings.png`) raises the root
from 14px to 16px uniformly, which proves the scale is really one number
wearing different `rem` multipliers, not a designed type scale with
distinct roles (a display size, a body size, a caption size, spaced apart
enough that raising the root actually changes the page's *rhythm*, not just
its magnification).

Density is inconsistent between sections rather than deliberately varied.
The image list is admirably tight (9–12px vertical padding per row); the
catalog cards and Archive/History cards (`05-desktop-archive.png`) are
looser grid tiles with more air, for no functional reason — they're not more
important, they're just built differently (a `<ul class="catalog">` grid vs.
a `<table>`). A reader has to re-learn the layout each time they cross from
"Your images" into "Add images." The to-do cards, the filter menu, the
details panel and the pick-list checklist (`09-desktop-update-checklist.png`)
each reinvent "a row with an icon, a name, and a button on the right" with
slightly different spacing and border-radius. There is no shared "list item"
component; there are four hand-built approximations of one.

The download dock (`12-desktop-dock-open.png`) is the page's best piece of
real design — a Steam-style queue with drag reordering, per-item stop/retry,
and a persistent summary bar — but visually it's the least polished: plain
system-font labels, a thin progress bar, icon buttons (↑ ↓ ⤒ ✕) with no
visual grouping, and on phone (`18c-phone-dock-open.png`) those four icon
buttons plus a drag handle are squeezed onto one card at a width that makes
each target well under the ~44px a thumb wants. The feature is good; its
presentation undersells it.

Two menu/positioning bugs turned up while driving the real thing rather than
reading the code, worth naming precisely since a redesign should not
re-introduce them:

- **The filter menu runs off the left edge of a phone screen**
  (`16b-phone-filter-menu.png`). `placeMenu` positions every `details.menu`
  by its distance from the *right* edge of the window (`items.style.right =
  innerWidth - anchor.right`), which is correct for a menu opened from a
  button near the right of a wide screen, but on a 390px-wide phone the
  Filter button sits near the *left*, and the menu (min-width 230px) is
  wider than the space to its right, so it is pinned off-screen to the
  left. `placeMenu` handles vertical overflow (up/down, and scrolling when
  neither fits) carefully — the horizontal case has no equivalent.
- **Escape does not close an open `details.menu`.** The page's own
  `keydown` handler for Escape only knows about Settings, the details
  panel and the download dock; a `details.menu` (Filter, or a row's "…"
  links menu) has to be dismissed by clicking elsewhere. This was visible
  first-hand: closing the filter menu with Escape while capturing
  screenshots did nothing, and it stayed open, floating over the next
  screen captured. Every other overlay in the page responds to Escape;
  this one doesn't.

## 2. Redesign direction

The app's users span someone plugging in a Ventoy stick for the first time
to someone running a NAS full of seventy images who wants a spreadsheet's
worth of control without a spreadsheet's ceremony. The right direction is
not "make it prettier" but **make the page's own hierarchy match how
urgently something matters**, and use restraint everywhere else so that
hierarchy reads clearly.

**Layout.** Keep the one-page-with-jump-bar structure (decision 1) — it's
right for this content, and nothing in the audit argues for tabs or a
sidebar. What should change is *weight*, not structure: the to-do row
becomes visually the loudest thing on the page after the folder identity —
larger card padding, a filled (not just left-bordered) tint per severity,
and the row below it recedes: lighter borders between rows, no border at
all on hover-lifted state instead of full row backgrounds, pills that carry
less saturation for "up to date" (the common case, most of the time) and
more for what needs a decision.

**Type scale.** Move from "one number times a handful of `rem` multipliers"
to four named roles, still in `rem` against the same root variable so
Larger Text keeps working: a **display** size for the folder path and
section titles (proposal: 1.5rem/700), a **title** size for card headings
and row names (1rem/650, roughly where it is now), a **body** size for
everything else (0.9rem), and a **caption** size for meta lines, timestamps,
and hints (0.8rem). The point isn't the exact numbers, it's that raising the
root should widen the *gap* between display and caption, not just scale a
flat page uniformly — that's what makes "larger text" feel like an
accessibility win rather than a zoom.

**Color roles.** Keep `light-dark()` and the single palette (house rule);
narrow how many status hues appear with equal strength at once. Concretely:
collapse "checksum mismatch" and "check failed" visually (both are "this
needs your attention, here's why" in red) rather than reading as different
problems; treat "up to date" as the *absence* of color — plain text, no
pill — so a pill on the page always means "look at this," not "here is this
row's status regardless." Reserve the accent blue for the one primary
action per screen (Update, Use this folder) rather than also using it for
links, chips, focus rings and the badge count all at once.

**Spacing system.** One 4px-based scale (4/8/12/16/24/32) applied
consistently to cards, list items and dialogs, replacing the current mix of
`6px 12px`, `9px 12px`, `10px 12px`, `10px 14px`, `12px 14px` etc. scattered
across `app.css`. This is mechanical but it's exactly what makes the catalog
grid, the archive list, the pick-list and the row table feel like one
page instead of four.

**Component shapes.** Introduce one shared "list row" component (icon/logo,
title + meta stack, trailing action) used by the image table's card mode,
the catalog grid, Archive, History, the pick-list checklist and the dock's
queued items — they are all the same information shape today, built five
separate times. This is the single highest-leverage change: it would also
shrink the CSS.

**Status expression.** Keep plain-word statuses and the hover explanation
(decision 4) — that's good and tested. Reduce the *pill* to the minority of
statuses that need one (update ready, needs a decision, broken) and let
"up to date" and "manual" sit as quiet text, so a glance at a long list
finds problems by their color standing out against a mostly quiet page,
rather than counting eight competing colors.

**Phone width.** The card transformation of the table (`14c-phone-
home-rows.png`) already works well and should be kept structurally. Two
concrete fixes: make `placeMenu` clamp horizontally the way it clamps
vertically (min 8px from either edge, not just anchored to the right), and
give the dock's queued-item icon buttons more room on narrow screens —
either drop to two visible actions (drag to reorder, remove) with "move to
top" folded into a menu, or stack the info above a full-width button row
instead of squeezing four icons beside the text.

## 3. Decision points

These are the real forks — where more than one good answer exists and the
maintainer should pick, not places where the audit found one obviously right
answer.

**1. How loud should the to-do cards be, relative to the list below them?**
Today they're the same visual weight as every other card (`01-desktop-
home.png`).
- *Keep as-is*: consistent, calm, nothing shouts. Costs: the page's stated
  job — surface what needs doing — doesn't actually stand out from "here is
  your folder path."
- *Louder cards, quieter list* (recommended): filled tint per severity on
  the cards, and pull saturation out of "up to date"/"manual" pills in the
  list so the list itself reads as reference material, not a wall of
  equally-urgent color. Cost: more CSS, and it's a visible personality
  change that should be shown to the maintainer before committing.
- *Cards become a single condensed banner* ("4 things need attention →")
  that expands to today's cards on click: maximum calm, minimum space, but
  buries the very thing decision 2 (STATUS.md) chose to put up front, and
  loses the one-glance detail ("about 16 B to download") that makes the
  cards useful without a click.

**2. Do status pills disappear for "up to date" and "manual"?**
Currently every status gets a colored pill, including the common,
unremarkable ones.
- *Keep pills on everything*: consistent, and a pill is always in the same
  place in the row, which is easier to scan by eye position alone.
- *Quiet text for "up to date"/"manual"/"unknown", pill only for what needs
  a decision* (recommended): the color budget goes further and a long list
  reads faster because the eye is drawn only to what's colored. Cost: two
  different visual treatments in the same column, which needs to be done
  carefully so it doesn't look like a mistake or a half-finished pill.

**3. One shared "list row" component, or keep each section's own markup?**
Right now the image table, the catalog grid, Archive, History and the
download queue each have their own row/card CSS.
- *Keep separate*: each section can be shaped exactly for its content (the
  table needs sortable columns; the catalog is a grid of "things I could
  add"). Lower risk per change, but the four places keep drifting apart —
  Archive and History already look subtly different from the catalog grid
  they're styled to resemble.
- *One shared component* (recommended): a single `.list-row` (or similar)
  used everywhere two-line-item-plus-action appears. Cuts the CSS
  materially and guarantees Archive, History, the catalog and the pick-list
  stay in sync automatically. Cost: a real refactor across five files, not
  a coat of paint — this is the one item in this list that's more
  restructuring than redesign, and should be scoped and reviewed as its
  own stage (see plan, stage 2).

**4. Does the main image list stay a `<table>` on desktop, or become cards
everywhere (like phone already is)?**
- *Keep the table* (recommended): column sorting (name/version/status) is a
  real, used feature (`renderHeadings`) that a card layout would have to
  reinvent or drop, and a Proxmox/NAS user with 70+ images benefits from
  scanning a column rather than a stack. The redesign above (quieter
  pills, shared spacing) can happen entirely inside the table.
- *Cards everywhere*: consistent with phone, and easier to make each row
  feel like a "thing" rather than "data," which suits a beginner audience.
  Costs real capability (sortable columns, tighter vertical density) for
  the power-user half of this app's audience — a real tension with the
  guiding principle ("configurable enough that an expert enjoys it").

**5. How many status colors survive?**
There are eight (`--ok`, `--update`, `--bad`, `--eol`, `--warn`,
`--missing`, `--manual`, `--muted`) plus the caution triangle.
- *Keep all eight*: each status is genuinely different and a distinct hue
  is the fastest way to tell them apart once learned; the legend (`what do
  the statuses mean?`) already exists to teach them.
- *Collapse to four families* (recommended): "fine" (quiet/no color),
  "needs a decision" (amber — update ready, older version), "broken" (red —
  mismatch, check failed), "informational" (blue/gray — missing, manual,
  unrecognized, EOL as a modifier rather than its own hue). Fewer colors to
  learn, and it matches how the to-do cards already group things
  functionally. Cost: EOL currently gets its own purple identity as "this
  release is unsupported," which is worth knowing at a glance separately
  from "old release" being a version question — collapsing it into a
  modifier badge (already partly done: the ⚠ mark) loses a little of that
  distinctness. Worth showing both ways before deciding.

**6. Fix the two menu bugs now, as part of this pass, or fold them into the
redesign's component work later?**
- *Fix now, standalone*: both (`placeMenu` horizontal clamping, Escape
  closing `details.menu`) are small, mechanical, and affect real use today
  independent of any redesign decision. Low risk, immediate benefit.
- *Fold into the redesign*: if the shared list-row/menu component (decision
  3) is being rebuilt anyway, fixing positioning twice is wasted work.
  Recommended only if stage 2 of the plan below is starting within the next
  short while; otherwise fix now (see plan, stage 0).

**7. Does "Larger text" get more type-scale steps, or stay one root
number?**
- *Stay as one root number*: simplest, already works, and "larger text"
  doing exactly what it says (scale everything) is honest and predictable.
- *Real type scale with named roles* (recommended, see Redesign direction):
  makes hierarchy stronger at the default size and *more* readable at the
  larger size, since gaps between roles widen rather than staying fixed
  ratios. Cost: touches most of `app.css`'s font-size declarations, so it's
  a wide, low-per-line-risk change best done as its own stage.

## 4. Staged plan

Ordered so each stage ships and is reviewable on its own; nothing here
requires a new release cycle by itself, and only the Go note below touches
non-page code.

0. **Fix the two menu bugs.** `placeMenu` horizontal clamping; Escape
   closes any open `details.menu`. Small, testable, no visual redesign
   attached — worth doing regardless of what else is decided. (`page`
   agent; touches `images.js`.)
1. **Spacing and type-scale pass.** Introduce the 4px spacing scale and
   the four named type roles as CSS custom properties; apply them to the
   existing markup without changing any component's shape. This alone
   should visibly calm the page and is entirely reversible/diffable.
2. **Status color and pill reduction** (decision 2 and 5, whichever way
   they're decided). Changes `STATUS_CLASS`/`STATUS_WORDS` usage in
   `images.js` and the `--*-fg`/`--*-bg` pairs in `app.css`; no HTML
   structure changes.
3. **To-do card weight** (decision 1). Once color roles are settled, give
   the cards their own visual identity relative to the list.
4. **Shared list-row component** (decision 3) — the real refactor. Do this
   after 1–3 land, so the new component is built with the final spacing/
   type/color rules rather than being redone again after. Touches
   `images.js`, `actions.js` (catalog cards), `archive.js`, `details.js`
   (pick-list), `downloads.js` (queued items) and `app.css`. This is the
   stage to budget the most review time for, since it changes five files'
   worth of markup generation at once — plausibly worth its own further
   split (e.g., catalog + archive/history together, then the pick-list and
   dock queue as a second piece) rather than one pull request.
5. **Dock and phone polish**: icon button sizing on narrow screens,
   dock visual treatment to match the rest of the new component language.

Nothing above needs to touch Go. If decision 4 (table vs. cards) goes the
"cards everywhere" way, that's still page-only (`images.js` + `app.css`),
but it's large enough to deserve its own stage between 3 and 4 rather than
folding into the shared-component refactor.

## 5. Risks

- **Accessibility settings must keep working.** All three (`data-contrast`,
  `data-text`, `data-motion`) are driven by `:root` attributes and CSS
  custom properties (`app.css` lines 34–61); any redesign that hard-codes a
  color or a `px` font size anywhere breaks one of them silently. The type
  scale in section 2 must stay expressed as `rem` against the same root
  number, and higher contrast's overrides (`--text`, `--border`, `--focus`
  etc.) need the same variables to exist under new names if any get
  renamed.
- **Reduced motion.** `:root[data-motion="reduce"] * { transition: none
  !important }` is a blanket rule; any new animated affordance (a card
  entrance, a hover lift) needs to be added to, or already covered by, that
  selector — easy to forget on a specific new component.
- **Phone layout.** The `@media (max-width: 720px)` table-to-card
  transform (`app.css` ~lines 821–835) is doing real structural work
  (`display: block` on `<table>`/`<tr>`/`<td>`); if the shared list-row
  refactor (plan stage 4) changes the table's DOM shape, this breakpoint's
  selectors need rewriting in lockstep, not as an afterthought — it's easy
  to redesign desktop and discover the phone view silently reverted to a
  scrolling table.
- **Open menus surviving a redraw.** `drawnKey`/`settingsKey`/`dockDrawn`
  exist specifically so a poll every 600ms–2s doesn't blow away an open
  menu or steal focus. Any new component that gets drawn from `render()`
  needs to either be cheap to always redraw or get its own key-guard; this
  is a pattern to preserve, not simplify away.
- **Keyboard focus.** `rememberFocus`/`restoreFocus` in `downloads.js` are
  bespoke and fragile-looking; a redesign of the dock's queued-item markup
  (plan stage 4/5) needs to keep `data-focus` attributes in step with
  whatever buttons remain, or reordering a download will silently drop
  focus to nowhere.
- **`textContent` only, never HTML.** Every `el()` helper call and every
  place a filename, disc label or catalog name is inserted must stay
  string-only. This audit didn't find a violation, but a redesign touching
  `renderRow`, `pastItem`, catalog cards etc. across five files (plan stage
  4) is exactly the kind of wide mechanical change where an `innerHTML` can
  creep in by habit; worth a `grep -rn innerHTML static/` pass before
  merging that stage.
- **The horizontal `placeMenu` fix (decision 6) touches every menu at
  once** — Filter, row links, and any new ones. Test on the narrowest
  supported width (390px) with a menu opened from a button near each edge,
  not just the one case screenshotted here.

## Screenshots (for reference)

All in `.../scratchpad/shots/`:
`01-desktop-home.png` (full list, to-do cards), `02-desktop-filter-menu.png`,
`03-desktop-details.png`, `04-desktop-catalog.png` (Add images),
`05-desktop-archive.png` (Archive + History, faked data),
`07-desktop-settings.png`, `08-desktop-folder-picker.png`,
`09-desktop-update-checklist.png`, `10-desktop-identify.png` (What is this
file?), `11-desktop-scan-and-downloads.png` / `12-desktop-dock-open.png`
(faked: a scan and a 4-item queue running together — note the dock bar
appears earlier in these two full-page captures than it does live, a
Chromium full-page-screenshot artifact for `position: fixed` elements, not a
real layout bug), `13-desktop-empty-folder.png`,
`14b-phone-home-top.png`/`14c-phone-home-rows.png`,
`15b-phone-details.png`, `16b-phone-filter-menu.png` (shows the off-screen
bug), `17b-phone-settings.png`, `18b-phone-dock.png`/`18c-phone-dock-open.png`.
