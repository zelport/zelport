# Relay Configuration Width Design

## Intent

Make Overview and Configuration use the same page frame in Ports over Relay.
The current half-width Configuration cards look accidental beside the full-width
Overview content.

## Chosen approach

Both Configuration panels fill the available module width. Their interactive
content remains left aligned and capped at 680px so form controls and explanatory
text do not become difficult to scan on wide displays.

The Relay connection panel constrains each direct content block. The settings
panel constrains its form. Existing narrow-screen padding and natural shrinking
remain unchanged. Overview, navigation, state, and module behavior do not change.

## Alternatives considered

- Keep the existing narrow cards. Rejected because the page frame changes when
  switching views and reads as an unfinished layout.
- Stretch cards and all form controls without a readable inner width. Rejected
  because very long inputs and text reduce scanability.
- Use full-width cards with a constrained inner column. Selected because it
  aligns the page while preserving form readability.

## Verification

The CSS contract test checks that neither Configuration panel owns a maximum
width and that the inner setup blocks and form retain the 680px cap. Browser
review checks Overview and Configuration at wide and narrow widths.
