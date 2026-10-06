"""zip โฟลเดอร์โดยเก็บชื่อไฟล์ภาษาไทยแบบ UTF-8 (Windows Explorer / macOS แสดงชื่อไทยถูกต้อง)
ใช้: python3 tools/zipdir.py <โฟลเดอร์> <ไฟล์.zip>"""
import os, sys, zipfile
src, out = os.path.abspath(sys.argv[1]), sys.argv[2]
base = os.path.dirname(src)
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
    for root, dirs, files in os.walk(src):
        dirs.sort()
        for name in sorted(dirs) + sorted(files):
            p = os.path.join(root, name)
            arc = os.path.relpath(p, base) + ("/" if os.path.isdir(p) else "")
            info = zipfile.ZipInfo.from_file(p, arc)
            info.flag_bits |= 0x800  # ชื่อไฟล์เป็น UTF-8
            if os.path.isdir(p):
                z.writestr(info, b"")
            else:
                info.compress_type = zipfile.ZIP_DEFLATED
                with open(p, "rb") as f:
                    z.writestr(info, f.read())
