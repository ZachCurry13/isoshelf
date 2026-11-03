# The design system

How the page looks, in one place, and how to change it. Read this before
touching a colour, a size or a stylesheet.

isoshelf's look is [Primer](https://primer.style) - GitHub's design system -
taken as its design tokens (`@primer/primitives 11.10.0`, MIT) and its component
specifications, written out in plain CSS under the page's own class names. No
Primer library runs in the page: isoshelf has no build step and draws its page
from forty small scripts, so what is adopted is Primer's *values and rules*, not
its components.

It was chosen over IBM Carbon, Microsoft Fluent 2 and Pico CSS (rough drafts of
each were built from the real published code and looked at side by side) because
it is documented end to end, its tokens are plain CSS variables, it is built for
dense lists of things (which an inventory of files is), and it ships the themes
the page needed anyway - light, dark and two high-contrast ones. Material was
set aside as a poor fit for a dense tool.

## Three layers

```
primer.css     Primer's tokens, under Primer's names. Generated. Never edited.
   |
tokens.css     isoshelf's names for them: --surface, --muted, --ok-fg ...
   |           The one file that says what each name is. Hand-written.
the rest       One stylesheet per part of the page, using only tokens.css's
               names. No colour, shadow or Primer name of its own.
```

Because the rules are in the middle layer, a different design is a different
`tokens.css` (and its own generated file beside `primer.css`), and no component
is touched. That is the whole point of the arrangement, and the tests below hold
it in place.

## The tokens

Every name `css/tokens.css` defines, what it is for, and where Primer's value
comes from. The look of the page is these values and nothing else.

| Name | For | From Primer |
|---|---|---|
| `--bg` | the page | `bgColor-default` |
| `--surface` | cards, the table, dialogs | `bgColor-default` |
| `--surface-2` | a quiet step: table headings, panels | `bgColor-muted` |
| `--inset` | a field set into the page | `bgColor-inset` |
| `--text` | words | `fgColor-default` |
| `--muted` | secondary words | `fgColor-muted` |
| `--border` | edges | `borderColor-default` |
| `--border-muted` | a quieter edge | `borderColor-muted` |
| `--accent` | links and marks | `fgColor-accent` |
| `--accent-fill` | something filled with the accent: a bar, a switch that is on | `bgColor-accent-emphasis` |
| `--on-accent` | words on that fill | `fgColor-onEmphasis` |
| `--accent-muted` | an accent tint | `bgColor-accent-muted` |
| `--focus` | the focus ring | `focus-outlineColor` |
| `--button-bg` | an ordinary button | `button-default-bgColor-rest` |
| `--button-bg-hover` | the same, pointed at | `button-default-bgColor-hover` |
| `--button-border` | its edge | `button-default-borderColor-rest` |
| `--primary-bg` | the button that does the thing (green) | `button-primary-bgColor-rest` |
| `--primary-bg-hover` | the same, pointed at | `button-primary-bgColor-hover` |
| `--primary-fg` | its words | `button-primary-fgColor-rest` |
| `--primary-border` | its edge | `button-primary-borderColor-rest` |
| `--control-bg` | text fields and selects | `control-bgColor-rest` |
| `--control-border` | their edge | `control-borderColor-rest` |
| `--track-bg` | a switch that is off | `controlTrack-bgColor-rest` |
| `--knob-bg` | the switch's knob | `controlKnob-bgColor-rest` |
| `--header-bg` | the top bar (dark in every theme) | `header-bgColor` |
| `--header-fg` | words on it | `header-fgColor-default` |
| `--header-fg-strong` | the name and the mark on it | `fgColor-onEmphasis` |
| `--tab-hover` | the underline under a jump-bar link you point at | `underlineNav-borderColor-hover` |
| `--shadow` | a thing resting on the page | `shadow-resting-small` |
| `--shadow-menu` | a menu | `shadow-floating-medium` |
| `--shadow-dialog` | a dialog or the details panel | `shadow-floating-large` |
| `--backdrop` | the veil behind a dialog | `overlay-backdrop-bgColor` |
| `--ok-fg`, `--ok-bg` | up to date, saved | `fgColor-success`, `bgColor-success-muted` |
| `--update-fg`, `--update-bg` | an update waiting; a notice | `fgColor-attention`, `bgColor-attention-muted` |
| `--bad-fg`, `--bad-bg` | a mismatch; an error | `fgColor-danger`, `bgColor-danger-muted` |
| `--eol-fg`, `--eol-bg` | end of life | `fgColor-done`, `bgColor-done-muted` |
| `--warn-fg`, `--warn-bg` | an image that might not boot | `fgColor-severe`, `bgColor-severe-muted` |
| `--missing-fg`, `--missing-bg` | a missing image | `fgColor-accent`, `bgColor-accent-muted` |
| `--manual-fg`, `--manual-bg` | download manually | `fgColor-muted`, `bgColor-neutral-muted` |
| `--muted-fg`, `--muted-bg` | couldn't check | `fgColor-muted`, `bgColor-neutral-muted` |
| `--star` | a starred image | `fgColor-attention` |
| `--radius` | buttons, fields, cards, rows | `borderRadius-medium` (6 px) |
| `--radius-sm` | small tags | `borderRadius-small` (3 px) |
| `--radius-lg` | dialogs | `borderRadius-large` (12 px) |
| `--radius-full` | pills, bars, counts | `borderRadius-full` |
| `--text-small` | captions, labels, file names, small buttons: 12 px | written here (see below) |
| `--text-medium` | what is being read: 14 px | written here |
| `--text-large` | titles and the name: 16 px | written here |
| `--leading` | line height | 1.5, Primer's |
| `--font` | the font | `fontStack-system` |
| `--mono` | file names and versions | `fontStack-monospace` |

Primer's own size tokens are in `rem` against a 16 px root, and this page's root
is 14 px (so that the whole page, being measured in `rem`, can be raised to 16 by
Larger text), so the three type steps are written in `tokens.css` rather than
taken from Primer. They are Primer's 12, 14 and 16.

Weights are 400, 500 and 600 only, as in Primer. There are no upper-case
micro-labels: Primer doesn't use them.

## Themes, and what Settings changes

Three attributes on `<html>` pick the colours: `data-color-mode` (`auto`,
`light` or `dark`; `auto` follows the computer), `data-light-theme` and
`data-dark-theme`. `settings.js` sets them from the Look settings, and
`index.html` carries the defaults so the page is right before any script runs.
`primer.css` defines twelve themes: light or dark, with ordinary colours or one of
Primer's two sets for colour blindness (`colorblind`, for red-green, and
`tritanopia`, for blue-yellow), each ordinary or high contrast. Their names are
those three put together: `light`, `dark_high_contrast`, `light_colorblind`,
`dark_tritanopia_high_contrast` and so on. `settings.js` builds the name from the
Theme, Status colours and Higher contrast choices; `internal/web/primer` lists
them all.

- **Status colours** selects a colour-blind set: red-green turns the greens into
  blue and the reds into orange, so the statuses and the primary button no longer
  depend on telling them apart.
- **Higher contrast** selects the high-contrast version of whichever set is in use. It also sets
  `data-contrast="high"`, which `accessibility.css` uses for the one thing that
  is not a colour: every status goes back to being a filled chip, not a dot.
- **Larger text** sets `data-text="large"`, which raises the root size from 14 to
  16 px. Everything else is in `rem`, so everything follows.
- **Reduce motion** sets `data-motion="reduce"`; `accessibility.css` stops the
  movement.

## The stylesheets

`css/` has one file for each part of the page, loaded in this order by
`index.html` and `styleguide.html`. Each starts with a line saying what it
holds, and none is over 200 lines.

| File | Styles |
|---|---|
| `primer.css` | Primer's tokens: generated, see below |
| `tokens.css` | isoshelf's names for the look (this document's table) |
| `base.css` | reset, body, links, the page grid, cards, the focus ring |
| `topbar.css` | the top bar and the jump bar |
| `target.css` | the folder card |
| `feedback.css` | progress bars, the spinner, the notice line |
| `controls.css` | buttons, fields, chips, the switch |
| `status.css` | status colours and the mark beside a status |
| `images.css` | the list of images, its rows, the phone layout |
| `logos.css` | the logo tile |
| `catalog.css` | Add images, Archive, History, Tools |
| `menus.css` | drop-down menus |
| `filters.css` | the Filter menu and its chips |
| `todo.css` | what wants doing, and the first-run question |
| `details.css` | the details panel |
| `settings.css` | the Settings panel |
| `picker.css` | the folder chooser |
| `identify.css` | the What is this file dialog |
| `dialogs.css` | the question, the checklist, the bug report |
| `downloads.css` | the downloads bar, adding a file, the drop overlay |
| `accessibility.css` | what higher contrast and less motion change beyond colour |

## The rules, and the tests that hold them

They are tests because the way this goes wrong is quiet: one colour typed into
one rule looks fine in light and is wrong in dark and in high contrast, and
nobody notices until someone who needs those does. All are in
`internal/web/css_test.go`.

- A rule uses a name from `tokens.css`, never a colour: `TestNoRawColoursOutsideTokens`.
- Only `tokens.css` names Primer: `TestOnlyTokensCssUsesPrimerNames`.
- Every name a rule uses is defined: `TestEveryTokenUsedIsDefined`.
- Every theme defines every Primer name the page uses: `TestEveryThemeDefinesEveryPrimerName`.
- `primer.css` is from the pinned version: `TestPrimerFileMatchesThePinnedVersion`.
- `index.html` links every stylesheet, `primer.css` then `tokens.css` first:
  `TestIndexLinksEveryStylesheet`.
- The style guide uses the same files and has a section for each:
  `TestStyleGuideLinksTheSameStylesheetsAsThePage`, `TestStyleGuideShowsEveryStylesheet`.
- No stylesheet runs past 200 lines: `TestStylesheetsStayShort`.

And in `internal/docs`: this document names every token, every stylesheet and
the pinned Primer version.

## How to

**Change a colour or a size.** Change the value in `tokens.css`. Nothing else.
If the new value is a different Primer token, say which in the table above.

**Add a colour.** Add a name in `tokens.css` defined from a Primer token, add it
to the table, then run `go run ./internal/web/primer/fetch` (the Primer token it
uses has to be in `primer.css` for every theme). Use the new name in the
rule.

**Look at a change.** Run isoshelf and open `/static/styleguide.html`. It shows
every token as the chosen theme resolves it and every component, using the
real files, with the theme, Larger text and the filled-status switch at the top.

**Update Primer.** Change `Version` in `internal/web/primer/primer.go`, run
`go run ./internal/web/primer/fetch` (it goes online, like the logo fetcher),
read the diff of `primer.css`, run the tests, and update the version in this
document. If Primer renamed a token the tests say which.

**Add a Primer theme** (it also has a dimmed one): add it to `primer.Themes`, run the
fetch, and add the choice to Look in `settings.js` and the setting behind it
(`settings.Appearance`).

**Add a component.** Put its rules in the stylesheet for the part of the page it
belongs to, using tokens. If it is a new part, add a stylesheet (and its line in
`index.html`, `styleguide.html` and the table above), and add it to the style
guide.

**Use another design system.** Write its tokens in a generated file beside
`primer.css`, and redefine the names in `tokens.css` from them. Components don't
change. Link it from `index.html` and `styleguide.html`.

## What this does not cover yet

**Spacing and layout.** Paddings, gaps and widths are still written as numbers in
the stylesheets (4, 8, 12, 16 px and some in between). Primer has a spacing scale
and `tokens.css` could carry it, but a sweep would nudge a hundred rules by a pixel
or two, so it was left until it is wanted. Until then, changing the density of the
page means editing numbers.

**Icons and motion.** The few icons are inline SVG that take the text colour; the
two animations are in `feedback.css` and `accessibility.css`.
