#!/usr/bin/env python3
"""Generate a GitHub social preview card for raid (1280x640).

Usage:
    python3 docs/assets/gen_social_preview.py [--out PATH]

Dependencies: Pillow (`pip install Pillow`).

Fonts: uses DejaVu Sans (Regular/Bold) and DejaVu Sans Mono (Regular/Bold).
Candidate paths below cover typical Linux, macOS, and Windows installations;
pass any missing ones with DEJAVU_FONT_DIR=/path if your system keeps them
somewhere custom.
"""
import argparse
import os
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

W, H = 1280, 640

# GitHub dark palette
BG        = (13, 17, 23)          # #0D1117
BG_PANEL  = (22, 27, 34)          # #161B22
BORDER    = (48, 54, 61)          # #30363D
FG        = (240, 246, 252)       # #F0F6FC
DIM       = (139, 148, 158)       # #8B949E
ACCENT    = (255, 107, 71)        # #FF6B47 warm coral
GREEN     = (63, 185, 80)         # #3FB950
YELLOW    = (210, 153, 34)        # #D29922


def _find_font(filename: str) -> str:
    """Locate a DejaVu font file across common platform locations."""
    env_dir = os.environ.get("DEJAVU_FONT_DIR")
    candidates = []
    if env_dir:
        candidates.append(Path(env_dir) / filename)
    candidates += [
        # Linux
        Path("/usr/share/fonts/truetype/dejavu") / filename,
        Path("/usr/share/fonts/TTF") / filename,
        Path("/usr/share/fonts/dejavu") / filename,
        # macOS (Homebrew cask + manual install)
        Path("/opt/homebrew/share/fonts") / filename,
        Path("/usr/local/share/fonts") / filename,
        Path.home() / "Library/Fonts" / filename,
        # Windows
        Path(r"C:\Windows\Fonts") / filename,
    ]
    for p in candidates:
        if p.is_file():
            return str(p)
    sys.exit(
        f"error: could not find {filename}.\n"
        "Install the DejaVu fonts (e.g. `apt install fonts-dejavu`, "
        "`brew install --cask font-dejavu`) or set DEJAVU_FONT_DIR to "
        "the directory containing them."
    )


SANS_BOLD = _find_font("DejaVuSans-Bold.ttf")
SANS      = _find_font("DejaVuSans.ttf")
MONO_BOLD = _find_font("DejaVuSansMono-Bold.ttf")
MONO      = _find_font("DejaVuSansMono.ttf")

img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)

# ── Subtle dotted grid background ─────────────────────────────────────────────
grid_color = (28, 33, 40)
step = 32
for x in range(0, W, step):
    for y in range(0, H, step):
        d.point((x, y), fill=grid_color)
        # tiny cross to make dots visible
        d.point((x + 1, y), fill=grid_color)
        d.point((x, y + 1), fill=grid_color)

# ── Accent bar (left edge) ────────────────────────────────────────────────────
d.rectangle([0, 0, 8, H], fill=ACCENT)

# ── Left column: wordmark + tagline ───────────────────────────────────────────
LEFT = 72
f_mark    = ImageFont.truetype(SANS_BOLD, 168)
f_tag_big = ImageFont.truetype(SANS_BOLD, 42)
f_tag_sm  = ImageFont.truetype(SANS, 26)
f_url     = ImageFont.truetype(MONO, 24)

# Wordmark
WORD_XY = (LEFT, 70)
d.text(WORD_XY, "raid", font=f_mark, fill=FG)

# Colored accent underline — full width of the wordmark, sitting just below
# the baseline. Uses the actual text bbox so the line matches the letter
# width instead of a hardcoded guess.
wm_bbox = d.textbbox(WORD_XY, "raid", font=f_mark)
ul_y1 = wm_bbox[3] + 8
ul_y2 = ul_y1 + 10
d.rectangle([wm_bbox[0], ul_y1, wm_bbox[2], ul_y2], fill=ACCENT)

# Tagline (two-line) — must fit to the left of the terminal panel (x < 760)
d.text((LEFT, 290), "Orchestrate your team's", font=f_tag_big, fill=FG)
d.text((LEFT, 340), "multi-repo dev environment.", font=f_tag_big, fill=FG)

# Sub-tagline with keywords
d.text((LEFT, 410), "YAML  ·  macOS  ·  Linux  ·  Windows", font=f_tag_sm, fill=DIM)

# URL at the bottom
d.text((LEFT, H - 64), "github.com/8bitAlex/raid", font=f_url, fill=DIM)

# ── Right column: terminal panel ──────────────────────────────────────────────
PX, PY = 760, 120
PW, PH = 448, 400
RADIUS = 14

# Panel shadow
shadow_off = 6
d.rounded_rectangle(
    [PX + shadow_off, PY + shadow_off, PX + PW + shadow_off, PY + PH + shadow_off],
    radius=RADIUS, fill=(6, 9, 13),
)
# Panel background
d.rounded_rectangle([PX, PY, PX + PW, PY + PH], radius=RADIUS, fill=BG_PANEL, outline=BORDER, width=2)

# Title bar
TB_H = 44
d.rounded_rectangle([PX, PY, PX + PW, PY + TB_H], radius=RADIUS, fill=(30, 36, 43))
# flatten bottom of title bar
d.rectangle([PX, PY + TB_H - RADIUS, PX + PW, PY + TB_H], fill=(30, 36, 43))

# Traffic lights
cy = PY + TB_H // 2
for i, color in enumerate([(255, 95, 86), (255, 189, 46), (39, 201, 63)]):
    cx = PX + 22 + i * 22
    d.ellipse([cx - 7, cy - 7, cx + 7, cy + 7], fill=color)

# Title text
f_title = ImageFont.truetype(SANS, 18)
title_text = "~/dev/my-project"
tb = d.textbbox((0, 0), title_text, font=f_title)
tw = tb[2] - tb[0]
d.text((PX + (PW - tw) // 2, PY + 10), title_text, font=f_title, fill=DIM)

# Terminal content
f_cmd = ImageFont.truetype(MONO_BOLD, 26)
f_out = ImageFont.truetype(MONO, 22)
PROMPT = "$ "
content_x = PX + 28
content_y = PY + TB_H + 28
line_h = 42

lines = [
    ("cmd",  "raid install"),
    ("out",  "  cloning 4 repos \u2713"),
    ("cmd",  "raid env staging"),
    ("out",  "  staging \u2713"),
    ("cmd",  "raid test"),
    ("out",  "  passing 128/128 \u2713"),
    ("cmd",  "raid deploy"),
]

for kind, text in lines:
    if kind == "cmd":
        d.text((content_x, content_y), PROMPT, font=f_cmd, fill=GREEN)
        prompt_bb = d.textbbox((content_x, content_y), PROMPT, font=f_cmd)
        d.text((prompt_bb[2], content_y), text, font=f_cmd, fill=FG)
    else:
        d.text((content_x, content_y), text, font=f_out, fill=DIM)
    content_y += line_h if kind == "cmd" else 36

# Blinking cursor block on the last line
cursor_x = content_x + 22
cursor_y = content_y - 2
d.rectangle([cursor_x, cursor_y, cursor_x + 14, cursor_y + 26], fill=ACCENT)

# Save — default to writing next to this script (docs/assets/social-preview.png)
# so the script works from any checkout. Override with --out.
parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
parser.add_argument(
    "--out",
    type=Path,
    default=Path(__file__).resolve().parent / "social-preview.png",
    help="Output PNG path (default: %(default)s)",
)
args = parser.parse_args()

args.out.parent.mkdir(parents=True, exist_ok=True)
img.save(args.out, "PNG", optimize=True)
print(f"Wrote {args.out} ({W}x{H})")
