# ระบบบันทึกเวลาเข้า-ออกงานด้วยการจดจำใบหน้าแบบ Edge AI บน Raspberry Pi 5 พร้อมแดชบอร์ดกลางสำหรับหลายอุปกรณ์
> **Edge AI Face Recognition Attendance System on Raspberry Pi 5 with Central Management Dashboard**  
> โครงงานสหกิจศึกษา 16 สัปดาห์ สาขาวิชาวิศวกรรมคอมพิวเตอร์

---

## 📌 ภาพรวมโครงการ (Project Overview)

ระบบบันทึกเวลาเข้า-ออกงานของพนักงาน (~200 คน) ด้วยเทคโนโลยีการรู้จำใบหน้าแบบ Edge AI ทำงานประมวลผลบน Raspberry Pi 5 ร่วมกับโมดูลเร่งความเร็ว Hailo-8 (ไม่ส่งภาพสดหรือภาพถ่ายขึ้นคลาวด์ตามหลัก PDPA) มีสถาปัตยกรรมแบบ **Offline-First** เก็บบันทึกข้อมูลลงฐานข้อมูลในเครื่องก่อนเสมอ และซิงก์ข้อมูลขึ้นเซิร์ฟเวอร์กลางเมื่อเชื่อมต่อเครือข่าย พร้อมแดชบอร์ดกลางสำหรับติดตามสถานะเครื่อง Kiosk ทุกจุด และบริหารจัดการข้อมูลผู้ใช้แบบรวมศูนย์

รองรับการพัฒนาและทดสอบได้ทั้งบน **Raspberry Pi 5 (ฮาร์ดแวร์จริง)** และบน **คอมพิวเตอร์ทั่วไป (PC / Laptop)** ผ่านระบบตรวจจับสภาพแวดล้อมและ Fallback อัตโนมัติ

---

## 🛠️ รายละเอียดฮาร์ดแวร์ (Hardware Specifications)

* **หน่วยประมวลผลหลัก:** Raspberry Pi 5 (8 GB RAM), Raspberry Pi OS Bookworm 64-bit
* **ชิปประมวลผล AI:** Raspberry Pi AI HAT+ 26 TOPS (Hailo-8 M.2, PCIe Gen3)
* **โมดูลกล้อง:** Raspberry Pi Camera Module 3 (Sony IMX708, ออโต้โฟกัส)
* **จอแสดงผล:** Raspberry Pi Touch Display 7 นิ้ว (ติดตั้งในแนวตั้ง Portrait 480×800)
* **อุปกรณ์เสริม:** Active Cooler, Power Supply 27W USB-C, MicroSD Card 128 GB

---

## 🏛️ สถาปัตยกรรมระบบ (System Architecture)

ระบบประกอบด้วย 3 ส่วนหลัก:

```
┌─────────────────────────────────┐           HTTPS (LAN)          ┌─────────────────────────────────┐
│     Kiosk Device (Pi 5)         │ ─────────────────────────────> │      Central Server (Docker)    │
│  - Face Pipeline (Hailo-8)      │ <───────────────────────────── │  - FastAPI REST API (Python)    │
│  - Local SQLite (WAL Mode)      │      Heartbeat & Data Sync     │  - PostgreSQL / SQLite          │
│  - PyQt5 Touchscreen UI (Portrait) │                                │  - Caddy Reverse Proxy (TLS)    │
└─────────────────────────────────┘                                └─────────────────────────────────┘
                                                                                    │
                                                                           Web UI   │ (Admin LAN)
                                                                                    ▼
                                                                   ┌─────────────────────────────────┐
                                                                   │     Management Dashboard        │
                                                                   │  - React 18 + Vite SPA          │
                                                                   │  - Devices, Users, Logs, Jobs   │
                                                                   └─────────────────────────────────┘
```

1. **Kiosk (Edge Device):**
   * **AI Pipeline:** SCRFD (Face Detection & 5 Landmarks) ➔ 5-Point Affine Alignment (112×112) ➔ ArcFace MobileFaceNet (512-d Feature Extraction บน Hailo-8) ➔ Cosine Similarity Search
   * **Cross-Platform Fallback:** ตรวจจับชิป Hailo และกล้องอัตโนมัติ หากรันบนคอมพิวเตอร์จะสลับไปใช้ CPU Fallback และ Webcam/Mock Stream เพื่อให้ทดสอบโค้ดได้ทันที
   * **Offline-First Storage:** SQLite (WAL mode) เก็บ events, outbox, users และ embeddings
   * **Sync Workers:** ทำงานเบื้องหลังแยกเธรด (Batch upload พร้อม Exponential Backoff, ดึงข้อมูลผู้ใช้ด้วย `since=<version>`, Heartbeat)
   * **Touch UI:** พัฒนาด้วย PyQt5 ออกแบบขนาดแนวตั้ง 480×800 (Portrait) พอดีกล่องคีออสก์จอสัมผัส 7 นิ้ว

2. **Central Server (เซิร์ฟเวอร์กลาง On-Premise):**
   * **Backend API:** Python FastAPI พร้อม SQLAlchemy และ Alembic (สลับใช้งานได้ทั้ง SQLite ช่วงพัฒนา และ PostgreSQL บนระบบจริง)
   * **Device Authentication:** ยืนยันตัวตนเครื่อง Kiosk ด้วย API Key แบบแยกรายเครื่อง (เข้ารหัส Hash ด้วย bcrypt ในระบบ)
   * **Data Integrity & Reconciliation:** จับคู่เวลาเข้า-ออก (IN/OUT Matching) บนเซิร์ฟเวอร์ และรองรับ Idempotency ผ่าน UUID ของเหตุการณ์
   * **Deployment:** บรรจุในคอนเทนเนอร์ด้วย Docker Compose พร้อม Caddy ทำหน้าที่ Reverse Proxy และ TLS ภายใน

3. **Admin Dashboard (แดชบอร์ดกลาง):**
   * **Frontend:** React 18, Vite, React Router, React Query (@tanstack/react-query), Chart.js
   * **ฟังก์ชันการทำงาน:**
     * 🖥️ **Devices:** ดูสถานะสดของกล้องทุกตัว (Online/Offline, FPS, อุณหภูมิ CPU, Outbox Pending, Sync Version)
     * 👤 **Users:** จัดการข้อมูลพนักงาน เพิ่ม/ลบ (Soft Delete) และกระจายไปยังทุก Kiosk อัตโนมัติ
     * 🕐 **Attendance:** รายการลงเวลาย้อนหลัง, กราฟสรุปความหนาแน่นเวลาทำงาน, ส่งออกรายงานเป็น CSV
     * 📸 **Enrollment Jobs:** สร้างคำขอลงทะเบียนใบหน้าจากส่วนกลางเพื่อส่งภาพให้ Kiosk ประมวลผล Embedding บนฮาร์ดแวร์จริง
     * 📝 **Audit Log:** บันทึกประวัติการกระทำของผู้ดูแลระบบทั้งหมดเพื่อความโปร่งใส

---

## 📁 โครงสร้างโปรเจกต์ (Project Structure)

```text
.
├── Caddyfile                   # การตั้งค่า Caddy Reverse Proxy + Static Web Serving
├── docker-compose.yml          # คอนฟิกบริการ Server, PostgreSQL และ Caddy
├── docs/                       # เอกสารประกอบโครงงาน รายงาน และคู่มือ
│
├── kiosk/                      ← ซอร์สโค้ดฝั่ง Raspberry Pi 5 (Python Flat Layout)
│   ├── config.example.yaml     # คอนฟิกตัวอย่างหลัก (YAML)
│   ├── config.py               # โหลดคอนฟิกจาก YAML / .env พร้อม Fallback
│   ├── main.py                 # จุดเริ่มโปรแกรม Kiosk (เชื่อมต่อ UI + Workers)
│   ├── camera.py               # กล้อง Picamera2 + OpenCV Webcam + Mock Stream
│   ├── detector.py             # รันโมเดลบนชิป Hailo-8 (SCRFD + ArcFace) พร้อม CPU Fallback
│   ├── alignment.py            # จัดมุมใบหน้า 112×112 จาก 5 Landmarks (Affine Transform)
│   ├── matcher.py              # เทียบเอกลักษณ์ใบหน้าด้วย Cosine Similarity
│   ├── database.py             # คลาสจัดการฐานข้อมูล SQLite (WAL Mode, Transactions, Debounce)
│   ├── schema.sql              # โครงสร้างตาราง SQLite บน Kiosk (users, outbox, events ฯลฯ)
│   ├── metrics.py              # ตรวจวัดความร้อน CPU, FPS และจำนวนคิว Outbox
│   ├── requirements.txt        # รายการแพ็กเกจ Python สำหรับ Kiosk
│   ├── models/                 # ไฟล์โมเดลคอมไพล์แล้ว (*.hef) จาก Hailo Model Zoo
│   │   ├── scrfd_2.5g.hef      # ตรวจจับใบหน้าและ 5 Landmarks (ขนาดเล็ก รวดเร็ว)
│   │   ├── scrfd_10g.hef       # ตรวจจับใบหน้าและ 5 Landmarks (ความแม่นยำสูง)
│   │   └── arcface_mobilefacenet.hef # สกัดเวกเตอร์เอกลักษณ์ใบหน้า 512 มิติ
│   ├── scripts/
│   │   ├── download_models.sh  # สคริปต์ดาวน์โหลดโมเดลทางการจาก Hailo Model Zoo S3
│   │   ├── run_detector.py     # ⭐ สคริปต์รันตรวจจับใบหน้าและ 5 Landmarks สดผ่านกล้อง
│   │   └── setup.sh            # สคริปต์เตรียม Virtualenv และ Dependency
│   ├── ui/                     # ส่วนหน้าจอสัมผัส PyQt5 (Portrait 480×800)
│   │   ├── __init__.py
│   │   └── app.py              # หน้าจอลงเวลา + สลับเข้า/ออก + กล่องลงทะเบียนใบหน้า 3-5 ภาพ
│   └── workers/                # เธรดเบื้องหลังแยกการทำงาน
│       ├── recognition_worker.py # ลูปจับภาพ + ประมวลผลใบหน้า + บันทึก Event
│       ├── outbox_worker.py    # ซิงก์ประวัติลงเวลาขึ้นเซิร์ฟเวอร์แบบ Exponential Backoff
│       ├── user_sync.py        # ซิงก์ข้อมูลพนักงานลงเครื่องด้วย version-based sync
│       └── heartbeat.py        # ส่งสถานะสุขภาพเครื่อง (CPU Temp, FPS, Outbox)
│
└── server/                     ← ฝั่งเซิร์ฟเวอร์องค์กร (รันบน Docker / Host)
    ├── backend/                # ซอร์สโค้ด API เซิร์ฟเวอร์กลาง (Python FastAPI)
    │   ├── app/
    │   │   ├── main.py         # FastAPI Entry Point + Seed ข้อมูลเริ่มต้น
    │   │   ├── db.py           # การเชื่อมต่อ SQLAlchemy (SQLite / PostgreSQL)
    │   │   ├── models.py       # ORM Data Models (10 ตารางตามสถาปัตยกรรม)
    │   │   ├── security.py     # การเข้ารหัส Bcrypt, JWT Token, Device API Key
    │   │   └── api/            # API Endpoints แยกหมวดหมู่
    │   │       ├── auth.py     # เข้าสู่ระบบ Admin Dashboard
    │   │       ├── sync.py     # ซิงก์เหตุการณ์จาก Kiosk + จัดการ version-based sync
    │   │       ├── users.py    # จัดการพนักงาน (CRUD, Optimistic Lock, Soft Delete)
    │   │       ├── devices.py  # สถานะ Heartbeat และลงทะเบียนอุปกรณ์
    │   │       ├── attendance.py # ดึงประวัติลงเวลา และจับคู่เวลาเข้า-ออก
    │   │       ├── enrollment.py # พักภาพถ่ายและสร้าง Job สำหรับลงทะเบียน
    │   │       └── audit.py    # บันทึก Audit Log เพื่อความโปร่งใส
    │   ├── alembic/            # Database Migrations (รองรับการอัปเกรดฐานข้อมูล)
    │   ├── alembic.ini
    │   ├── requirements.txt    # รายการแพ็กเกจ Python สำหรับ Backend
    │   ├── Dockerfile          # Dockerfile ของ API Server
    │   └── .env.example
    ├── frontend/               # หน้าเว็บแดชบอร์ดจัดการระบบ (React 18 + Vite)
    │   ├── src/
    │   │   ├── api.js          # HTTP Client พร้อมดึง JWT จาก localStorage
    │   │   ├── App.jsx         # กำหนด Layout และ Routes ทั้งหมด
    │   │   ├── main.jsx        # จุดเริ่มการเรนเดอร์ React
    │   │   ├── components/     # UI Components (Sidebar, StatusBadge, ProtectedRoute)
    │   │   └── pages/          # หน้าเว็บ (Devices, Users, Attendance, Enrollment, Audit, Login)
    │   ├── index.html
    │   ├── vite.config.js
    │   └── package.json
    ├── certs/                  # ใบรับรองความปลอดภัย TLS ภายใน
    └── data/                   # ที่เก็บฐานข้อมูล SQLite และภาพสำหรับลงทะเบียน
```

---

## 🚀 เริ่มต้นใช้งานและทดสอบ (Getting Started)

ระบบรองรับการรันและทดสอบ 2 รูปแบบ:

### ก. การรันและทดสอบบนคอมพิวเตอร์ทั่วไป (PC / Laptop)

สามารถพัฒนาและทดสอบระบบได้ทันทีก่อนนำไปลง Raspberry Pi 5:

```bash
cd kiosk

# 1. สร้าง Virtual Environment และเปิดใช้งาน
python3 -m venv venv
source venv/bin/activate

# 2. ติดตั้งแพ็กเกจ (สำหรับบนคอมพิวเตอร์สามารถใช้ opencv-python เพื่อแสดงภาพ GUI)
pip install -r requirements.txt
pip install opencv-python

# 3. เตรียมไฟล์คอนฟิก
cp .env.example .env

# 4. ทดสอบรันตรวจจับใบหน้าผ่านกล้อง Webcam (หรือภาพจำลอง)
python scripts/run_detector.py

# 5. หรือทดสอบเปิดหน้าจอ Kiosk UI เต็มรูปแบบ (PyQt5 Portrait 480x800)
python main.py
```

> ระบบจะตรวจพบว่าไม่ได้รันบน Pi 5 และจะเปิด **CPU Fallback Mode** พร้อมใช้ Webcam ของคอมพิวเตอร์หรือสร้างภาพจำลองอัตโนมัติ

---

### ข. การรันบน Raspberry Pi 5 + Hailo-8 (ฮาร์ดแวร์จริง)

> **ข้อกำหนดเบื้องต้น:**  
> ติดตั้งแพ็กเกจระบบ Hailo และ Picamera2 บน Raspberry Pi OS Bookworm:
> ```bash
> sudo apt update && sudo apt install -y hailo-all python3-picamera2
> ```

```bash
cd kiosk

# 1. ติดตั้งสภาพแวดล้อม (ดึง System Site Packages เพื่อใช้งาน hailo_platform และ picamera2)
chmod +x scripts/setup.sh
./scripts/setup.sh

# 2. ดาวน์โหลดโมเดลทางการของ Hailo-8 (SCRFD, ArcFace, YOLOv8n)
chmod +x scripts/download_models.sh
./scripts/download_models.sh

# 3. ตั้งค่าคอนฟิก Kiosk
nano .env

# 4. ทดสอบรันตรวจจับใบหน้าด้วยชิป Hailo-8 ผ่านกล้อง Camera Module 3
source venv/bin/activate
python scripts/run_detector.py

# 5. รันโปรแกรมหลักของ Kiosk เพื่อให้บริการลงเวลา
python main.py
```

---

### ค. การรันเซิร์ฟเวอร์กลางและแดชบอร์ด (Central Server & Dashboard)

#### รันเซิร์ฟเวอร์ด้วย Docker Compose
```bash
# คัดลอกและตั้งค่าคอนฟิกเซิร์ฟเวอร์ Backend
cp server/backend/.env.example server/backend/.env

# โหมดพัฒนา (ใช้ SQLite ในตัว สะดวกรวดเร็ว)
docker compose up -d

# โหมดใช้งานจริง (เปิดใช้บริการ PostgreSQL เพิ่มเติม)
docker compose --profile prod up -d
```

#### การรันแบบแยกโหมดพัฒนา (Standalone Development)
```bash
# 1. รัน Backend API (FastAPI)
cd server/backend
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8443 --reload

# 2. รัน Frontend Dashboard (React + Vite)
cd ../frontend
npm install
npm run dev
# เข้าใช้งานผ่านเบราว์เซอร์: http://localhost:5173/dashboard/
```

---

## 🔒 ความปลอดภัยและนโยบายความเป็นส่วนตัว (PDPA Compliance)

* **ไม่จัดเก็บภาพถ่ายใบหน้า:** จัดเก็บเฉพาะเวกเตอร์ตัวเลข (Embedding 512 มิติ) เท่านั้น
* **การลงทะเบียนผ่านเว็บ:** ภาพถ่ายที่ส่งจากแดชบอร์ดจะถูกพักไว้ชั่วคราว และจะถูกส่งไปยังชิป Hailo เพื่อแปลงเป็นเวกเตอร์ก่อนจะถูกลบทำลายทันที (TTL ไม่เกิน 24 ชั่วโมง)
* **สิทธิ์ในการลบข้อมูล (Right to Erasure):** เมื่อลบพนักงานออกจากเซิร์ฟเวอร์ ระบบจะกระจายคำสั่ง Soft Delete ไปยัง Kiosk ทุกเครื่องเพื่อนำเวกเตอร์ออกจากฐานข้อมูลท้องถิ่น
* **การสื่อสารปลอดภัย:** ส่งข้อมูลผ่านโปรโตคอล HTTPS ภายในเครือข่ายองค์กร พร้อมระบบยืนยันสิทธิ์ API Key รายอุปกรณ์

---

## 📅 แผนการดำเนินงาน 16 สัปดาห์ (16-Week Roadmap)

| สัปดาห์ | รายละเอียดงาน | สถานะ |
|---|---|:---:|
| **1-2** | กำหนดขอบเขตโครงการ, ประกอบและตั้งค่า Pi 5 / Hailo-8 / Camera / OS, วางโครงสร้างโปรเจกต์ | ✅ เรียบร้อย |
| **3-4** | ติดตั้ง Face Detection (SCRFD) และ 5-Point Alignment บนชิป Hailo-8, สคริปต์ตรวจจับสด, Fallback บนคอมพิวเตอร์ | ✅ เรียบร้อย |
| **5-6** | ติดตั้ง Feature Extraction (ArcFace MobileFaceNet), ลงทะเบียนใบหน้าที่ Kiosk, ฐานข้อมูล SQLite WAL | ✅ เรียบร้อย |
| **7-8** | ปรับจูน Cosine Similarity Threshold, ระบบเปรียบเทียบใบหน้า Matcher, ส่งมอบ MVP กลางภาค | ✅ เรียบร้อย |
| **8-9** | พัฒนา FastAPI REST API (Python), Database Schema (PostgreSQL/SQLite), Docker Compose | ✅ เรียบร้อย |
| **9-10** | พัฒนา PyQt5 Touchscreen UI สำหรับจอแสดงผล 7 นิ้วแนวตั้ง (Portrait 480x800), ระบบไร้สัมผัส First-In Last-Out | ✅ เรียบร้อย |
| **10-11** | พัฒนาระบบ Offline-first Sync (Outbox Worker, User Pull Sync, Heartbeat, Timezone Reconciliation) | ✅ เรียบร้อย |
| **11-13** | พัฒนา แดชบอร์ดกลางด้วย React 18 + Vite (Devices, Users, Attendance Log, Analytics, Audit) | ✅ เรียบร้อย |
| **13-14** | การทดสอบรวมระบบจริง (End-to-End Test, Latency, ทดสอบตัดสัญญาณเครือข่ายแล้วกู้คืน) | 🟡 กำลังดำเนินการ |
| **14-15** | การทดสอบความเสถียร 24-48 ชม., จัดทำคู่มือการติดตั้งและการใช้งานระบบ, จัดทำรายงาน | แผนงานถัดไป |
| **16** | นำเสนอโครงงานสหกิจศึกษาและส่งมอบระบบ | แผนงานถัดไป |

---

## ⚠️ ปัญหาที่ทราบแล้วและสิ่งที่ต้องแก้ไขก่อน Production (Known Issues & Future Improvements)

### 🔴 Critical — ต้องแก้ก่อน Deploy จริง

| # | ปัญหา | ไฟล์ | รายละเอียด |
|---|--------|------|-----------|
| 1 | **Hardcoded Default Password & API Key** | `server/backend/app/main.py` L28, L40 | Seed ค่าเริ่มต้น `admin1234` และ API key `changeme-...` — ต้องบังคับเปลี่ยนตอน first run หรืออ่านจาก env |
| 2 | **JWT Secret ใช้ค่า Dev** | `server/backend/app/security.py` L19 | `JWT_SECRET` fallback เป็น `"dev-secret-change-me"` — ควร crash ถ้าไม่ set env |
| 3 | **CORS เปิด `allow_origins=["*"]`** | `server/backend/app/main.py` L71 | อนุญาตทุก origin + `allow_credentials=True` — ควร lock เฉพาะ origin ของ dashboard |

### 🟡 Should Fix — ควรแก้เพื่อความเสถียร

| # | ปัญหา | ไฟล์ | รายละเอียด |
|---|--------|------|-----------|
| 4 | **`verify=False` ทุก HTTPS request** | `outbox_worker.py`, `user_sync.py`, `heartbeat.py` | ปิด SSL verification 4 จุด — เสี่ยง MITM ควรใช้ self-signed cert + `verify="/path/to/ca.pem"` |
| 5 | **ไม่มี Rate Limiting** | Server API ทั้งหมด | ไม่มี rate limit → เสี่ยง brute-force login — ควรเพิ่ม `slowapi` |
| 6 | **Outbox ไม่มี Max Retry / Dead Letter** | `outbox_worker.py` | Event ที่ sync ไม่สำเร็จจะลองซ้ำไม่จำกัด — ควรเพิ่ม `retry_count` + dead-letter |
| 7 | **CPU Fallback ใช้ Random Embedding** | `detector.py` L260-L262 | ถ้าไม่มี ArcFace ONNX จะสร้าง embedding สุ่ม — ควรแจ้งเตือนให้ชัดเจนบน UI |

### 🔵 Improvement — ปรับปรุงเพิ่มเติม

| # | ปัญหา | รายละเอียด |
|---|--------|-----------|
| 8 | **Test Coverage น้อย** | มีแค่ 3 test files — ขาด test สำหรับ auth, face detection, enrollment, frontend |
| 9 | **Server ยังไม่มี HTTPS จริง** | Port 8443 แต่ไม่มี `ssl_keyfile` — ควรเพิ่ม cert หรือใช้ reverse proxy |
| 10 | **`seed_defaults()` รันทุกครั้งที่ start** | เป็น side-effect ตอน import — ควรย้ายเป็น CLI command |

### ⚪ Nice to Have

| # | ปัญหา | รายละเอียด |
|---|--------|-----------|
| 11 | **ไม่มี Log Rotation** | Kiosk ไม่มี config เขียน log ลงไฟล์หรือ rotate |
| 12 | **Dashboard ไม่มี Realtime Update** | ใช้ REST poll — ยังไม่มี WebSocket สำหรับ live update |
| 13 | **ยังไม่มี Deployment Script สำหรับ Kiosk** | ไม่มี systemd service file หรือ auto-start script สำหรับ Pi |
