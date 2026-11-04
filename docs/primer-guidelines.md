# Primer's rules, as a checklist

What GitHub's design manual ([primer.style](https://primer.style)) says, reduced to
the rules that apply to isoshelf, so the page can be checked against all of it and
not only the parts that look nice. Read in 2026-11-04. A rule marked **unverified**
is one the page could not be read for; it is not a rule until someone reads it.
`docs/primer-audit.md` holds the result of checking the page against this list;
`docs/design-system.md` says how the look is built.

## Foundations

**Layout** ([source](https://primer.style/foundations/layout))
- F1. Full pages are centred, at most 1280px wide. Content padding is 16px up to 1012px and 24px from 1280px.
- F2. Under 768px one column; from 768px up to two; from 1400px up to three.
- F3. Breakpoints (reference values, not rules): 320, 544, 768, 1012, 1280, 1400.
- F4. Settings-style list and detail: on narrow screens show the list or the detail, with a parent link back; never both.
- F5. A pane of metadata or actions may become a bottom sheet on narrow screens; don't stack a pane of links above the content.

**Spacing and size** ([source](https://primer.style/foundations/primitives/size))
- F6. Spacing scale (px): 2, 4, 6, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 64, 80, 96, 112, 128. Nothing else.
- F7. Densities: condensed 8px, normal 16px, spacious 24px.
- F8. Control heights: 24, 28, 32 (default), 40, 48. Gap between controls 4px (small) or 8px; 12-16px for touch.
- F9. Radius 3, 6 (default), 12, full. Border 1px (default), 2px, 4px. Focus ring 2px with -2px offset.

**Typography** ([source](https://primer.style/product/primitives/typography/), [guide](https://primer.style/foundations/typography))
- F10. Body medium 14px/1.5 is the dense default. Weights 300, 400, 500, 600 only. Caption and body small 12px. Title small 16/600.
- F11. Units in rem; unitless line heights. Left-aligned text, about 80 characters per line at most.
- F12. Monospace is for code (file names and versions are code here). Don't make emphasis with colour alone.
- F13. Headings: use semantic order; never change the order for looks.
- F14. Links navigate or download; buttons act. Links prefer an underline or another signifier meeting contrast.

**Colour** ([source](https://primer.style/accessibility/color-considerations))
- F15. Never colour alone: state needs text or an icon too. Contrast 4.5:1 text, 3:1 large text, input borders, functional icons.
- F16. Light and dark both pass; high-contrast themes exist; forced-colours mode is left alone.
- F17. Octicon pairs: info = accent, check = success, x = danger, alert = attention. Per-role meanings elsewhere are **unverified**.

**Icons** ([source](https://primer.style/octicons/usage-guidelines/))
- F18. Octicons at 12, 16 or 24px, only at their drawn size; 16 is the default; 12 only in condensed UI.
- F19. Don't recolour except to `fgColor-onEmphasis` on a coloured background. A meaningful icon has 3:1 contrast.
- F20. Decorative icons are `aria-hidden`. Meaningful ones have `aria-label` and `role="img"`.
- F21. Outline for off, filled for on. MIT licence, "(c) GitHub, Inc." (check the LICENSE file for the artwork).
- F22. An icon-only button needs an accessible name (visible text, then sr-only text, then `aria-labelledby`, then `aria-label`), written verb-noun.

**Accessibility** ([focus](https://primer.style/accessibility/design-guidance/focus-management), [motion](https://primer.style/accessibility/design-guidance/motion-and-animation), [semantics](https://primer.style/accessibility/design-guidance/semantic-html-and-aria))
- F23. Everything interactive is reachable by keyboard with a visible `:focus-visible` indicator; never remove an outline without replacing it.
- F24. Dialogs: focus moves in, is trapped, returns to the trigger. After removing content, move focus somewhere logical.
- F25. Native elements first; no `aria-label` on divs; unique ids; landmarks `main` and `nav`.
- F26. Transitions only under `prefers-reduced-motion: no-preference`. Motion over 5 seconds must be stoppable; no more than 3 flashes a second.
- F27. Controls at least 24x24 CSS px. Text must stay usable when resized.

## Components

**PageHeader** ([source](https://primer.style/product/components/page-header/))
- C1. The page's single top-level heading, with context, actions and navigation. Medium title by default; large for user-made content.

**UnderlineNav** ([source](https://primer.style/product/components/underline-nav/))
- C2. Switches between related views. Each tab has its own URL; items have a label, optionally an icon (all or none) and a counter.
- C3. Unique `aria-label` on the nav and `aria-current="page"` on the current item. Directly above the content it controls. Tabs that only swap panels are UnderlinePanels instead; don't mix.

**Button** ([source](https://primer.style/product/components/button/))
- C4. One primary per page where possible, never two in a group; secondary is the usual choice; invisible for compound controls; danger sparingly, with a confirm dialog.
- C5. Primary goes last in a group. Labels in sentence case, short, one line. Avoid `disabled` (not focusable); use `aria-disabled`. While loading, keep the button and announce politely. Dropdown triggers carry `aria-haspopup` and `aria-expanded`.

**ActionList, ActionMenu, SelectPanel** ([source](https://primer.style/product/components/action-menu/))
- C6. Menus: all items have icons or none; danger items last; no inputs or filter inside a menu (use SelectPanel); inactive items explain themselves in text.
- C7. SelectPanel: filterable pick list; shows selected first; non-modal saves automatically, modal needs Save and Cancel; don't reorder after a pick.
- C8. Destructive actions get a confirm dialog; reversible ones get undo.

**Label, Counter, StateLabel** ([source](https://primer.style/product/components/label/))
- C9. Label: small text for status or metadata; variants default, primary, secondary, accent, success, attention, severe, danger, done. Do/don't **unverified**.
- C10. Counter adds a count to nav items and buttons. StateLabel is for issue and pull request states, not file status (inferred).

**Banner** ([source](https://primer.style/product/components/banner/))
- C11. Page or section feedback; one at a time; short; critical, warning, success, info. A title is required. At most two actions. Critical banners aren't dismissible. Near the thing it is about.

**Blankslate** ([source](https://primer.style/product/components/blankslate/))
- C12. Explains why content is missing: a title, optional brief secondary text, at most one primary action and a link; a graphic never carries the meaning alone.

**Dialog** ([source](https://primer.style/product/components/dialog/))
- C13. A title (an h1 labelled by `aria-labelledby`) and a close button that can't be hidden; primary actions on the right, secondary on the left; single column.
- C14. A dialog with form changes isn't dismissed by the backdrop. Esc closes the top one. A 16px safe area; with inputs it is full-screen on narrow screens.
- C15. Confirmation dialogs: a specific title under five words (never "Are you sure?"), a body naming what is affected, imperative buttons of one to three words, never Yes and No, danger styling on the destructive one. Don't confirm low-risk or easily undone actions.

**Tooltip** ([source](https://primer.style/product/components/tooltip/))
- C16. Labels an icon-only button; only on interactive elements; short; never repeats visible text.

**Forms** ([source](https://primer.style/product/ui-patterns/forms/))
- C17. Labels three words or fewer, sentence case, never a placeholder instead of a label; fields stacked vertically; required fields marked; errors say what is wrong and how to fix it; `aria-invalid` and `aria-describedby`; avoid disabled submit.
- C18. ToggleSwitch applies at once, is labelled with a noun or verb (not the state), has `aria-labelledby`, and never replaces a checkbox. Checkboxes and radios sit in groups.
- C19. Saving: explicit by default; automatic only for toggles, segmented controls and single-select menus; don't mix in one form; say with text whether it saved; one save button, enabled, with an active verb; failure keeps the data.

**Progress and loading** ([source](https://primer.style/ui-patterns/loading))
- C20. No indicator under 1 second; a spinner for 1-3 seconds or unknown; a progress bar with visible text ("4 of 12") from about 3 seconds; over 10 seconds also allow background work; skeletons for large areas; announce with `role="status"`; set `aria-busy`.

**Links and truncation** ([source](https://primer.style/product/components/link/))
- C21. Link variants default, inline, muted. Prefer wrapping to truncating; if truncating, the full text in a tooltip and reachable by keyboard.

**Lists and tables** ([source](https://primer.style/product/components/data-table/guidelines/))
- C22. A table compares flat data across columns; a list suits long or empty-celled content. Blank empty cells (no dashes). Right-align numbers. Sortable headers are buttons. A blankslate when empty. Skeletons while loading. Drop columns on narrow screens.

## Patterns and content

- P1. Empty states ([source](https://primer.style/ui-patterns/empty-states)): a title, one primary action, no vague "There was a problem", alert icon on errors.
- P2. Degraded states ([source](https://primer.style/ui-patterns/degraded-experiences)): when a secondary part fails, show the page without it and say so; hide counts that are unavailable; never disable a control because of availability.
- P3. Notifications ([source](https://primer.style/ui-patterns/notification-messaging)): avoid toasts; use a Banner or an inline message near the control; keep a dialog open while its action runs and show a failure Banner at its top; if success is obvious, announce it to screen readers only.
- P4. Filtering ([source](https://primer.style/product/scenario-patterns/filter/)): filters above the content they govern, named; every filter input visibly labelled; the trigger shows the selected value; a way back from applied filters that doesn't rely on colour; empty results name the filters and offer reset; result changes announced in a `role="status"` region.
- P5. Navigation ([source](https://primer.style/product/ui-patterns/navigation/)): start the page with a PageHeader; tabs directly above their content; skip links; tab order follows visual order; narrow filters move into a menu.
- P6. Progressive disclosure ([source](https://primer.style/ui-patterns/progressive-disclosure)): sparingly; a chevron for show more, an ellipsis for truncated text, a kebab for menus; never a caret.
- P7. Drag and drop ([source](https://primer.style/accessibility/patterns/drag-and-drop/)): always another way to do it; keyboard dragging from a grab icon with arrows, Enter or Space, and Escape; announce every step through a live region; visual cues for origin and destination.
- P8. Content ([source](https://primer.style/product/getting-started/foundations/content/)): sentence case; no punctuation on headings, labels and buttons; no colons in labels; "and" not "&"; action labels start with a verb; never "click here"; plain active voice with "you"; no humour in errors; errors are specific, never blame the user, no "sorry" or "oops"; relative dates for recent times and full dates beyond about a month; lowercase "am" and "pm"; numerals with counts.

## Not covered by the sources read (unverified)

Per-role colour meanings, tooltip requirements for icon buttons, truncation rules, tab ARIA details, motion and shadow values, touch target minimums beyond 24px, landmarks, and the CSS-only `Box` and `Box-row` list pattern (the chosen draft used it). These need the pages read in a real browser.
