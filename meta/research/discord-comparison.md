# Discord compared with Kallisti (2026-10-17)

What Discord has from the user's view, what Kallisti has, and a first
opinion on each gap. The aim is a list to choose from, not a plan.

Rules for the opinions: 1. it is Mumble, 2. it is easy to use, 3. when
they conflict, easy to use wins. Kallisti is one community per server,
so features that exist to manage many servers count for less.

Opinion: **add** (people expect it, fits), **maybe** (useful, but
cost or scope), **skip** (does not fit, or only matters at Discord's
size). "Have" means Kallisti already does it.

Discord sources are listed at the end. Exact numbers (limits, ranges)
change often.

## Text chat

| Discord | Kallisti | Opinion |
|---|---|---|
| Markdown: bold, italic, strikethrough, code, quotes, links | have | — |
| Markdown: `\|\|spoiler\|\|`, `#` headers, `-#` subtext, lists | partly | **add** spoilers; others maybe. Spoilers are common in games and media chat |
| Code blocks with syntax colours | code blocks, no colours | maybe |
| Replies, jump to the original | have | — |
| Reply without a ping to the author | no | maybe |
| Reactions, picker, who reacted | have | — |
| Custom server emoji | no | **maybe**. Uploads exist already; Mumble clients would see text only |
| `:shortcode:` suggestions | no (open issue) | add |
| Threads | no | maybe. Big work; smaller communities use them less |
| Forum channels | no | skip |
| Pins | have | — |
| Search | have | — |
| Search filters (`from:`, `has:`, `in:`, dates) | no | **add** `from:` and `in:` first; cheap on the server |
| Forward a message | no | skip. One community; "copy link" (below) covers most of it |
| Copy a link to a message | no | **add**. Small, and makes "look here" possible |
| Polls | no | maybe. Fun in small groups, but a new message type |
| Link embeds | have | — |
| Remove an embed from your own message, `<url>` with no embed | no (?) | maybe |
| Attachments, paste, drag and drop | have | — |
| Mark an attachment as a spoiler | no | maybe, with text spoilers |
| Voice messages (hold the mic, waveform) | no | **maybe**. Fits a voice app; small on phones |
| GIF picker (Tenor) | no | skip, or later. Sends user searches to a third party |
| Stickers | no | skip |
| Slash commands with autocomplete | no | maybe, only together with a bot API |
| `@everyone`, `@here`, `@role` | no | **add** `@here` (people in the server now) or `@everyone`, with a permission |
| "(edited)" label | have | — |
| Up arrow edits your last message | have | — |
| Typing indicator | have | — |
| Unread badges, read state | have | — |
| "New" line at the first unread message, "jump to present" | partly (?) | **add** what is missing |
| Esc marks a channel read; "mark as unread" | no | add Esc; mark as unread maybe |
| `@silent` messages | no | skip until Web Push exists |
| Scheduled messages | no (Discord neither) | skip |

## Voice and video

| Discord | Kallisti | Opinion |
|---|---|---|
| Voice channels with users and speaking rings in the sidebar | have | — |
| Voice activity / push-to-talk | have | — |
| Noise suppression, echo cancellation | have (3 levels) | — |
| Per-user volume to 200 %, local mute | have | — |
| Server mute, server deafen, move, disconnect a user | Mumble clients only | **add** to the web app. Mumble has all of it; only the UI is missing |
| Drag a user into another channel (mods) | no | maybe, with the item above |
| Priority speaker | Mumble has it, no web UI | maybe |
| Stage channels | listen-only channels are close | skip |
| Soundboard | no | maybe. Fun, but it plays to Mumble clients too, and can annoy |
| Voice channel status ("ranked grind") | no | **maybe**. Mumble channel comments could hold it |
| Screen share with sound | have | — |
| Camera | no (open issue) | open issue |
| Grid and focus view for several streams | no (open issue) | open issue |
| Activities (games in the call) | no | skip |
| Text chat in voice channels | have | — |
| User limit for a channel | Mumble has it (max users), no web UI (?) | maybe |
| Bitrate for a channel | server-wide in Mumble | skip |
| End-to-end encrypted voice | no; Mumble encrypts to the server | skip. Self-hosting is the trust model |

## Server structure and moderation

| Discord | Kallisti | Opinion |
|---|---|---|
| Categories (own groups of channels) | two fixed groups | maybe. Mumble's channel tree could give it |
| Channel types: announcement, forum, media, stage | no | skip; a read-only text channel covers "announcements" |
| Read-only channels (only some roles can write) | listen-only for voice; text (?) | **add** if text lacks it |
| Roles with colours, member list grouped by role | admin and moderator only | maybe. Mumble groups could be the roles |
| Channel permission overrides | Mumble ACLs, no web UI | maybe a simple "who can see / write" per channel |
| Onboarding, rules screen, welcome screen | no | **maybe** a simple "rules" text that new users accept |
| Server discovery | no | skip |
| Invite links with expiry and max uses | no; the URL is open to all | **maybe**. Useful for private servers; guest accounts make it harder |
| Scheduled events with reminders | no | maybe later |
| Boosts, server tags | no | skip |
| AutoMod (word filter, spam filter) | no | maybe later; small communities seldom need it |
| Audit log | no | maybe. Admins ask "who deleted that?" |
| Slowmode | no | skip |
| Timeout (no chat, no voice for a time) | no | **add**. Gentler than a ban; Mumble can do it with a temporary mute plus a chat block |
| Kick, ban (and delete recent messages) | ban list in settings; kick and ban from Mumble clients only | **add** kick and ban to the user menu |
| Member list, profile popout | member list; popout (?) | — |
| Age verification | no | skip. Discord's age checks (2025–2026) push some users to self-hosted apps; worth a line on the website, not a feature |

## Social

| Discord | Kallisti | Opinion |
|---|---|---|
| DMs | have | — |
| Group DMs | no | skip; make a channel |
| Friends list, friend requests | no | skip. One community; the member list is enough |
| Profile: bio, pronouns, banner | avatar only | **maybe** bio and pronouns. Mumble has a user comment field that could hold the bio |
| Custom status (emoji + text, expires) | no | maybe, also in the Mumble comment |
| Presence: idle, do not disturb, invisible | online/offline only | **maybe** idle (automatic) and DND (stops notifications). Invisible does not fit Mumble |
| "Playing X", Spotify | no | skip |
| Nickname per server | no | skip; one server, so the name is the nickname |
| Block a user (hide their messages) | local mute for voice only | **add** hiding their text too. Cheap, and a safety feature |
| Inbox: mentions and unreads in one place | no | **maybe**. Search for "mentions of me" could cover it |

## Notifications

| Discord | Kallisti | Opinion |
|---|---|---|
| Per channel: all / mentions / nothing | mute only | maybe. Mute plus mentions covers most |
| Mute for 15 min / 1 h / 8 h / 24 h | mute until turned off | maybe |
| Mobile push | no (open issue: Web Push) | open issue |
| Desktop notifications | have (mentions and DMs) | — |
| Badge on the tab or app icon | no (?) | **add** a count in the tab title or favicon. Small and useful |

## Client

| Discord | Kallisti | Opinion |
|---|---|---|
| Ctrl/Cmd+K quick switcher | no | **add**. Small, and power users expect it |
| Alt+Up/Down between channels, Alt+Shift for unread ones | no | add with the switcher |
| Ctrl+/ shortcut list | no | add with the switcher |
| Themes (light, dark, true black) | dark only | **maybe** a light theme. Some people cannot read light text on dark |
| Compact message mode | no | maybe |
| Reduced motion, font size | no; browser zoom works | maybe; follow `prefers-reduced-motion` at least |
| Phones: swipe for the channel list, long press for actions | have | — |
| Phones: swipe for the member list, swipe a message to reply | no (?) | maybe |
| Installable app (PWA) | no | **add**. Needed for Web Push on iPhones anyway |
| Streamer mode | no | skip |
| Game overlay | no | skip |
| Several accounts | no | skip |
| Desktop: global push-to-talk, start with the system, tray | desktop app, PTT open issue | open issue |

## Bots and integrations

| Discord | Kallisti | Opinion |
|---|---|---|
| Bots | any Mumble bot works (jukebox) | — |
| Webhooks (post to a channel from CI, RSS) | no | **maybe**. Small on the server, useful for many groups |
| Slash commands, buttons in messages | no | skip for now |
| Rich embeds from bots | no | skip with the above |

## Short list

My order, for discussion. Most are small.

1. Moderation in the web app: kick, ban, timeout, server mute, move.
   Mumble does all of it; only the UI is missing.
2. Hide a user's messages (block), next to the local voice mute.
3. Spoilers (`||text||`, spoiler attachments).
4. Ctrl/Cmd+K switcher, Alt+Up/Down, Esc marks read.
5. Copy a link to a message.
6. Search filters `from:` and `in:`.
7. `@here` / `@everyone` with a permission.
8. Unread count in the tab title; installable PWA (then Web Push).
9. Idle and do not disturb.
10. Bio and pronouns in the profile (Mumble comment).

Bigger ones to decide on: threads, polls, voice messages, custom emoji,
invite links, roles with colours, a light theme, webhooks.

Skip: forum, stage and announcement channels, stickers, GIFs, friends,
group DMs, nicknames, boosts, discovery, activities, overlay, streamer
mode, several accounts.

## Sources

- Search: https://support.discord.com/hc/en-us/articles/115000468588
- Forwarding: https://support.discord.com/hc/en-us/articles/24640649961367
- Polls: https://support.discord.com/hc/en-us/articles/22163184112407
- Reactions: https://support.discord.com/hc/en-us/articles/12102061808663
- Voice messages: https://support.discord.com/hc/en-us/articles/13091096725527
- Inbox: https://support.discord.com/hc/en-us/articles/360045027712
- Pins to 250, role gradients: https://discord.com/blog/discord-update-september-25-2025-changelog
- Voice channel status: https://support.discord.com/hc/en-us/articles/19583625604887
- Stage channels: https://support.discord.com/hc/en-us/articles/1500005513722
- Soundboard: https://discord.com/blog/how-to-use-the-discord-soundboard-add-more-sounds
- E2EE voice: https://discord.com/blog/every-voice-and-video-call-on-discord-is-now-end-to-end-encrypted
- Onboarding: https://support.discord.com/hc/en-us/articles/11074987197975
- AutoMod: https://support.discord.com/hc/en-us/articles/4421269296535
- Server tags: https://support.discord.com/hc/en-us/articles/31444248479639
- Teen-by-default / age checks: https://discord.com/press-releases/discord-launches-teen-by-default-settings-globally
- Notifications: https://support.discord.com/hc/en-us/articles/215253258
- Shortcuts: https://support.discord.com/hc/en-us/articles/31232432266647
- Themes (2025–2026): https://discord.com/blog/improving-mobile-with-squircles-styles-and-spacing
- Account switcher: https://support.discord.com/hc/en-us/articles/4412732047127
- Markdown: https://gist.github.com/matthewzring/9f7bbfd102003963f9be7dbcf7d40e51
