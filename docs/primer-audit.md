# Audit of the page against Primer's rules

Checked 2026-11-04 against `docs/primer-guidelines.md` (v0.10.1). Three read-only
audits: structure and styling, behaviour and patterns, wording. Where a rule is not
named here it passed or does not apply. The aim is that someone who runs GitHub's
design system could audit the page and find nothing out of line; this lists
everything they would find today. Work through it top to bottom; when an item is
done, change its mark to **done** and say which release.

## Scorecard

| Area | Rules checked | Pass | Partial | Fail |
|---|---|---|---|---|
| Foundations (F1-F27) | 27 | 2 | 20 | 5 |
| Components (C1-C22) | 22 | 0 | 17 | 5 |
| Patterns and content (P1-P8) | 8 | 0 | 6 | 2 |
| Wording sweep | 79 findings in 10 groups | | | |

Nothing is wrong in colour, radius, weights, mono for file names, the sentence case
of labels, or the absence of toasts, "click here", "oops" and exclamation marks.

## Package 1 - foundations (spacing, size, focus, breakpoints, motion) - done in v0.11.0, see the result at the end

1. **Spacing is off Primer's scale.** 9 distinct values (1, 3, 5, 7, 9, 10, 14, 18, 52 px)
   in about 109 of 330 spacing declarations; 10px is the page's real unit (F6, F7).
2. **Control heights are wrong.** `.btn` is about 30px (should be 32), `.btn.small` about
   26px (should be 28), inputs 8px/10px padding, icon button 30px wide, control gaps
   6px and 10px (should be 4 or 8) (F8).
3. **Focus ring** is 3px with 1px offset; Primer's is 2px with -2px (F9).
4. **One breakpoint, 720px.** Primer's ranges are 768, 1012 and 1400, with two columns
   from 768 (F2, F3).
5. **Page padding** is 14px 20px; Primer's is 16px to 1012px and 24px from 1280px. The
   downloads bar is 1320px wide against a 1280px page (F1).
6. **Type:** a few hard-coded line heights (1, 1.1); long lines unbounded except one
   place; no 80-character measure (F10, F11).
7. **Forced-colours mode** is not handled (F16). Contrast of muted text, the star at
   55% opacity and the pills is not measured (F15).
8. **Motion:** caret and bar transitions are not inside `prefers-reduced-motion:
   no-preference`; only the Settings override stops them (F26).
9. **Targets:** row checkboxes and the star have no size rule; keep every target at
   least 24px (F27).

## Package 2 - icons - done in v0.11.0

10. **No Octicons.** Icons are text glyphs (gear, x, warning triangle, up and down
    arrows, a caret, a star); two SVGs are drawn at 14px, which is not an Octicon size
    (F18-F22). Needed: Octicons at 16px (12px only in condensed spots), bundled with
    their MIT notice; decorative ones `aria-hidden`, meaningful ones labelled;
    outline for off and filled for on (star, pin).
11. **Icon-only buttons:** "Close" is a bare noun, not verb-noun; `title` duplicates
    `aria-label`; the drag grip's hint is on an `aria-hidden` span (F22, C16).
12. **Disclosures** use a custom caret; Primer says a chevron for show-more, an
    ellipsis for truncated text and a kebab for menus (P6).

## Package 3 - structure - done in v0.11.0, see the result at the end

13. **No PageHeader and no `h1`.** The brand is a span; there is no skip link (F13,
    C1, P5).
14. **The jump bar is not an UnderlineNav.** No counters on the items, no icons, no
    `aria-current`, not directly above the content (C2, C3).
15. **The image list is a table**, with sortable headings that are not buttons,
    numbers not right-aligned, no skeletons while loading, and no list-in-a-box
    structure like the chosen draft (C22). Dense rows of mixed content suit
    Primer's list-in-a-Box pattern better than DataTable.
16. **Status and tags are custom pills**, not Label variants; counts are plain muted
    text, not Counters (C9, C10).
17. **Empty states are bare text**, with no title and no primary action ("No images
    found in this folder." offers nothing to do) (C12, P1).
18. **Notices are one status line**, not a Banner: no title, no severity, one element
    toggled by a class, success vanishes after about 10 seconds, and a dialog closes
    before the work runs so a failure appears after it has closed (C11, P3).
19. **Truncation** (dock names, picker rows) has no full text for keyboard or tooltip
    (C21).

## Package 4 - behaviour - done in v0.11.0, see the result at the end

20. **Settings mixes save models.** Toggles and selects save at once; the speed limit
    saves on blur; there is a Reset button and a Check now button; "Saved" disappears
    after 3 seconds outside any live region; failure goes to the generic notice
    (C19). Switches are plain checkboxes, not a ToggleSwitch with `role="switch"`
    (C18).
21. **The Filter menu** puts checkboxes in a `details` menu; Primer says a menu has no
    inputs, and a filter panel is a SelectPanel with a trigger showing the selection
    (C6, C7, P4).
22. **The results line** ("N of M images") is not a live region, so filtering is not
    announced; the search box has no visible label (P4).
23. **Queue reordering** is mouse-only drag plus move buttons. There is no keyboard
    drag from the grip and nothing announces a move (P7).
24. **Confirm dialogs:** the destructive button is not danger-styled; two titles are
    five words; "Stop all downloads?" does not name what stops; Cancel sits on the
    right in some dialogs and the left in others (C13, C15).
25. **Dialogs with inputs** can be dismissed by clicking outside; none goes
    full-screen on a narrow screen; titles are `h2`, not `h1`; `ask` and `pick` have
    no close button (C13, C14).
26. **Forms** ("Name it myself", sign-in): no required marks, `aria-invalid` or
    `aria-describedby`; errors go to the shared notice, not next to the field (C17).
27. **Availability is shown by disabling:** Refresh, the profile select, Empty archive
    use `disabled` (not focusable); Primer says `aria-disabled` with an explanation
    (C5, P2).
28. **Focus is lost** after a row is removed and the table redraws; `ask` does not
    focus the safe button (F24).
29. **Table sort headers** may not be keyboard-reachable (F23).

## Package 5 - wording (79 findings) - done in v0.11.0

30. Punctuation on labels and buttons: ellipses on 7 buttons ("Remove…", "Choose
    folder…"), two colons on labels, trailing periods on status text (15).
31. Button labels: "Confirm", "Done", "Got it", "No, I'll press Update", pronoun
    buttons ("Delete them", "Stop them"), "Details", "Next in line" (17).
32. Error messages with no fix, or that blame ("Those two passwords aren't the
    same"): about 20, including bare `bad request`.
33. Settings labels over three words: 14 ("Check for updates automatically",
    "Where this folder's records are kept").
34. Dates: absolute after 24 hours, abbreviations ("5 min ago"), no relative form to
    about a month (6).
35. Dialog titles over five words (2) and confirmations that don't name what is
    affected (1); "Click to show them"; "Just plugged a drive in?" (2).

## Decisions made

- The list-in-a-Box layout of the chosen draft, not DataTable.
- Anchored sections with an UnderlineNav that follows the scroll (as in the draft),
  not separate tab views.
- Every setting saves at once and says so in a live region; the speed limit becomes
  a single-select of common values plus the current custom one.
- Filtering becomes a labelled filter panel; the sort stays a select.
- Octicons are bundled (MIT notice in the repository).

## Result, v0.11.0

Every item above was done, except these, which are open or a decision, so that
nobody reading this is told the page is more finished than it is:

**Closed after v0.11.1 (the three that were open):**
- **Contrast (F15) is measured, in all twelve themes**, by
  `internal/web/contrast_test.go`: 25 pairs of words or icons on their
  background (body, muted, link, header, buttons, every status pill on its tint
  and on the page), 300 measurements, each at least WCAG's 4.5:1 for text or
  3:1 for an icon or the focus ring. The tightest is "End of life" on its tint
  in the light themes, 4.54:1. A Primer update that makes something hard to
  read fails the build.
- **The measure (F11):** Settings hints stop at 80 characters, as the
  first-run text already did at 70.
- **Truncation and tooltips (C16, C21), read in the manual this time.**
  Nothing truncates (no `text-overflow` anywhere), which is the manual's first
  choice. The tooltip now does what Primer's must: Escape hides it without
  moving focus or the pointer, and a long one wraps instead of running off a
  320 px screen. Tooltips came off everything you can't press (the status
  pills, which the status key and the details panel explain; the 32-bit and
  ARM badges, whose full name is in the details panel; the "popular" pill,
  whose explanation moved to the Popular filter's checkbox) and off a download's
  name in the bar, which repeated the words it sat on.

**Open (not done):**
- **Longer descriptions on buttons with a visible label** are still the
  browser's own `title` tooltip (about forty of them, such as what each of the
  three "replace" choices does). Primer allows a short description on such a
  button, but its own is shown on keyboard focus too and the browser's isn't, so
  a keyboard user without a screen reader can't see them. The fix is to put the
  ones that matter on the page (Primer: "persist the content") and drop the
  rest, which is a wording pass, not a style one.
- **The exact time on a History entry** is a `title` on plain text. Primer's own
  RelativeTime does the same, so it stays.

**Decided, with the reason:**
- **The tab bar is links to places on one page**, not tabs that swap panels, so it
  is an UnderlineNav and the current one is `aria-current="location"` (Primer's
  example says `page`, which is right for navigation between pages and wrong for
  this).
- **The Filter menu stays a menu holding a list of choices**, applied at once and
  closed by Close, with its count on the trigger: Primer's non-modal SelectPanel in
  effect, though it is built on `details` and not a component of its own.
- **One column at every width.** Primer allows two columns from 768 px; a list of
  files with a status and buttons on each row has nothing to put in a second one
  (a side pane was drawn and dropped: it is not in the chosen draft).
- **The search box has an accessible label but no visible one**, as Primer's own
  SearchInput does in a toolbar; the Sort menu has a visible label.
- **Reset and Check now in Settings are actions, not settings**, so they are not
  a second save model; every setting saves at once and says so aloud.
