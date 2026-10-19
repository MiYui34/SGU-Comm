# Word replacements

## Summary

For fun and to keep things mostly SFW: admins keep a list of words or
phrases and their replacements ("nice ass" → "beautiful behind"). A
matching message is posted with the phrase replaced, not refused. A
phrase can have several replacements; one is picked at random each time.
Matching sees through simple tricks: spaces or dots between letters
("a s s", "a.s.s"), Cyrillic and other lookalike letters ("аss"), case
and accents.

## Acceptance Criteria

- Tests (Go): whole words and phrases, several replacements picked at
  random (seeded in tests), the tricks above, no false hits inside other
  words ("class", "assistant"), edits too, Mumble clients' texts too.
- Lives next to the word filter (message rules); the same admin tab.
- The author sees the replaced text (no secret rewriting on others only).

## Notes

- Reuse the matching of the word filter (msgrules) and extend it with a
  lookalike map (Unicode confusables, a small built-in table) and
  separator folding; the filter gains these too.
