"""เตรียมโลโก้/ไอคอนของโปรแกรม -> assets/logo.png และ assets/icon-1024.png
1) ถ้ามีไฟล์สองตัวนี้อยู่แล้ว (อัปโหลดเอง) จะไม่แตะ
2) ถ้ามีโลโก้ TESR ใน assets/brand/tesr-emblem.webp.b64 จะสร้างจากโลโก้นั้น
3) ถ้าไม่มีอะไรเลย จะสร้างโลโก้ชั่วคราวสี CI"""
import base64, io
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

A = Path(__file__).resolve().parent.parent / "assets"
LOGO, ICON, EMB = A / "logo.png", A / "icon-1024.png", A / "brand" / "tesr-emblem.webp.b64"
BLACK, CRIMSON, GOLD = (10, 10, 10, 255), (139, 0, 0, 255), (201, 168, 76, 255)

if LOGO.exists() and ICON.exists():
    print("logo/icon already present")
elif EMB.exists():
    emb = Image.open(io.BytesIO(base64.b64decode(EMB.read_text()))).convert("RGBA")
    if not ICON.exists():  # โลโก้กลางพื้นโปร่งใส 1024x1024
        s, pad = 1024, 40
        k = (s - 2 * pad) / max(emb.size)
        e = emb.resize((round(emb.width * k), round(emb.height * k)), Image.LANCZOS)
        icon = Image.new("RGBA", (s, s), (0, 0, 0, 0))
        icon.paste(e, ((s - e.width) // 2, (s - e.height) // 2), e)
        icon.save(ICON)
    if not LOGO.exists():  # โลโก้สูง 256 px สำหรับหัวโปรแกรม
        emb.resize((round(emb.width * 256 / emb.height), 256), Image.LANCZOS).save(LOGO)
    print("logo/icon built from TESR emblem")
else:
    font = lambda n: ImageFont.truetype(str(A / "fonts/ChakraPetch-Bold.ttf"), n)
    img = Image.new("RGBA", (720, 240), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.text((360, 104), "TESR", font=font(170), fill=GOLD, anchor="mm")
    d.rectangle((150, 198, 570, 214), fill=CRIMSON)
    img.save(LOGO)
    ic = Image.new("RGBA", (1024, 1024), (0, 0, 0, 0))
    d = ImageDraw.Draw(ic)
    d.rounded_rectangle((0, 0, 1023, 1023), radius=220, fill=BLACK)
    d.rounded_rectangle((40, 40, 983, 983), radius=190, outline=CRIMSON, width=24)
    d.text((512, 470), "TESR", font=font(300), fill=GOLD, anchor="mm")
    d.rectangle((262, 650, 762, 680), fill=CRIMSON)
    ic.save(ICON)
    print("placeholder logo/icon")
