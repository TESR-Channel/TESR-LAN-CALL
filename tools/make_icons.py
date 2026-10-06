"""สร้างไอคอนทุกแพลตฟอร์มจากไฟล์ต้นฉบับ assets/icon-1024.png
ใช้: pip install pillow && python3 tools/make_icons.py
แล้วรันคำสั่งนี้ถ้ามี rsrc (ฝังไอคอนใน .exe ของ Windows):
  go install github.com/akavel/rsrc@latest
  rsrc -ico assets/icon.ico -arch amd64 -o packaging/windows/rsrc_windows_amd64.syso
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

A = Path(__file__).resolve().parent.parent / "assets"
src = Image.open(A / "icon-1024.png").convert("RGBA")
if src.size[0] != src.size[1]:
    s = max(src.size)
    sq = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    sq.paste(src, ((s - src.size[0]) // 2, (s - src.size[1]) // 2))
    src = sq
src = src.resize((1024, 1024), Image.LANCZOS)
src.save(A / "icon-1024.png")
for s in (512, 256, 192, 180, 128, 64, 32):
    src.resize((s, s), Image.LANCZOS).save(A / f"icon-{s}.png")
src.save(A / "icon.ico", sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
src.save(A / "icon.icns")

# ภาพประกอบในตัวติดตั้ง Windows (สี CI)
font = lambda n: ImageFont.truetype(str(A / "fonts/ChakraPetch-Bold.ttf"), n)
side = Image.new("RGB", (164, 314), (10, 10, 10))
d = ImageDraw.Draw(side)
ic = src.resize((120, 120), Image.LANCZOS)
side.paste(ic, (22, 40), ic)
d.text((82, 200), "LAN Call", font=font(24), fill=(201, 168, 76), anchor="mm")
d.rectangle((0, 306, 164, 314), fill=(139, 0, 0))
side.save(A / "installer-side.bmp")
head = Image.new("RGB", (150, 57), (10, 10, 10))
logo = Image.open(A / "logo.png").convert("RGBA")
logo.thumbnail((130, 44))
head.paste(logo, ((150 - logo.width) // 2, (52 - logo.height) // 2), logo)
ImageDraw.Draw(head).rectangle((0, 52, 150, 57), fill=(139, 0, 0))
head.save(A / "installer-header.bmp")
print("icons updated in", A)
