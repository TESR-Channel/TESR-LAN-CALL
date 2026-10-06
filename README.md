# TESR LAN Call

**โปรแกรมโทรวิดีโอภายในองค์กรผ่าน Wi-Fi / LAN ของบริษัท**
Windows · Mac · Linux · Raspberry Pi · iPad · มือถือ · หุ่นยนต์ Kuro-X

ใช้กดโทรหากันภายในบริษัทได้ทันที ไม่ต้องใช้อินเทอร์เน็ต ไม่ต้องสมัครบัญชี
ทุกเครื่องในวงเดียวกันหากันเองอัตโนมัติ และหุ่นยนต์รับสายเองได้โดยไม่ต้องมีคนกด

## ⬇️ ดาวน์โหลด

กดลิงก์ตามอุปกรณ์ที่ใช้ (ได้เวอร์ชันล่าสุดเสมอ)

| อุปกรณ์ | ดาวน์โหลด | ติดตั้ง |
|---|---|---|
| **Windows 10 / 11** | [TESR-LAN-Call-Setup.exe](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/TESR-LAN-Call-Setup.exe) | ดับเบิลคลิก แล้วกดถัดไปจนจบ |
| **Mac** (ชิป M และ Intel) | [TESR-LAN-Call-Mac.zip](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/TESR-LAN-Call-Mac.zip) | แตกไฟล์ แล้วลากไปใส่ Applications |
| **Ubuntu / Debian** | [tesr-lan-call_amd64.deb](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/tesr-lan-call_amd64.deb) | ดับเบิลคลิก แล้วกด Install |
| **Raspberry Pi 4/5** (64-bit) | [tesr-lan-call_arm64.deb](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/tesr-lan-call_arm64.deb) | `sudo apt install ./tesr-lan-call_arm64.deb` |
| **Raspberry Pi** (32-bit) | [tesr-lan-call_armhf.deb](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/tesr-lan-call_armhf.deb) | `sudo apt install ./tesr-lan-call_armhf.deb` |
| **iPad / iPhone / Android** | ไม่ต้องดาวน์โหลด | สแกน QR จากหน้าโปรแกรมบนคอม ([ดูวิธี](#-ipad--iphone--android)) |
| **ครบทุกระบบ + คู่มือภาษาไทย** | [TESR-LAN-Call-All-Platforms.zip](https://github.com/TESR-Channel/TESR-LAN-CALL/releases/latest/download/TESR-LAN-Call-All-Platforms.zip) | แบ่งโฟลเดอร์ For-Windows, For-Mac, ... พร้อมไฟล์วิธีติดตั้ง |

ดูทุกเวอร์ชันได้ที่หน้า [Releases](https://github.com/TESR-Channel/TESR-LAN-CALL/releases)

---

## 🪟 Windows

1. ดับเบิลคลิก **TESR-LAN-Call-Setup.exe**
2. ถ้าขึ้นหน้าจอสีฟ้า *"Windows protected your PC"* กด **More info** แล้ว **Run anyway** (ขึ้นแค่ครั้งแรก เพราะยังไม่ได้ลงลายเซ็นดิจิทัล)
3. กด **Yes** เพื่ออนุญาตติดตั้ง แล้วกด **ถัดไป** จนจบ
   - ☑ **เปิดเองตอนเปิดเครื่อง** ติ๊กไว้ให้แล้ว (แนะนำ จะรับสายได้ตลอด)
   - ☐ **โหมดหุ่นยนต์ / จอแสดงผล** ติ๊กเฉพาะเครื่องบนหุ่นหรือจอหน้างาน
4. เสร็จแล้วมีไอคอน **TESR LAN Call** บน Desktop และโปรแกรมเปิดขึ้นมาเอง
5. ครั้งแรกกด **อนุญาต** กล้อง ไมค์ และการแจ้งเตือน แล้วตั้งชื่อเครื่องให้คนอื่นจำได้

ตัวติดตั้งเปิดไฟร์วอลล์ให้เอง ถอนการติดตั้งได้ที่ Settings > Apps > TESR LAN Call

## 🍎 Mac

1. ดับเบิลคลิก **TESR-LAN-Call-Mac.zip** แล้วลากแอป **TESR LAN Call** ไปใส่ **Applications**
2. เปิดครั้งแรก: คลิกขวาที่แอป > **Open** > **Open** (ทำครั้งเดียว)
   - ถ้ายังเปิดไม่ได้: **System Settings > Privacy & Security** > เลื่อนลงล่าง กด **Open Anyway**
   - ถ้าขึ้นว่าแอป *damaged*: เปิด Terminal แล้วพิมพ์ `xattr -cr "/Applications/TESR LAN Call.app"`
3. อนุญาต Local Network กล้อง ไมค์ และการแจ้งเตือน
4. ลากไอคอนจาก Applications ไปไว้ที่ Dock เพื่อเปิดครั้งต่อไป

แนะนำให้มี Google Chrome ในเครื่อง โปรแกรมจะเปิดเป็นหน้าต่างแอป (ถ้าไม่มีจะเปิดใน Safari แทน)

## 🐧 Linux / Raspberry Pi

```bash
sudo apt install ./tesr-lan-call_amd64.deb     # PC
sudo apt install ./tesr-lan-call_arm64.deb     # Raspberry Pi OS 64-bit
```
ไม่แน่ใจว่าเครื่องเป็นแบบไหน พิมพ์ `dpkg --print-architecture`

ติดตั้งแล้วจะมีไอคอนบน Desktop และในเมนู (ถ้าขึ้นว่าเปิดไม่ได้ ให้คลิกขวาที่ไอคอน > **Allow Launching**)
แนะนำ Chromium หรือ Chrome (`sudo apt install chromium-browser`) Raspberry Pi OS มีมาให้แล้ว

## 📱 iPad / iPhone / Android

ไม่ต้องติดตั้งแอป ใช้งานผ่านคอมเครื่องใดก็ได้ในบริษัทที่ลงโปรแกรมไว้

1. ต่อ Wi-Fi วงเดียวกับคอม
2. ที่หน้าโปรแกรมบนคอม ดูกล่อง **ใช้บนมือถือหรือ iPad** แล้วสแกน QR
3. ถ้าขึ้นว่าการเชื่อมต่อไม่เป็นส่วนตัว
   - iPhone/iPad: **แสดงรายละเอียด** > **ไปที่เว็บไซต์นี้**
   - Android: **ขั้นสูง** > **ไปยัง ... (ไม่ปลอดภัย)**
4. ตั้งชื่ออุปกรณ์ แล้วอนุญาตกล้องและไมค์
5. เพิ่มไอคอนบนหน้าจอโฮม
   - iPhone/iPad: ปุ่มแชร์ > **เพิ่มไปยังหน้าจอโฮม**
   - Android: เมนู ⋮ > **เพิ่มลงในหน้าจอหลัก**

> มือถือรับสายได้เมื่อเปิดแอปไว้บนหน้าจอเท่านั้น เหมาะกับ iPad ที่ตั้งประจำจุด เช่น ห้องประชุม หน้าห้อง
> แนะนำให้ตั้ง IP ของคอมเครื่องนั้นให้ตายตัว QR จะได้ไม่เปลี่ยน

## 🤖 หุ่นยนต์ Kuro-X / จอหน้างาน

**โหมดหุ่นยนต์** เปิดเต็มจอเองตอนเปิดเครื่อง และ**รับทุกสายทันทีโดยไม่ต้องมีคนกด**
ผู้ควบคุมขับหุ่นด้วยโปรแกรมเดิม ถึงเป้าหมายแล้วกดโทรหาหุ่นได้เลย คนหน้าหุ่นจะเห็นหน้าและคุยกันได้ทันที

| ระบบบนหุ่น | วิธีเปิดโหมดหุ่นยนต์ |
|---|---|
| Windows | ตอนติดตั้ง ติ๊ก **โหมดหุ่นยนต์ / จอแสดงผล** |
| Linux / Raspberry Pi | พิมพ์ `tesr-lan-call --robot` ครั้งเดียว |
| ทุกระบบ | ในโปรแกรม ติ๊ก **โหมดหุ่นยนต์ / จอแสดงผล** แล้วเปิดโปรแกรมใหม่ |

ออกจากเต็มจอ: `Alt+F4` (Windows/Linux) หรือ `Cmd+Q` (Mac)

## 📞 วิธีใช้งาน

1. เปิดโปรแกรม จะเห็นรายชื่อเครื่องที่ออนไลน์อยู่
2. กด **โทร** ที่ชื่อปลายทาง ปลายทางกด **รับสาย** (เครื่องโหมดหุ่นยนต์รับเอง)
3. ระหว่างคุย ปิด/เปิดไมค์ กล้อง สลับกล้อง แชร์หน้าจอได้ กด **วางสาย** เมื่อจบ
4. สายที่ไม่ได้รับจะอยู่ใน **ประวัติการโทร** พร้อมปุ่ม **โทรกลับ**

ปิดหน้าต่างได้ตามปกติ โปรแกรมยังทำงานเบื้องหลัง **เมื่อมีสายเข้าหน้าต่างจะเด้งขึ้นมาเอง**
ปิดโปรแกรมจริงด้วยปุ่ม **ปิดโปรแกรม** มุมขวาบน

## 🛠 แก้ปัญหา

| อาการ | วิธีแก้ |
|---|---|
| ไม่เห็นเครื่องอื่น | เช็กว่าอยู่ Wi-Fi วงเดียวกัน (Wi-Fi สำหรับแขกมักบล็อกการคุยกันระหว่างเครื่อง) หรือกด **เพิ่มด้วย IP** |
| ไฟร์วอลล์ | เปิด TCP 47800, TCP 47843, UDP 47801 (ตัวติดตั้ง Windows และ Linux เปิดให้เอง) |
| เสียงเรียกไม่ดัง | แตะแถบสีแดงด้านบนหนึ่งครั้ง |
| มือถือเข้าไม่ได้ | IP ของคอมอาจเปลี่ยน สแกน QR ใหม่แล้วกดยอมรับคำเตือนอีกครั้ง |
| ภาพกระตุก | เพิ่ม access point ให้ครอบคลุมเส้นทางที่หุ่นวิ่ง |
| ดูบันทึกการทำงาน | `~/.tesr_lan_call/log.txt` (Windows: `C:\Users\<ชื่อ>\.tesr_lan_call\log.txt`) |

---

## 👩‍💻 สำหรับทีมพัฒนา

**ออกเวอร์ชันใหม่:** แก้ค่า `version` ใน `main.go` แล้ว push ขึ้น `main`
GitHub Actions จะสร้างตัวติดตั้งทุกระบบและอัปโหลดขึ้นหน้า Releases ให้เอง (ประมาณ 3-5 นาที) ลิงก์ดาวน์โหลดด้านบนจะชี้ไปเวอร์ชันใหม่ทันที

**เปลี่ยนโลโก้:** อัปโหลด `icon-1024.png` (1024x1024) และ `logo.png` (แนวนอน พื้นโปร่งใส) ไว้ในโฟลเดอร์ [`assets/`](assets/) แล้วระบบจะสร้างไอคอนทุกแพลตฟอร์มให้ตอน build

**Build เองบน Ubuntu** (ได้ครบทุกระบบในคำสั่งเดียว):
```bash
sudo apt install -y golang-go nsis llvm zip python3-pil
bash build.sh        # ผลลัพธ์อยู่ที่ ../dist/
```

| ไฟล์ | หน้าที่ |
|---|---|
| `main.go` | ค้นหาเครื่อง (UDP), ส่งสัญญาณโทร (HTTP), โหมดมือถือ (HTTPS), รับสายตอนปิดหน้าต่าง |
| `platform.go` | เปิดหน้าต่างแอป (Edge/Chrome), เปิดเองตอนเปิดเครื่อง |
| `web/index.html` | หน้าโปรแกรมทั้งหมด (WebRTC) |
| `packaging/` | ตัวติดตั้ง Windows (NSIS), Mac (Info.plist), คู่มือแต่ละโฟลเดอร์ |
| `build.sh` | สร้างแพ็กเกจทุกระบบ |

**การทำงาน:** ทุกเครื่องประกาศตัวเองทาง UDP broadcast ทุก 2 วินาที กดโทรแล้วสัญญาณวิ่งผ่านโปรแกรมของสองฝั่งทาง HTTP ส่วนภาพและเสียงวิ่งตรงระหว่างเครื่องด้วย WebRTC (เข้ารหัส ไม่ออกนอก LAN)

**ข้อจำกัด:** ตัวติดตั้งยังไม่ได้ลงลายเซ็นดิจิทัล จึงมีคำเตือนตอนเปิดครั้งแรก · มือถือรับสายได้เฉพาะตอนเปิดแอปอยู่บนหน้าจอ · ใครก็ได้ในวง LAN เดียวกันโทรเข้าได้ ควรใช้ในเครือข่ายที่ไว้ใจได้

© TESR Co., Ltd. · ฟอนต์ Chakra Petch และ IBM Plex Sans Thai ใช้สัญญาอนุญาต SIL Open Font License
