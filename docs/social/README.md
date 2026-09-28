# Social assets

Transiva's social cards use the apple.com blue/white palette (the same system as the other Zyvor repos).

| File | What it is |
|---|---|
| `transiva-share-card.html` / `.png` | 1200×630 light card: README hero and Open Graph image |
| `transiva-share-card-dark.html` / `.png` | 1200×630 dark card: shown by the README `<picture>` in dark mode |
| `transiva-social-card.html` / `.jpg` | 1600×900 (16:9) LinkedIn / X card (five-step story) |
| `zyvor-mark-blue.svg` | the Zyvor "Z" mark in blue, used by the cards (`zyvor-logo.svg` is the original orange brand file) |

## Palette

| Role | Light | Dark |
|---|---|---|
| Background | `#ffffff` → `#f5f5f7`, faint blue wash | `#000000` → `#0b0b0f`, faint blue wash |
| Text / secondary | `#1d1d1f` / `#6e6e73` | `#f5f5f7` / `#a1a1a6` |
| Hairline | `#d2d2d7` | `#2c2c2e` |
| Accent (blue) | `#0071e3` → `#2997ff` | `#0a84ff` → `#64b5ff` |
| Orange | `#ff6a2a`, one dot per image (the h2kvm hand-off) | same |

Fonts are Helvetica Neue and Menlo, so the cards render identically without web fonts.

## Rebuild

```bash
./docs/social/build-social-card.sh
```

Needs Google Chrome and macOS `sips` (override the browser with `CHROME=/path/to/chrome`). The script writes the
three outputs above.

## GitHub Social preview

GitHub's repository Social preview can't be set through the API. After changing the card, upload
`docs/social/transiva-share-card.png` by hand in the repository's **Settings → Social preview**.

Copy follows the project README (Community Edition scope: two sources, full exports, no CBT).
