# Phones zoom in on text fields

## Summary

On phones (iOS Safari) the page zooms in when a text field gets focus,
for example on the login screen: Safari zooms on fields under 16px.

## Requirements

- Text fields, text areas and selects use at least 16px on touch screens.
- Zoom stays allowed (no maximum-scale in the viewport).

## Acceptance Criteria

- The CSS rule is in the global styles; checked in a phone-sized browser.

## Result

- Login fields use 16px (they inherited the label's 12px); on touch and
  narrow screens all text fields use at least 16px. Zoom stays allowed.
  Checked at iPhone 14 width: login fields and message box 16px. Not
  tested on a real iPhone.
