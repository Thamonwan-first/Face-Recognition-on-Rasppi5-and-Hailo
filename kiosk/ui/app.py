"""
PyQt5 Touchscreen UI for Kiosk (Portrait / จอแนวตั้ง 480×800 for Raspberry Pi 7" display).
"""
import sys
import logging
from datetime import datetime
import numpy as np

from PyQt5.QtCore import Qt, QTimer, pyqtSlot
from PyQt5.QtGui import QImage, QPixmap, QFont
from PyQt5.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel, QPushButton,
    QVBoxLayout, QHBoxLayout, QFrame, QDialog, QLineEdit, QMessageBox,
)

from alignment import align_face
from metrics import MetricsTracker
from workers.recognition_worker import RecognitionWorker

logger = logging.getLogger(__name__)

# Modern Dark Kiosk Palette
DARK_BG = "#1e272e"
CARD_BG = "#2d3436"
BORDER_COLOR = "#3d4852"
ACCENT_GREEN = "#00b894"
ACCENT_RED = "#d63031"
ACCENT_BLUE = "#0984e3"
ACCENT_AMBER = "#fdcb6e"
TEXT_PRIMARY = "#dfe6e9"
TEXT_SECONDARY = "#b2bec3"

THAI_MONTHS = [
    "", "ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.",
    "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."
]


class EnrollmentDialog(QDialog):
    """Portrait face enrollment dialog (3-5 photos)."""
    def __init__(self, camera, detector, database, config, parent=None):
        super().__init__(parent)
        self.camera = camera
        self.detector = detector
        self.db = database
        self.config = config
        self.captured_embeddings = []

        self.setWindowTitle("ลงทะเบียนใบหน้าพนักงาน")
        self.setStyleSheet(f"background-color: {DARK_BG}; color: {TEXT_PRIMARY};")
        self.resize(440, 720)
        self._build_ui()

        self._preview_timer = QTimer(self)
        self._preview_timer.timeout.connect(self._update_preview)
        self._preview_timer.start(50)

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(12, 12, 12, 12)
        layout.setSpacing(10)

        # Header Title
        title = QLabel("📸 ลงทะเบียนใบหน้าใหม่")
        title.setFont(QFont("Sans", 15, QFont.Bold))
        title.setAlignment(Qt.AlignCenter)
        layout.addWidget(title)

        # Inputs Form
        form_frame = QFrame()
        form_frame.setStyleSheet(f"background-color: {CARD_BG}; border-radius: 8px; padding: 6px;")
        f_layout = QVBoxLayout(form_frame)
        f_layout.setSpacing(6)

        self.name_input = QLineEdit()
        self.name_input.setPlaceholderText("ชื่อ-นามสกุล (ภาษาไทยหรืออังกฤษ)")
        self.name_input.setStyleSheet("background: #1e272e; color: white; padding: 8px; border-radius: 6px; font-size: 13px;")
        f_layout.addWidget(self.name_input)

        self.emp_id_input = QLineEdit()
        self.emp_id_input.setPlaceholderText("รหัสพนักงาน (เช่น EMP001)")
        self.emp_id_input.setStyleSheet("background: #1e272e; color: white; padding: 8px; border-radius: 6px; font-size: 13px;")
        f_layout.addWidget(self.emp_id_input)
        layout.addWidget(form_frame)

        # Live Camera Preview (Portrait Card)
        self.preview_label = QLabel("กำลังโหลดภาพกล้อง...")
        self.preview_label.setAlignment(Qt.AlignCenter)
        self.preview_label.setMinimumSize(400, 280)
        self.preview_label.setStyleSheet(
            f"border: 2px dashed {ACCENT_BLUE}; border-radius: 10px; background-color: {CARD_BG};"
        )
        layout.addWidget(self.preview_label, stretch=1)

        # Progress and Guide
        guide_frame = QFrame()
        guide_frame.setStyleSheet(f"background-color: {CARD_BG}; border-radius: 8px; padding: 8px;")
        g_layout = QVBoxLayout(guide_frame)
        g_layout.setSpacing(4)

        self.progress_label = QLabel(f"📷 ถ่ายภาพ: 0 / {self.config.min_enroll_images}")
        self.progress_label.setFont(QFont("Sans", 13, QFont.Bold))
        self.progress_label.setAlignment(Qt.AlignCenter)
        self.progress_label.setStyleSheet(f"color: {ACCENT_AMBER};")
        g_layout.addWidget(self.progress_label)

        self.hint_label = QLabel("คำแนะนำ: หันหน้าตรง ➔ ก้มเล็กน้อย ➔ เอียงข้างเล็กน้อย")
        self.hint_label.setFont(QFont("Sans", 10))
        self.hint_label.setAlignment(Qt.AlignCenter)
        self.hint_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        g_layout.addWidget(self.hint_label)
        layout.addWidget(guide_frame)

        # Action Buttons
        self.capture_btn = QPushButton("📷 กดถ่ายภาพบันทึกใบหน้า")
        self.capture_btn.setFont(QFont("Sans", 13, QFont.Bold))
        self.capture_btn.setMinimumHeight(48)
        self.capture_btn.setStyleSheet(
            f"background-color: {ACCENT_BLUE}; color: white; border-radius: 8px; border: none;"
        )
        self.capture_btn.clicked.connect(self._on_capture)
        layout.addWidget(self.capture_btn)

        cancel_btn = QPushButton("ยกเลิก")
        cancel_btn.setFont(QFont("Sans", 11))
        cancel_btn.setMinimumHeight(38)
        cancel_btn.setStyleSheet("background-color: #636e72; color: white; border-radius: 8px; border: none;")
        cancel_btn.clicked.connect(self.reject)
        layout.addWidget(cancel_btn)

    def _update_preview(self):
        frame_rgb = self.camera.capture()
        if frame_rgb is None:
            return
        h, w, ch = frame_rgb.shape
        qimg = QImage(frame_rgb.data, w, h, ch * w, QImage.Format_RGB888).copy()
        pix = QPixmap.fromImage(qimg).scaled(self.preview_label.size(), Qt.KeepAspectRatio, Qt.SmoothTransformation)
        self.preview_label.setPixmap(pix)

    def _on_capture(self):
        name = self.name_input.text().strip()
        emp_id = self.emp_id_input.text().strip()
        if not name or not emp_id:
            QMessageBox.warning(self, "แจ้งเตือน", "กรุณากรอกชื่อและรหัสพนักงานให้ครบถ้วนก่อนถ่ายภาพ")
            return

        frame_rgb = self.camera.capture()
        if frame_rgb is None:
            return

        faces = self.detector.detect_faces(frame_rgb)
        if not faces:
            QMessageBox.warning(self, "ไม่พบใบหน้า", "ไม่พบใบหน้าในกรอบกล้อง กรุณาขยับเข้ามาใกล้ขึ้น")
            return
        if len(faces) > 1:
            QMessageBox.warning(self, "พบหลายใบหน้า", "พบมากกว่า 1 ใบหน้า กรุณายืนเพียงคนเดียวในกรอบ")
            return

        landmarks = faces[0].get("landmarks", [])
        if len(landmarks) < 5:
            QMessageBox.warning(self, "คุณภาพไม่พอ", "ไม่พบ 5 จุดสำคัญบนใบหน้า กรุณามองตรงแล้วลองใหม่")
            return

        aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
        emb = self.detector.extract_embedding(aligned)
        self.captured_embeddings.append(emb)

        count = len(self.captured_embeddings)
        self.progress_label.setText(f"📷 ถ่ายภาพ: {count} / {self.config.min_enroll_images}")

        # Update hints for next shot
        if count == 1:
            self.hint_label.setText("ท่าถัดไป: ก้มศีรษะลงเล็กน้อย หรือมองเงยขึ้นเล็กน้อย")
        elif count == 2:
            self.hint_label.setText("ท่าถัดไป: เอียงใบหน้าไปทางซ้ายหรือขวาเล็กน้อย")

        if count >= self.config.min_enroll_images:
            self._preview_timer.stop()
            self.capture_btn.setEnabled(False)
            self.capture_btn.setText("⏳ กำลังสร้างเวกเตอร์ 512 มิติ...")

            mean_emb = np.mean(self.captured_embeddings, axis=0)
            norm = np.linalg.norm(mean_emb)
            if norm > 0:
                mean_emb = mean_emb / norm

            try:
                self.db.register_user(name, emp_id, mean_emb, device_id=self.config.device_id)
                QMessageBox.information(self, "สำเร็จ", f"ลงทะเบียนพนักงาน '{name}' เรียบร้อยแล้ว!")
                self.accept()
            except Exception as e:
                QMessageBox.critical(self, "ข้อผิดพลาด", f"บันทึกข้อมูลล้มเหลว: {e}")
                self.reject()

    def closeEvent(self, event):
        self._preview_timer.stop()
        event.accept()


class MainWindow(QMainWindow):
    """
    Portrait Touchscreen UI (480×800) for Raspberry Pi 5 + 7" Touchscreen.
    Designed for vertical kiosk enclosure with big touch-friendly buttons.
    """
    def __init__(self, config, camera, detector, database):
        super().__init__()
        self.config = config
        self.camera = camera
        self.detector = detector
        self.db = database
        self.current_direction = config.device_direction

        self.setWindowTitle("ระบบลงเวลาเข้า-ออกงาน — Edge AI Kiosk (Portrait)")
        self.setStyleSheet(f"background-color: {DARK_BG}; color: {TEXT_PRIMARY};")

        # Set portrait screen size (480x800)
        if config.fullscreen:
            self.showFullScreen()
        else:
            self.resize(config.display_width, config.display_height)

        self._build_ui()
        self._start_clock()
        self._start_worker()

    def _build_ui(self):
        central = QWidget()
        self.setCentralWidget(central)

        # Main Vertical Layout for Portrait Screen
        layout = QVBoxLayout(central)
        layout.setContentsMargins(10, 8, 10, 8)
        layout.setSpacing(8)

        # ── 1. Top Header (Clock, Date, Device ID, Current Mode) ─────
        hdr = QFrame()
        hdr.setStyleSheet(
            f"background-color: {CARD_BG}; border: 1px solid {BORDER_COLOR}; border-radius: 10px; padding: 6px 10px;"
        )
        h_layout = QVBoxLayout(hdr)
        h_layout.setContentsMargins(4, 4, 4, 4)
        h_layout.setSpacing(2)

        # Row 1: Clock & Mode Badge
        row1 = QHBoxLayout()
        self.clock_label = QLabel("00:00:00")
        self.clock_label.setFont(QFont("Monospace", 20, QFont.Bold))
        self.clock_label.setStyleSheet(f"color: {ACCENT_BLUE};")
        row1.addWidget(self.clock_label)

        self.date_label = QLabel("")
        self.date_label.setFont(QFont("Sans", 11))
        self.date_label.setStyleSheet(f"color: {TEXT_SECONDARY}; padding-left: 6px;")
        row1.addWidget(self.date_label)
        row1.addStretch()

        self.mode_badge = QLabel("⚡ First-In, Last-Out")
        self.mode_badge.setFont(QFont("Sans", 10, QFont.Bold))
        self.mode_badge.setStyleSheet(
            f"background-color: #2b3a4a; color: {ACCENT_BLUE}; border: 1px solid {ACCENT_BLUE}; border-radius: 6px; padding: 4px 8px;"
        )
        row1.addWidget(self.mode_badge)
        h_layout.addLayout(row1)

        # Row 2: Device location
        self.dev_label = QLabel(f"📍 {self.config.device_id} • {self.config.device_name}")
        self.dev_label.setFont(QFont("Sans", 10))
        self.dev_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        h_layout.addWidget(self.dev_label)
        layout.addWidget(hdr)

        # ── 2. Camera Preview Container (Main Visual Frame) ─────────
        self.cam_frame = QFrame()
        self.cam_frame.setStyleSheet(
            f"background-color: {CARD_BG}; border: 2px solid {BORDER_COLOR}; border-radius: 12px;"
        )
        c_layout = QVBoxLayout(self.cam_frame)
        c_layout.setContentsMargins(6, 6, 6, 6)

        self.cam_label = QLabel("กำลังเริ่มกล้อง...")
        self.cam_label.setAlignment(Qt.AlignCenter)
        self.cam_label.setMinimumSize(440, 340)
        self.cam_label.setStyleSheet("border-radius: 8px; background-color: #000;")
        c_layout.addWidget(self.cam_label)
        layout.addWidget(self.cam_frame, stretch=1)

        # ── 3. Recognition Result Status Card ────────────────────────
        self.st_frame = QFrame()
        self.st_frame.setStyleSheet(
            f"background-color: {CARD_BG}; border: 1px solid {BORDER_COLOR}; border-radius: 10px; padding: 10px;"
        )
        s_layout = QVBoxLayout(self.st_frame)
        s_layout.setSpacing(4)
        s_layout.setContentsMargins(8, 8, 8, 8)

        self.name_label = QLabel("พร้อมสแกนใบหน้า")
        self.name_label.setFont(QFont("Sans", 18, QFont.Bold))
        self.name_label.setAlignment(Qt.AlignCenter)
        self.name_label.setWordWrap(True)
        s_layout.addWidget(self.name_label)

        sub_row = QHBoxLayout()
        self.conf_label = QLabel("")
        self.conf_label.setFont(QFont("Sans", 12))
        self.conf_label.setAlignment(Qt.AlignCenter)
        self.conf_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        sub_row.addWidget(self.conf_label)

        self.dir_label = QLabel("")
        self.dir_label.setFont(QFont("Sans", 13, QFont.Bold))
        self.dir_label.setAlignment(Qt.AlignCenter)
        sub_row.addWidget(self.dir_label)
        s_layout.addLayout(sub_row)

        self.time_label = QLabel("")
        self.time_label.setFont(QFont("Sans", 11))
        self.time_label.setAlignment(Qt.AlignCenter)
        self.time_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        s_layout.addWidget(self.time_label)

        layout.addWidget(self.st_frame)

        # ── 4. Contactless Guidance & Actions ────────────────────────
        ctrl_frame = QFrame()
        ctrl_frame.setStyleSheet(
            f"background-color: {CARD_BG}; border: 1px solid {BORDER_COLOR}; border-radius: 10px; padding: 6px;"
        )
        ctrl_layout = QVBoxLayout(ctrl_frame)
        ctrl_layout.setSpacing(6)
        ctrl_layout.setContentsMargins(6, 6, 6, 6)

        # Contactless System Info Box
        guide_box = QFrame()
        guide_box.setStyleSheet(
            "background-color: #1a252f; border: 1px dashed #34495e; border-radius: 8px; padding: 6px;"
        )
        g_layout = QVBoxLayout(guide_box)
        g_layout.setSpacing(2)
        g_layout.setContentsMargins(4, 4, 4, 4)

        g_title = QLabel("✨ ระบบไร้สัมผัส 100% (First-In, Last-Out)")
        g_title.setFont(QFont("Sans", 11, QFont.Bold))
        g_title.setAlignment(Qt.AlignCenter)
        g_title.setStyleSheet(f"color: {ACCENT_GREEN};")
        g_layout.addWidget(g_title)

        g_desc = QLabel("สแกนครั้งแรก = เข้างาน (IN) • ครั้งถัดไป = ออกงาน (OUT)")
        g_desc.setFont(QFont("Sans", 9))
        g_desc.setAlignment(Qt.AlignCenter)
        g_desc.setStyleSheet(f"color: {TEXT_SECONDARY};")
        g_layout.addWidget(g_desc)
        ctrl_layout.addWidget(guide_box)

        # Enroll Button (for new staff)
        enroll_btn = QPushButton("📸 ลงทะเบียนใบหน้าพนักงานใหม่")
        enroll_btn.setFont(QFont("Sans", 11))
        enroll_btn.setMinimumHeight(40)
        enroll_btn.setStyleSheet(
            f"background-color: {ACCENT_BLUE}; color: white; border-radius: 8px; border: none;"
        )
        enroll_btn.clicked.connect(self._open_enrollment)
        ctrl_layout.addWidget(enroll_btn)

        layout.addWidget(ctrl_frame)

        # ── 5. Bottom Telemetry Footer ───────────────────────────────
        ftr = QFrame()
        ftr.setStyleSheet(
            f"background-color: {CARD_BG}; border-radius: 6px; padding: 4px 8px;"
        )
        f_layout = QHBoxLayout(ftr)
        f_layout.setContentsMargins(4, 2, 4, 2)

        self.ai_label = QLabel(f"⚡ {self.detector.backend_name}")
        self.ai_label.setFont(QFont("Sans", 9))
        self.ai_label.setStyleSheet(f"color: {ACCENT_GREEN};")
        f_layout.addWidget(self.ai_label)
        f_layout.addStretch()

        self.conn_label = QLabel("🟢 Offline-First")
        self.conn_label.setFont(QFont("Sans", 9))
        f_layout.addWidget(self.conn_label)
        f_layout.addStretch()

        self.emp_count_label = QLabel("👥 พนักงาน: 0")
        self.emp_count_label.setFont(QFont("Sans", 9))
        self.emp_count_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        f_layout.addWidget(self.emp_count_label)
        layout.addWidget(ftr)

    def _start_clock(self):
        self._clock_timer = QTimer(self)
        self._clock_timer.timeout.connect(self._tick_clock)
        self._clock_timer.start(1000)
        self._tick_clock()

    def _tick_clock(self):
        now = datetime.now()
        self.clock_label.setText(now.strftime("%H:%M:%S"))
        thai_m = THAI_MONTHS[now.month] if 1 <= now.month <= 12 else ""
        self.date_label.setText(f"{now.day} {thai_m} {now.year + 543}")

    def _start_worker(self):
        self.worker = RecognitionWorker(self.camera, self.detector, self.db, self.config, self.current_direction)
        self.worker.frame_ready.connect(self._update_frame)
        self.worker.face_recognized.connect(self._on_recognized)
        self.worker.unknown_detected.connect(self._on_unknown)
        self.worker.start()
        self.emp_count_label.setText(f"👥 พนักงาน: {len(self.worker.gallery_cache)}")

    @pyqtSlot(QImage)
    def _update_frame(self, qimg):
        pix = QPixmap.fromImage(qimg).scaled(self.cam_label.size(), Qt.KeepAspectRatio, Qt.SmoothTransformation)
        self.cam_label.setPixmap(pix)

    @pyqtSlot(str, float, str)
    def _on_recognized(self, name, conf, direction):
        # Flash frame border green
        self.cam_frame.setStyleSheet(f"background-color: {CARD_BG}; border: 2px solid {ACCENT_GREEN}; border-radius: 12px;")
        self.name_label.setText(f"✅ {name}")
        self.name_label.setStyleSheet(f"color: {ACCENT_GREEN};")
        self.conf_label.setText(f"ความมั่นใจ: {conf:.1%}")
        if direction == "IN":
            self.dir_label.setText("⬆ เข้างานสำเร็จ (IN)")
            self.dir_label.setStyleSheet(f"color: {ACCENT_GREEN};")
        else:
            self.dir_label.setText("⬇ บันทึกออกงาน (OUT)")
            self.dir_label.setStyleSheet("color: #e17055;")
        self.time_label.setText(f"เวลา: {datetime.now().strftime('%H:%M:%S')}")
        QTimer.singleShot(4000, self._reset_status)

    @pyqtSlot()
    def _on_unknown(self):
        if "พร้อมสแกน" in self.name_label.text():
            self.cam_frame.setStyleSheet(f"background-color: {CARD_BG}; border: 2px solid {ACCENT_RED}; border-radius: 12px;")
            self.name_label.setText("❓ ไม่พบข้อมูลใบหน้า")
            self.name_label.setStyleSheet(f"color: {ACCENT_RED};")
            self.conf_label.setText("กรุณาลงทะเบียนก่อนใช้งาน")
            QTimer.singleShot(3000, self._reset_status)

    def _reset_status(self):
        self.cam_frame.setStyleSheet(f"background-color: {CARD_BG}; border: 2px solid {BORDER_COLOR}; border-radius: 12px;")
        self.name_label.setText("พร้อมสแกนใบหน้า")
        self.name_label.setStyleSheet(f"color: {TEXT_PRIMARY};")
        self.conf_label.setText("")
        self.dir_label.setText("")
        self.time_label.setText("")

    def _open_enrollment(self):
        self.worker.stop()
        diag = EnrollmentDialog(self.camera, self.detector, self.db, self.config, self)
        diag.exec_()
        self._start_worker()

    def closeEvent(self, event):
        if hasattr(self, 'worker') and self.worker:
            self.worker.stop()
        event.accept()


def create_app(config, camera, detector, database):
    app = QApplication(sys.argv)
    app.setApplicationName("FaceAttendance")
    app.setFont(QFont("Sans", 11))
    window = MainWindow(config, camera, detector, database)
    return app, window
