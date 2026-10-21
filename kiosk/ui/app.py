"""
PyQt5 Touchscreen UI for Kiosk (Portrait / จอแนวตั้ง 480×800).
Minimalist Design: แสดงเฉพาะ เวลา, กล้องสด, และผลการสแกน (ใครสแกน / ไม่พบข้อมูล)
"""
import sys
import logging
from datetime import datetime
import numpy as np

from PyQt5.QtCore import Qt, QTimer, pyqtSlot
from PyQt5.QtGui import QImage, QPixmap, QFont
from PyQt5.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel,
    QVBoxLayout, QHBoxLayout, QFrame, QDialog, QLineEdit, QMessageBox, QPushButton
)

from alignment import align_face
from workers.recognition_worker import RecognitionWorker

logger = logging.getLogger(__name__)

# Minimalist Dark Theme
DARK_BG = "#12181f"
CARD_BG = "#1e272e"
BORDER_COLOR = "#2d3436"
ACCENT_GREEN = "#00b894"
ACCENT_RED = "#d63031"
ACCENT_BLUE = "#0984e3"
TEXT_PRIMARY = "#f5f6fa"
TEXT_SECONDARY = "#a4b0be"

THAI_MONTHS = [
    "", "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
    "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"
]


class MainWindow(QMainWindow):
    """
    Minimalist Kiosk Screen:
    1. เวลา (Digital Clock + Date)
    2. หน้าจอกล้อง (Live Camera Feed)
    3. ผลการสแกน (ชื่อคนสแกน / ไม่พบข้อมูลใบหน้า)
    """
    def __init__(self, config, camera, detector, database):
        super().__init__()
        self.config = config
        self.camera = camera
        self.detector = detector
        self.db = database
        self.current_direction = config.device_direction

        self.setWindowTitle("Face Attendance Kiosk")
        self.setStyleSheet(f"background-color: {DARK_BG}; color: {TEXT_PRIMARY};")

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

        layout = QVBoxLayout(central)
        layout.setContentsMargins(8, 6, 8, 6)
        layout.setSpacing(6)

        # ── 1. ส่วนเวลา (Clock & Date - แถวแนวนอนกะทัดรัด) ────────
        time_card = QFrame()
        time_card.setStyleSheet(
            f"background-color: {CARD_BG}; border-radius: 10px;"
        )
        t_layout = QHBoxLayout(time_card)
        t_layout.setContentsMargins(14, 6, 14, 6)

        self.clock_label = QLabel("00:00:00")
        self.clock_label.setFont(QFont("Monospace", 22, QFont.Bold))
        self.clock_label.setStyleSheet(f"color: {TEXT_PRIMARY};")
        t_layout.addWidget(self.clock_label)

        t_layout.addStretch()

        self.date_label = QLabel("")
        self.date_label.setFont(QFont("Sans", 12))
        self.date_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        t_layout.addWidget(self.date_label)

        layout.addWidget(time_card)

        # ── 2. หน้าจอกล้อง (Live Camera Preview - ขยายสูงเต็มตา) ──
        self.cam_frame = QFrame()
        self.cam_frame.setStyleSheet(
            f"background-color: #000000; border: 3px solid {BORDER_COLOR}; border-radius: 14px;"
        )
        c_layout = QVBoxLayout(self.cam_frame)
        c_layout.setContentsMargins(2, 2, 2, 2)

        self.cam_label = QLabel("กำลังเริ่มกล้อง...")
        self.cam_label.setAlignment(Qt.AlignCenter)
        self.cam_label.setStyleSheet("border-radius: 12px; background-color: #000000;")
        c_layout.addWidget(self.cam_label)

        # ให้กล้องขยายพื้นที่แนวตั้งสูงสุด (stretch=1)
        layout.addWidget(self.cam_frame, stretch=1)

        # ── 3. ด้านล่าง: ชื่อ + เวลา และ ปุ่มลงทะเบียน ──────────────
        bottom_box = QWidget()
        b_layout = QVBoxLayout(bottom_box)
        b_layout.setContentsMargins(4, 2, 4, 4)
        b_layout.setSpacing(4)

        # ชื่อพนักงานที่สแกน
        self.name_label = QLabel("พร้อมสแกนใบหน้า")
        self.name_label.setFont(QFont("Sans", 20, QFont.Bold))
        self.name_label.setAlignment(Qt.AlignCenter)
        self.name_label.setWordWrap(True)
        b_layout.addWidget(self.name_label)

        # เวลาบันทึก
        self.status_detail_label = QLabel("")
        self.status_detail_label.setFont(QFont("Sans", 13))
        self.status_detail_label.setAlignment(Qt.AlignCenter)
        self.status_detail_label.setStyleSheet(f"color: {TEXT_SECONDARY};")
        b_layout.addWidget(self.status_detail_label)

        # ปุ่มลงทะเบียนใบหน้า
        self.enroll_btn = QPushButton("📸 ลงทะเบียนใบหน้า")
        self.enroll_btn.setFont(QFont("Sans", 12, QFont.Bold))
        self.enroll_btn.setMinimumHeight(40)
        self.enroll_btn.setStyleSheet(
            f"background-color: {ACCENT_BLUE}; color: white; border-radius: 8px; border: none; padding: 4px;"
        )
        self.enroll_btn.clicked.connect(self._open_enrollment)
        b_layout.addWidget(self.enroll_btn)

        layout.addWidget(bottom_box)

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

    @pyqtSlot(QImage)
    def _update_frame(self, qimg):
        target_size = self.cam_label.size()
        tw, th = target_size.width(), target_size.height()

        if tw <= 0 or th <= 0:
            return

        qw, qh = qimg.width(), qimg.height()
        target_ratio = tw / th
        source_ratio = qw / qh

        # Center-Crop: ตัดส่วนเกินซ้าย-ขวาออก เพื่อให้ได้อัตราส่วนแนวตั้งโดยหน้าไม่ยืด
        if source_ratio > target_ratio:
            crop_w = int(qh * target_ratio)
            crop_h = qh
            crop_x = (qw - crop_w) // 2
            crop_y = 0
        else:
            crop_w = qw
            crop_h = int(qw / target_ratio)
            crop_x = 0
            crop_y = (qh - crop_h) // 2

        cropped = qimg.copy(crop_x, crop_y, crop_w, crop_h)
        pix = QPixmap.fromImage(cropped).scaled(
            target_size, Qt.IgnoreAspectRatio, Qt.SmoothTransformation
        )
        self.cam_label.setPixmap(pix)

    @pyqtSlot(str, float, str)
    def _on_recognized(self, name, conf, direction):
        """เมื่อสแกนเจอพนักงาน"""
        dir_text = "เข้างาน" if direction == "IN" else "ออกงาน"
        now_time = datetime.now().strftime("%H:%M:%S")

        # กรอบกล้องเปลี่ยนเป็นสีเขียว
        self.cam_frame.setStyleSheet(f"background-color: #000; border: 3px solid {ACCENT_GREEN}; border-radius: 14px;")

        self.name_label.setText(f"✅ {name}")
        self.name_label.setStyleSheet(f"color: {ACCENT_GREEN};")

        self.status_detail_label.setText(f"บันทึก: {dir_text} • เวลา {now_time}")
        self.status_detail_label.setStyleSheet("color: #ffffff; font-weight: bold;")

        QTimer.singleShot(4000, self._reset_status)

    @pyqtSlot()
    def _on_unknown(self):
        """เมื่อตรวจจับใบหน้าแล้วไม่พบในระบบ"""
        if "พร้อมสแกน" in self.name_label.text():
            # กรอบกล้องเปลี่ยนเป็นสีแดง
            self.cam_frame.setStyleSheet(f"background-color: #000; border: 3px solid {ACCENT_RED}; border-radius: 14px;")

            self.name_label.setText("❌ ไม่พบข้อมูลใบหน้า")
            self.name_label.setStyleSheet(f"color: {ACCENT_RED};")

            self.status_detail_label.setText("ไม่พบข้อมูลในระบบ")
            self.status_detail_label.setStyleSheet(f"color: {TEXT_SECONDARY};")

            QTimer.singleShot(3000, self._reset_status)

    def _reset_status(self):
        """รีเซ็ตกลับสู่สถานะพร้อมสแกน"""
        self.cam_frame.setStyleSheet(f"background-color: #000; border: 3px solid {BORDER_COLOR}; border-radius: 14px;")

        self.name_label.setText("พร้อมสแกนใบหน้า")
        self.name_label.setStyleSheet(f"color: {TEXT_PRIMARY};")

        self.status_detail_label.setText("")

    def keyPressEvent(self, event):
        """กดปุ่ม E บนคีย์บอร์ดเพื่อเปิดหน้าต่างลงทะเบียนใบหน้า (Admin Shortcut)"""
        if event.key() == Qt.Key_E:
            self._open_enrollment()
        else:
            super().keyPressEvent(event)

    def _open_enrollment(self):
        self.worker.stop()
        diag = EnrollmentDialog(self.camera, self.detector, self.db, self.config, self)
        diag.exec_()
        self._start_worker()

    def closeEvent(self, event):
        if hasattr(self, 'worker') and self.worker:
            self.worker.stop()
        event.accept()


class EnrollmentDialog(QDialog):
    """หน้าต่างลงทะเบียนใบหน้า (เปิดผ่าน Shortcut หรือ Admin)"""
    def __init__(self, camera, detector, database, config, parent=None):
        super().__init__(parent)
        self.camera = camera
        self.detector = detector
        self.db = database
        self.config = config
        self.captured_embeddings = []

        self.setWindowTitle("ลงทะเบียนใบหน้าพนักงานใหม่")
        self.setStyleSheet(f"background-color: {DARK_BG}; color: {TEXT_PRIMARY};")
        self.resize(440, 700)
        self._build_ui()

        self._preview_timer = QTimer(self)
        self._preview_timer.timeout.connect(self._update_preview)
        self._preview_timer.start(50)

    def _build_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(14, 14, 14, 14)
        layout.setSpacing(10)

        title = QLabel("📸 ลงทะเบียนใบหน้าพนักงานใหม่")
        title.setFont(QFont("Sans", 15, QFont.Bold))
        title.setAlignment(Qt.AlignCenter)
        layout.addWidget(title)

        form_frame = QFrame()
        form_frame.setStyleSheet(f"background-color: {CARD_BG}; border-radius: 8px; padding: 6px;")
        f_layout = QVBoxLayout(form_frame)
        f_layout.setSpacing(6)

        self.name_input = QLineEdit()
        self.name_input.setPlaceholderText("ชื่อ-นามสกุล")
        self.name_input.setStyleSheet("background: #12181f; color: white; padding: 8px; border-radius: 6px; font-size: 13px;")
        f_layout.addWidget(self.name_input)

        self.emp_id_input = QLineEdit()
        self.emp_id_input.setPlaceholderText("รหัสพนักงาน (เช่น EMP001)")
        self.emp_id_input.setStyleSheet("background: #12181f; color: white; padding: 8px; border-radius: 6px; font-size: 13px;")
        f_layout.addWidget(self.emp_id_input)
        layout.addWidget(form_frame)

        self.preview_label = QLabel("กำลังโหลดภาพกล้อง...")
        self.preview_label.setAlignment(Qt.AlignCenter)
        self.preview_label.setMinimumSize(400, 280)
        self.preview_label.setStyleSheet(
            f"border: 2px dashed {ACCENT_BLUE}; border-radius: 10px; background-color: {CARD_BG};"
        )
        layout.addWidget(self.preview_label, stretch=1)

        self.progress_label = QLabel(f"📷 ถ่ายภาพ: 0 / {self.config.min_enroll_images}")
        self.progress_label.setFont(QFont("Sans", 13, QFont.Bold))
        self.progress_label.setAlignment(Qt.AlignCenter)
        self.progress_label.setStyleSheet("color: #fdcb6e;")
        layout.addWidget(self.progress_label)

        self.capture_btn = QPushButton("📷 กดถ่ายภาพ")
        self.capture_btn.setFont(QFont("Sans", 13, QFont.Bold))
        self.capture_btn.setMinimumHeight(48)
        self.capture_btn.setStyleSheet(f"background-color: {ACCENT_BLUE}; color: white; border-radius: 8px;")
        self.capture_btn.clicked.connect(self._on_capture)
        layout.addWidget(self.capture_btn)

        cancel_btn = QPushButton("ยกเลิก")
        cancel_btn.setMinimumHeight(38)
        cancel_btn.setStyleSheet("background-color: #636e72; color: white; border-radius: 8px;")
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
            QMessageBox.warning(self, "พบหลายใบหน้า", "พบมากกว่า 1 ใบหน้า กรุณายืนเพียงคนเดียว")
            return

        landmarks = faces[0].get("landmarks", [])
        if len(landmarks) < 5:
            QMessageBox.warning(self, "คุณภาพไม่พอ", "ไม่พบ 5 จุดสำคัญบนใบหน้า กรุณาลองใหม่")
            return

        aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
        emb = self.detector.extract_embedding(aligned)
        self.captured_embeddings.append(emb)

        count = len(self.captured_embeddings)
        self.progress_label.setText(f"📷 ถ่ายภาพ: {count} / {self.config.min_enroll_images}")

        if count >= self.config.min_enroll_images:
            self._preview_timer.stop()
            self.capture_btn.setEnabled(False)
            self.capture_btn.setText("⏳ กำลังสร้างเวกเตอร์...")

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


def create_app(config, camera, detector, database):
    app = QApplication(sys.argv)
    app.setApplicationName("FaceAttendance")
    app.setFont(QFont("Sans", 11))
    window = MainWindow(config, camera, detector, database)
    return app, window
