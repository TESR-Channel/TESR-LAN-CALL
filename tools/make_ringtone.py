"""สร้างเสียงเรียกเข้า TESR "Modern" -> assets/ringtone.wav (ไม่ต้องใช้ไลบรารีเสริม)
อาร์เพจจิโอ C-E-G-C-G-E สองรอบ + เสียงสะท้อน ปรับให้ดังเต็มระดับ (peak -1 dBFS)"""
import math, struct, wave
from pathlib import Path

SR = 44100
LEN = 6.0
OUT = Path(__file__).resolve().parent.parent / "assets" / "ringtone.wav"


def env(i, n, a=0.004, r=0.05):
    A, R = int(SR * a), int(SR * r)
    if i < A:
        return i / A
    if i >= n - R:
        return (n - i) / R
    return 1.0


def pluck(f, d=0.5):
    n = int(SR * d)
    out = []
    for i in range(n):
        x = i / SR
        s = (math.sin(2 * math.pi * f * x) + 0.4 * math.sin(2 * math.pi * 2 * f * x)
             + 0.2 * math.sin(2 * math.pi * 3 * f * x)) * math.exp(-x * 6)
        out.append(s * env(i, n))
    return out


buf = [0.0] * int(SR * LEN)
seq = [523.25, 659.25, 783.99, 1046.5, 783.99, 659.25]
for rep in range(2):
    for k, f in enumerate(seq):
        start = int(SR * (rep * 3.0 + k * 0.18))
        for j, v in enumerate(pluck(f)):
            if start + j < len(buf):
                buf[start + j] += v
d = int(SR * 0.18)  # เสียงสะท้อน
buf = [buf[i] + (buf[i - d] * 0.35 if i >= d else 0.0) for i in range(len(buf))]

peak = max(abs(v) for v in buf)
k = math.tanh(1.8)
buf = [math.tanh(v / peak * 1.8) / k for v in buf]
peak = max(abs(v) for v in buf)
OUT.parent.mkdir(parents=True, exist_ok=True)
with wave.open(str(OUT), "wb") as w:
    w.setnchannels(1)
    w.setsampwidth(2)
    w.setframerate(SR)
    w.writeframes(b"".join(struct.pack("<h", int(v / peak * 0.89 * 32767)) for v in buf))
print("wrote", OUT)
