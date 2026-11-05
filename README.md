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
│  - Face Pipeline (Hailo-8)      │ <───────────────────────────── │  - Go REST API (Gin Clean Arch) │
│  - Local SQLite (WAL Mode)      │      Heartbeat & Data Sync     │  - PostgreSQL Database          │
│  - PyQt5 Touchscreen UI (Portrait) │                             │  - Caddy Reverse Proxy (TLS)    │
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
   * **AI Pipeline:** SCRFD (Face Detection & 5 Landmarks) ➔ 5-Point Affine Alignment (112×112) ➔ ArcFace MobileFaceNet (512-d Feature Extraction บน Hailo-8) ➔ Vectorized Matrix BLAS Cosine Matching
   * **Cross-Platform Fallback:** ตรวจจับชิป Hailo และกล้องอัตโนมัติ หากรันบนคอมพิวเตอร์จะสลับไปใช้ CPU Fallback และ Webcam/Mock Stream เพื่อให้ทดสอบโค้ดได้ทันที
   * **Offline-First Storage:** SQLite (WAL mode) เก็บ events, outbox, users และ embeddings พร้อม retention purge ลบข้อมูลซิงก์แล้วเกิน 7 วัน
   * **Sync Workers:** ทำงานเบื้องหลังแยกเธรด (Batch upload พร้อม Exponential Backoff, ดึงข้อมูลผู้ใช้ด้วย `since=<version>`, Heartbeat)
   * **Touchless Kiosk UI:** พัฒนาด้วย PyQt5 ออกแบบขนาดแนวตั้ง (Portrait) ล็อกหน้าจอแบบ Frameless Kiosk Mode ป้องกันการปัด/ย่อหน้าต่างหลุดสู่ Desktop, ระบบเพิกเฉยต่อการสัมผัส (Touch-proof), และระบบซ่อนลูกศรเมาส์อัตโนมัติ (Auto-Hide Cursor) เมื่ออยู่นิ่ง โดยมีคีย์ลัดเฉพาะแอดมิน `Ctrl + Q` สำหรับปิดโปรแกรม
   * **Cross-Platform Fallback:** ตรวจจับชิป Hailo และกล้องอัตโนมัติ หากรันบนคอมพิวเตอร์จะสลับไปใช้ CPU Fallback และ Webcam/Mock Stream เพื่อให้ทดสอบโค้ดได้ทันที
   * **Offline-First Storage:** SQLite (WAL mode) เก็บ events, outbox, users และ embeddings พร้อม retention purge ลบข้อมูลซิงก์แล้วเกิน 7 วัน
   * **Sync Workers:** ทำงานเบื้องหลังแยกเธรด (Batch upload พร้อม Exponential Backoff, ดึงข้อมูลผู้ใช้ด้วย `since=<version>`, Heartbeat พร้อมส่งรายงาน IP)

2. **Central Server (เซิร์ฟเวอร์กลาง On-Premise):**
   * **Backend API:** Go (Golang) โครงสร้าง **Clean Architecture** (Gin Framework) ทำงานร่วมกับฐานข้อมูล **PostgreSQL** แบบ Concurrency สูง
   * **Device Authentication:** ยืนยันตัวตนเครื่อง Kiosk ด้วย API Key เข้ารหัส Hash ด้วย SHA-256 พร้อม Index Lookup ความเร็วคงที่ $O(1)$
   * **Data Integrity & Reconciliation:** จับคู่เวลาเข้า-ออก (IN/OUT Matching) บนเซิร์ฟเวอร์ รองรับช่วงกะ/พักกลางวัน และ Idempotency ผ่าน UUID ของเหตุการณ์
   * **Single Database Engine:** ใช้ PostgreSQL เป็นฐานข้อมูลหลักเพียงระบบเดียว ป้องกันปัญหา Database Lock และรองรับ Kiosk ขยายตัวได้นับร้อยเครื่อง
   * **Deployment:** บรรจุในคอนเทนเนอร์ด้วย Docker Compose พร้อม Caddy ทำหน้าที่ Reverse Proxy และ TLS ภายใน

3. **Admin Dashboard (แดชบอร์ดกลาง):**
   * **Frontend:** React 18, Vite, React Router, React Query (@tanstack/react-query), Chart.js
   * **ฟังก์ชันการทำงาน:**
     * **รายการอุปกรณ์ (Devices):** ดูสถานะสดและที่อยู่ IP ของกล้องทุกตัว (ออนไลน์/ออฟไลน์, ที่อยู่ IP, FPS, อุณหภูมิ CPU, Outbox Pending) พร้อมระบบลงทะเบียนเครื่องใหม่และออก API Key
     * **ข้อมูลพนักงาน (Users):** จัดการข้อมูลพนักงาน, แยกแสดงวันเริ่มงานและวันสิ้นสุดสัญญาจ้างชัดเจน, พร้อมปุ่มลงทะเบียน/อัปเดตใบหน้า (อัปโหลด 1-5 ภาพส่งไปสร้าง Embedding บนชิป Hailo-8) ภายในหน้าเดียว
     * **ประวัติการลงเวลา (Attendance):** บันทึกประวัติเวลาเข้า-ออกงาน, คำนวณชั่วโมงทำงานรวม, กรองตามเครื่องสแกนและช่วงวันที่, และส่งออกรายงานเป็นไฟล์ CSV
     * **บันทึกประวัติการใช้งาน (Audit Log):** บันทึกประวัติการกระทำของผู้ดูแลระบบทั้งหมดเพื่อความโปร่งใส
     * **จัดการผู้ดูแลระบบ (Admins Management):** เพิ่ม/แก้ไข/ลบ บัญชีผู้ดูแลระบบ พร้อมระบบ **เปิด-ปิดสิทธิ์การเข้าถึงแต่ละเมนูได้อย่างอิสระ (Granular Menu Permissions)** รายบุคคล
     * **การตั้งค่าระบบและบัญชี (Settings):** เข้าถึงผ่านไอคอนฟันเฟือง ⚙️ ข้างบทบาทผู้ดูแลระบบ รวมศูนย์การเปลี่ยนรหัสผ่านของตนเอง และการตั้งค่าเวลาเข้า-ออกงานมาตรฐาน, ชั่วโมงขั้นต่ำ, และเกณฑ์ความแม่นยำใบหน้า (Cosine Threshold)
   * **ความเรียบง่ายและเป็นทางการ:** ส่วนติดต่อผู้ใช้เป็นภาษาไทยล้วน ปราศจากสัญลักษณ์อิโมจิและคำศัพท์ภาษาอังกฤษในวงเล็บ เข้าถึงผ่านเส้นทางตรง `/login`

---

## 📁 โครงสร้างโปรเจกต์ (Project Structure)

```text
.
├── Caddyfile                   # การตั้งค่า Caddy Reverse Proxy + Static Web Serving
├── docker-compose.yml          # คอนฟิกบริการ Server, PostgreSQL และ Caddy
├── document/                   ← โฟลเดอร์เอกสารและรายงานโครงงาน (.docx)
│   ├── project_proposal.docx   # รายงานข้อเสนอโครงงานฉบับสมบูรณ์
│   └── รายงานโครงงาน_ระบบบันทึกเวลาทำงานด้วยการรู้จำใบหน้า_5บท.docx # เล่มรายงานโครงงาน 5 บท
├── docs/                       # เอกสารประกอบโครงงานและคู่มือเพิ่มเติม
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
│   │   └── app.py              # หน้าจอลงเวลามินิมอลไร้สัมผัส แสดงผลนาฬิกา กล้อง และผลลัพธ์การสแกน
│   └── workers/                # เธรดเบื้องหลังแยกการทำงาน
│       ├── recognition_worker.py # ลูปจับภาพ + ประมวลผลใบหน้า + บันทึก Event
│       ├── outbox_worker.py    # ซิงก์ประวัติลงเวลาขึ้นเซิร์ฟเวอร์แบบ Exponential Backoff
│       ├── user_sync.py        # ซิงก์ข้อมูลพนักงานลงเครื่องด้วย version-based sync
│       └── heartbeat.py        # ส่งสถานะสุขภาพเครื่อง (CPU Temp, FPS, Outbox)
│
└── server/                     ← ฝั่งเซิร์ฟเวอร์องค์กร (Go + PostgreSQL)
    ├── backend/                # ซอร์สโค้ด API เซิร์ฟเวอร์กลาง (Go Clean Architecture)
    │   ├── main.go             # Composition Root (Dependency Injection) & Bootstrapping
    │   ├── internal/
    │   │   ├── entity/         # Core Business Entities (user, device, attendance, admin, errors)
    │   │   ├── ports/          # Interface Contracts (Repositories & Services)
    │   │   ├── usecase/        # Business Logic (Auth, User, Attendance, Sync, Device)
    │   │   ├── adapters/
    │   │   │   ├── repository/ # PostgreSQL Implementation (pgx/v5)
    │   │   │   └── handler/    # HTTP Web Handlers & Gin Router
    │   │   └── pkg/            # Utility helpers (Hasher, JWT Token Maker)
    │   ├── Dockerfile          # Multi-stage Docker build สำหรับ Go Binary
    │   ├── go.mod / go.sum     # รายการโมดูล Go
    │   └── .env.example        # ตัวอย่างคอนฟิก PostgreSQL, JWT และ Storage
    ├── frontend/               # หน้าเว็บแดชบอร์ดจัดการระบบ (React 18 + Vite)
    │   ├── src/
    │   │   ├── api.js          # HTTP Client พร้อมดึง JWT จาก localStorage
    │   │   ├── App.jsx         # กำหนด Layout และ Routes ทั้งหมด
    │   │   ├── main.jsx        # จุดเริ่มการเรนเดอร์ React
    │   │   ├── components/     # UI Components (Sidebar, StatusBadge, ProtectedRoute)
    │   │   └── pages/          # หน้าเว็บ (Devices, Users, Attendance, Enrollment, Audit, Admins, Login)
    │   ├── index.html
    │   ├── vite.config.js
    │   └── package.json
    ├── certs/                  # ใบรับรองความปลอดภัย TLS ภายใน
    └── data/                   # ที่เก็บภาพถ่ายชั่วคราวสำหรับลงทะเบียน (enrollment_images)
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

#### รันเซิร์ฟเวอร์ด้วย Docker Compose (แนะนำ)
```bash
# คัดลอกและตั้งค่าคอนฟิกเซิร์ฟเวอร์ Backend
cp server/backend/.env.example server/backend/.env

# รัน PostgreSQL + Go Backend Server + Caddy Reverse Proxy ในคำสั่งเดียว
docker compose up -d
```

#### การรันแบบแยกโหมดพัฒนา (Standalone Development)
```bash
# 1. รัน Backend API (Go Clean Architecture + PostgreSQL)
cd server/backend
# ตรวจสอบว่า PostgreSQL รันอยู่ (port 5432)
go run main.go
# หรือคอมไพล์เป็นไบนารี: go build -o bin/server main.go && ./bin/server

# 2. รัน Frontend Dashboard (React 18 + Vite)
cd ../frontend
npm install
npm run dev
# เข้าใช้งานผ่านเบราว์เซอร์: http://localhost:5173/login
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
| **5-6** | ติดตั้ง Feature Extraction (ArcFace MobileFaceNet), ระบบแปลงและจับคู่เวกเตอร์ใบหน้า, ฐานข้อมูล SQLite WAL | ✅ เรียบร้อย |
| **7-8** | ปรับจูน Cosine Similarity Threshold, ระบบเปรียบเทียบใบหน้า Matcher, ส่งมอบ MVP กลางภาค | ✅ เรียบร้อย |
| **8-9** | ออกแบบ Database Schema และสร้าง REST API สำหรับ Kiosk Data Sync | ✅ เรียบร้อย |
| **9-10** | พัฒนา PyQt5 Touchscreen UI สำหรับจอแสดงผล 7 นิ้วแนวตั้ง (Portrait 480x800), ระบบไร้สัมผัส First-In Last-Out | ✅ เรียบร้อย |
| **10-11** | พัฒนาระบบ Offline-first Sync (Outbox Worker, User Pull Sync, Heartbeat, Timezone Reconciliation) | ✅ เรียบร้อย |
| **11-13** | พัฒนา แดชบอร์ดกลางด้วย React 18 + Vite (Devices, Users, Attendance Log, Analytics, Audit) | ✅ เรียบร้อย |
| **13-14** | รีแฟคเตอร์ Backend สู่ **Go Clean Architecture + PostgreSQL ล้วน**, ปรับปรุง Big O & Security Audit | ✅ เรียบร้อย |
| **14-15** | พัฒนาระบบสัญญาจ้างพนักงาน (เริ่มงาน-หมดสัญญา), ระบบจัดการผู้ดูแลและสิทธิ์รายเมนู (Granular RBAC), ปรับปรุงความปลอดภัย, จัดทำเล่มรายงาน 5 บท (.docx) | ✅ เรียบร้อย |
| **16** | นำเสนอโครงงานสหกิจศึกษาและส่งมอบระบบ | แผนงานถัดไป |

---

## 🛡️ สถานะการปรับปรุงระบบและความปลอดภัย (Security & Performance Status)

### ✅ ปรับปรุงและแก้ไขเรียบร้อยแล้ว (Resolved)

| รายการ | การปรับปรุง | ผลลัพธ์ |
|---|---|---|
| **Device Auth Complexity** | เปลี่ยนจาก bcrypt O(N) loop มาใช้ **SHA-256 + Index** | ลดจาก $O(N)$ (~70ms ต่อ Kiosk) เหลือ $O(1)$ lookup ทันที ป้องกัน CPU 100% |
| **Attendance Multi-shift / Lunch Return** | กรองเฉพาะ `status = 'open'` ในการจับคู่เวลา | ไม่อ่านทับบันทึกเดิม พนักงานสแกนพักเที่ยงหรือทำงานกะย่อยบันทึกเวลาได้ถูกต้องครบถ้วน |
| **Database Architecture** | ย้ายเซิร์ฟเวอร์หลักมาใช้ **PostgreSQL ล้วน** | แก้ปัญหา SQLite Database Lock บน Server, รองรับ Kiosk และ Concurrency สูง |
| **Face Matcher Optimization** | เวกเตอร์ไรซ์ด้วย **NumPy BLAS Dot Product** | เปลี่ยนจากลูป $O(N)$ ใน Python เป็น Matrix Math ความเร็วสูง ใช้เวลา <1ms บนแกลเลอรีขนาดใหญ่ |
| **Kiosk Outbox Purge** | เพิ่ม Daily Cleanup ลบประวัติที่ซิงก์แล้วเกิน 7 วัน | ป้องกันฐานข้อมูล SQLite บน MicroSD Card ขยายตัวจนหน่วยความจำเต็ม |
| **User Sync N+1 Query** | รวม User ข้อมูลและไบต์ Embedding ด้วย `LEFT JOIN` | ซิงก์ข้อมูลพนักงานทั้งหมดได้ใน 1 Roundtrip |
| **JWT & Secret Enforcement** | บังคับระบุ `JWT_SECRET` ใน Production | เซิร์ฟเวอร์จะ Crash ทันทีหากไม่ตั้งค่าความปลอดภัย ป้องกันการปลอมแปลง Token |
| **Kiosk HTTPS Verification** | รองรับการตรวจสอบ CA Certificate ผ่านคอนฟิก `ssl_verify` | ป้องกันการโจมตีแบบ Man-in-the-Middle (MITM) ระหว่างส่งเวกเตอร์และเหตุการณ์ |
| **Contract Period Control** | เพิ่ม `start_date` และ `end_date` พร้อมระงับสิทธิ์ที่เซิร์ฟเวอร์ | พนักงานที่ยังไม่เริ่มงานหรือหมดสัญญาจะถูกระงับการสแกนหน้าอัตโนมัติ โดยข้อมูลยังคงอยู่ในระบบเพื่อต่อสัญญา |
| **Granular Menu Permissions** | ระบบสิทธิ์ผู้ดูแลแดชบอร์ดแบบเลือกเปิด-ปิดเมนูอิสระ (RBAC) | กำหนดสิทธิ์รายบุคคลผ่าน Checkbox ป้องกันระดับ Go Middleware (`403 Forbidden`) พร้อมฟังก์ชันเปลี่ยนรหัสผ่าน |
| **Routing & UI Modernization** | ปรับเส้นทางเข้าสู่ระบบตรงที่ `/login` และทำความสะอาดภาษา | เว็บไซต์สะอาดตา เป็นทางการ ไร้อิโมจิและวงเล็บภาษาอังกฤษ ตอบสนองรวดเร็ว |
| **Dashboard Realtime Update** | สตรีมข้อมูลสดผ่าน Server-Sent Events (SSE) `/api/realtime/stream` | อัปเดตสถิติและเหตุการณ์ลงเวลาทันทีแบบ Real-time โดยไม่ต้อง Polling ถี่ๆ ลดโหลดฐานข้อมูล |
| **Kiosk Systemd Service** | ไฟล์ `attendance-kiosk.service` และสคริปต์ติดตั้งอัตโนมัติ | รันแอป Kiosk ทันทีเมื่อเปิดเครื่อง Raspberry Pi พร้อม Auto-restart หากแอปปิดตัวลง |
| **Rate Limiting Middleware** | In-Memory Token Bucket ใน Go Gin | ป้องกันการโจมตี Brute-force และ Request Flooding (20 req/s, burst 40) พร้อมข้อยกเว้นสำหรับ Device API Key |
| **Device IP Auto-Detection** | ตรวจจับและบันทึก IP ของเครื่อง Kiosk อัตโนมัติจาก Heartbeat และ Request | แอดมินตรวจสอบที่อยู่ IP ของเครื่องสแกนทุกจุดผ่านแดชบอร์ดได้ทันที แม้เครื่องย้าย Wi-Fi |
| **Dynamic Work Time Settings** | หน้าต่างตั้งค่าระบบและบัญชีผู้ใช้รวมศูนย์ผ่านไอคอนฟันเฟือง ⚙️ | ปรับเวลาเข้า-ออกงานมาตรฐาน, ชม. ขั้นต่ำ และเกณฑ์ Cosine Threshold ใบหน้าผ่านหน้าเว็บได้โดยไม่ต้องแก้โค้ด |
| **Touchless Kiosk Lockdown** | โหมดไร้สัมผัส Frameless Kiosk Lock + Auto-Hide Cursor | ล็อกหน้าจอเต็มจอห้ามปัด/ย่อหลุดไป Desktop, เพิกเฉยต่อการแตะจอ, และซ่อนลูกศรเมาส์อัตโนมัติเมื่ออยู่นิ่ง |

### ⚪ แผนพัฒนาเพิ่มเติม (Future Enhancements)

- [ ] รองรับการแจ้งเตือนพนักงานผ่าน LINE Notify / Telegram เมื่อมีการสแกนบันทึกเวลา
- [ ] เพิ่มระบบสำรองฐานข้อมูล PostgreSQL อัตโนมัติไปยัง Cloud Storage / S3
