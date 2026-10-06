"""สร้างโลโก้/ไอคอนชั่วคราวตามสี CI ของ TESR (ใช้เมื่อยังไม่มีไฟล์จริงใน assets/)
ได้ assets/logo.png และ assets/icon-1024.png"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

A = Path(__file__).resolve().parent.parent / "assets"
BLACK, CRIMSON, GOLD = (10, 10, 10, 255), (139, 0, 0, 255), (201, 168, 76, 255)
font = lambda s: ImageFont.truetype(str(A / "fonts/ChakraPetch-Bold.ttf"), s)

if not (A / "logo.png").exists():
    img = Image.new("RGBA", (720, 240), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.text((360, 104), "TESR", font=font(170), fill=GOLD, anchor="mm")
    d.rectangle((150, 198, 570, 214), fill=CRIMSON)
    img.save(A / "logo.png")

if not (A / "icon-1024.png").exists():
    ic = Image.new("RGBA", (1024, 1024), (0, 0, 0, 0))
    d = ImageDraw.Draw(ic)
    d.rounded_rectangle((0, 0, 1023, 1023), radius=220, fill=BLACK)
    d.rounded_rectangle((40, 40, 983, 983), radius=190, outline=CRIMSON, width=24)
    d.text((512, 470), "TESR", font=font(300), fill=GOLD, anchor="mm")
    d.rectangle((262, 650, 762, 680), fill=CRIMSON)
    ic.save(A / "icon-1024.png")
print("placeholder logo/icon ready")
