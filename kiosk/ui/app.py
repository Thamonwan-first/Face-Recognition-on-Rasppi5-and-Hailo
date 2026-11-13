"""
PyQt5 Touchscreen UI for Kiosk (Portrait / จอแนวตั้ง).
Full-Screen Borderless Camera View + Minimal Warm Cream Palette & Floating Card.
"""
import sys
import os
import time
import logging
from datetime import datetime
import numpy as np

from PyQt5.QtCore import Qt, QTimer, pyqtSlot
from PyQt5.QtGui import QImage, QPixmap, QFont, QColor, QFontDatabase, QPainter, QPainterPath, QBrush, QPen
from PyQt5.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel,
    QVBoxLayout, QHBoxLayout, QFrame, QDialog, QSizePolicy,
    QGraphicsDropShadowEffect
)

from workers.recognition_worker import RecognitionWorker

logger = logging.getLogger(__name__)

# Typography: Kanit Google Font (โหลดจาก kiosk/ui/fonts/ อัตโนมัติ)
FONT_FAMILY = "Kanit"


def init_fonts():
    """โหลดฟอนต์ Kanit เข้าสู่ระบบ QFontDatabase หากมีไฟล์อยู่ในโฟลเดอร์ fonts"""
    fonts_dir = os.path.join(os.path.dirname(__file__), "fonts")
    for font_file in ["Kanit-Regular.ttf", "Kanit-Medium.ttf", "Kanit-Bold.ttf"]:
        path = os.path.join(fonts_dir, font_file)
        if os.path.exists(path):
            QFontDatabase.addApplicationFont(path)


# Minimal Warm White & Cream Palette (โทนขาวครีม มินิมอล อบอุ่น เรียบหรู)
WARM_CREAM_BG = "#F9F7F2"       # สีพื้นหลังขาวครีม (Warm Off-White / Alabaster)
HEADER_BG = "#F3EFE8"           # สีแถบเวลาด้านบน (Soft Warm Sand Cream)
HEADER_BORDER = "#E5DFD4"       # เส้นขอบล่างแถบเวลา
CARD_BG = "rgba(255, 253, 249, 0.97)" # การ์ดป๊อปอัปขาวครีมละมุน
CARD_BORDER = "#DDD5C7"         # เส้นขอบการ์ดโทนอุ่น เรียบหรู
TEXT_PRIMARY = "#23201D"        # ข้อความหลัก สีชาร์โคลโทนอุ่น (Warm Deep Charcoal)
TEXT_SECONDARY = "#5C564F"      # ข้อความรอง สีเทาโทนอุ่น (Warm Taupe)
TEXT_MUTED = "#8A8277"          # ข้อความกำกับ สีทรายโทนอบอุ่น
ACCENT_SUCCESS = "#2E6B4D"      # สีเขียวใบไม้อบอุ่นสำหรับสถานะบันทึกสำเร็จ (Warm Sage Green)

THAI_DAYS = [
    "วันจันทร์", "วันอังคาร", "วันพุธ", "วันพฤหัสบดี", "วันศุกร์", "วันเสาร์", "วันอาทิตย์"
]

THAI_MONTHS = [
    "", "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
    "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม"
]


def make_circular_pixmap(image_path: str, size: int = 60) -> QPixmap | None:
    """แปลงรูปภาพจาก path ให้เป็นวงกลมขอบมนเรียบสวยงาม"""
    if not image_path or not os.path.exists(image_path):
        return None
    src = QPixmap(image_path)
    if src.isNull():
        return None
    scaled = src.scaled(size, size, Qt.KeepAspectRatioByExpanding, Qt.SmoothTransformation)
    out = QPixmap(size, size)
    out.fill(Qt.transparent)
    painter = QPainter(out)
    painter.setRenderHint(QPainter.Antialiasing, True)
    path = QPainterPath()
    path.addEllipse(0, 0, size, size)
    painter.setClipPath(path)
    x_off = (size - scaled.width()) // 2
    y_off = (size - scaled.height()) // 2
    painter.drawPixmap(x_off, y_off, scaled)
    painter.end()
    return out


def make_monogram_pixmap(name: str, size: int = 60) -> QPixmap:
    """สร้างวงกลมตัวอักษรย่อเมื่อไม่มีรูปโปรไฟล์"""
    out = QPixmap(size, size)
    out.fill(Qt.transparent)
    painter = QPainter(out)
    painter.setRenderHint(QPainter.Antialiasing, True)
    painter.setBrush(QBrush(QColor(234, 230, 222)))
    painter.setPen(QPen(QColor(216, 210, 198), 1.5))
    painter.drawEllipse(1, 1, size - 2, size - 2)
    painter.setPen(QPen(QColor(45, 42, 38)))
    painter.setFont(QFont(FONT_FAMILY, 16, QFont.Bold))
    initial = (name.strip()[0] if name.strip() else "?").upper()
    painter.drawText(0, 0, size, size, Qt.AlignCenter, initial)
    painter.end()
    return out


class AttendanceSuccessPopup(QDialog):
    """
    ป๊อปอัปแจ้งผลการลงเวลาสำเร็จ สไตล์มินิมอล โทนขาวครีม (Warm Minimalist Cream)
    เงาฟุ้งนุ่มนวล ขอบมนเรียบหรู พร้อมรูปโปรไฟล์พนักงาน ปิดเองอัตโนมัติใน 2.5 วินาที
    """
    def __init__(self, parent, name, direction_text, time_str, emp_id="", division="", photo_path=""):
        super().__init__(parent)
        self.setWindowFlags(Qt.Dialog | Qt.FramelessWindowHint)
        self.setAttribute(Qt.WA_TranslucentBackground, True)
        self.setFixedSize(430, 275)

        outer = QVBoxLayout(self)
        outer.setContentsMargins(12, 12, 12, 12)

        card = QFrame(self)
        card.setStyleSheet(
            f"""
            QFrame {{
                background-color: {CARD_BG};
                border: 1.5px solid {CARD_BORDER};
                border-radius: 20px;
            }}
            """
        )

        # เงาฟุ้งนุ่มนวล สไตล์ Minimal Elevation
        shadow = QGraphicsDropShadowEffect(card)
        shadow.setBlurRadius(24)
        shadow.setColor(QColor(35, 32, 29, 45))
        shadow.setOffset(0, 6)
        card.setGraphicsEffect(shadow)

        layout = QVBoxLayout(card)
        layout.setContentsMargins(20, 16, 20, 16)
        layout.setSpacing(4)

        # 1. หัวข้อสถานะ
        title = QLabel("บันทึกเวลาสำเร็จ")
        title.setFont(QFont(FONT_FAMILY, 13, QFont.Bold))
        title.setAlignment(Qt.AlignCenter)
        title.setStyleSheet(f"color: {ACCENT_SUCCESS}; border: none; background: transparent;")
        layout.addWidget(title)

        # 2. รูปโปรไฟล์พนักงาน (วงกลม)
        avatar_lbl = QLabel()
        avatar_lbl.setAlignment(Qt.AlignCenter)
        avatar_lbl.setStyleSheet("border: none; background: transparent;")
        pix = make_circular_pixmap(photo_path, size=58)
        if pix is None:
            pix = make_monogram_pixmap(name, size=58)
        avatar_lbl.setPixmap(pix)
        layout.addWidget(avatar_lbl)

        # 3. ชื่อ-นามสกุล พร้อมคำนำหน้า
        name_lbl = QLabel(name)
        name_lbl.setFont(QFont(FONT_FAMILY, 19, QFont.Bold))
        name_lbl.setAlignment(Qt.AlignCenter)
        name_lbl.setStyleSheet(f"color: {TEXT_PRIMARY}; border: none; background: transparent;")
        name_lbl.setWordWrap(True)
        layout.addWidget(name_lbl)

        # 4. รหัสพนักงาน และ สังกัดฝ่าย
        sub_items = []
        if emp_id:
            sub_items.append(f"รหัส: {emp_id}")
        if division:
            sub_items.append(division)
        sub_text = " • ".join(sub_items) if sub_items else ""

        if sub_text:
            sub_lbl = QLabel(sub_text)
            sub_lbl.setFont(QFont(FONT_FAMILY, 11))
            sub_lbl.setAlignment(Qt.AlignCenter)
            sub_lbl.setStyleSheet(f"color: {TEXT_MUTED}; border: none; background: transparent;")
            sub_lbl.setWordWrap(True)
            layout.addWidget(sub_lbl)

        # 5. ทิศทางและเวลา
        time_lbl = QLabel(f"บันทึก: {direction_text} • เวลา {time_str} น.")
        time_lbl.setFont(QFont(FONT_FAMILY, 11))
        time_lbl.setAlignment(Qt.AlignCenter)
        time_lbl.setStyleSheet(f"color: {TEXT_SECONDARY}; border: none; background: transparent;")
        layout.addWidget(time_lbl)

        outer.addWidget(card)

        # จัดตำแหน่งกึ่งกลางหน้าต่างหลัก
        if parent:
            geo = parent.geometry()
            self.move(
                geo.x() + (geo.width() - self.width()) // 2,
                geo.y() + (geo.height() - self.height()) // 2,
            )

        # ปิดอัตโนมัติใน 2.5 วินาที
        QTimer.singleShot(2500, self.close)

    def mousePressEvent(self, event):
        # โหมดไร้สัมผัส: เพิกเฉยต่อการแตะจอ เพื่อไม่ให้ปิดป๊อปอัปก่อนครบเวลา
        event.ignore()


class MainWindow(QMainWindow):
    """
    Kiosk Screen:
    - กล้องสดไร้ขอบ ขยายเต็มหน้าจอ (Full-Screen Camera)
    - แถบเวลาด้านบน โทนขาวครีมมินิมอล (Warm Cream Minimal Header)
    - ไม่มีการ์ดล่างเกะกะสายตา
    - แจ้งเตือนด้วย Floating Minimal Card ตรงกลางจอ
    """
    def __init__(self, config, camera, detector, database):
        super().__init__()
        self.config = config
        self.camera = camera
        self.detector = detector
        self.db = database
        self.current_direction = config.device_direction
        self._active_popup = None

        self.setWindowTitle("Face Attendance Kiosk")
        self.setStyleSheet(f"background-color: {WARM_CREAM_BG}; color: {TEXT_PRIMARY};")

        # Kiosk Lock: ไร้ขอบหน้าต่าง ล็อกให้อยู่บนสุดเสมอ ป้องกันคนปัดหรือย่อหน้าต่าง
        self.setWindowFlags(Qt.Window | Qt.FramelessWindowHint | Qt.WindowStaysOnTopHint)

        if config.fullscreen:
            self.showFullScreen()
        else:
            self.setFixedSize(config.display_width, config.display_height)

        # Auto-hide Cursor: ซ่อนเมาส์ไม่ให้เกะกะ แต่ถ้าขยับเมาส์จะปรากฏ และซ่อนตัวเมื่อนิ่งเกิน 3 วินาที
        self.setMouseTracking(True)
        self.setCursor(Qt.BlankCursor)
        self._cursor_timer = QTimer(self)
        self._cursor_timer.setSingleShot(True)
        self._cursor_timer.timeout.connect(self._hide_cursor)

        self._build_ui()
        self._start_clock()
        self._start_worker()

    def mouseMoveEvent(self, event):
        # เมื่อมีการขยับเมาส์ ให้แสดงเคอร์เซอร์ขึ้นมา และเริ่มนับเวลาถอยหลัง 3 วินาที
        self.unsetCursor()
        self._cursor_timer.start(3000)
        super().mouseMoveEvent(event)

    def _hide_cursor(self):
        self.setCursor(Qt.BlankCursor)

    def mousePressEvent(self, event):
        # เพิกเฉยต่อการแตะ/คลิกหน้าจอ ป้องกันคนแตะเล่น
        event.ignore()

    def mouseReleaseEvent(self, event):
        event.ignore()

    def mouseDoubleClickEvent(self, event):
        event.ignore()

    def keyPressEvent(self, event):
        # ปุ่มลับเฉพาะแอดมิน: Ctrl + Q สำหรับปิดโปรแกรม
        if event.modifiers() == Qt.ControlModifier and event.key() == Qt.Key_Q:
            logger.info("Admin exit shortcut (Ctrl+Q) triggered.")
            self.close()
        else:
            # เพิกเฉยต่อปุ่มอื่นๆ เช่น ESC, F11, Spacebar เพื่อไม่ให้หลุดออกจากหน้าต่างเต็มจอ
            event.ignore()

    def _build_ui(self):
        central = QWidget()
        self.setCentralWidget(central)

        layout = QVBoxLayout(central)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # ── 1. ส่วนเวลาและวันที่ลอยด้านบน (Centered Stacked Minimal Header) ───────
        time_bar = QFrame()
        time_bar.setStyleSheet(
            f"background-color: {HEADER_BG}; border-bottom: 1px solid {HEADER_BORDER};"
        )
        t_layout = QVBoxLayout(time_bar)
        t_layout.setContentsMargins(16, 12, 16, 10)
        t_layout.setSpacing(2)

        self.clock_label = QLabel("00:00:00")
        self.clock_label.setFont(QFont(FONT_FAMILY, 28, QFont.Bold))
        self.clock_label.setAlignment(Qt.AlignCenter)
        self.clock_label.setStyleSheet(f"color: {TEXT_PRIMARY}; border: none; background: transparent;")
        t_layout.addWidget(self.clock_label)

        self.date_label = QLabel("")
        self.date_label.setFont(QFont(FONT_FAMILY, 12))
        self.date_label.setAlignment(Qt.AlignCenter)
        self.date_label.setStyleSheet(f"color: {TEXT_SECONDARY}; border: none; background: transparent;")
        t_layout.addWidget(self.date_label)

        layout.addWidget(time_bar)

        # ── 2. หน้าจอกล้องสดเต็มจอ (Full-Screen Borderless Camera) ──
        self.cam_label = QLabel("กำลังเริ่มกล้อง...")
        self.cam_label.setAlignment(Qt.AlignCenter)
        self.cam_label.setStyleSheet(f"background-color: {WARM_CREAM_BG}; color: {TEXT_MUTED}; border: none;")
        self.cam_label.setSizePolicy(QSizePolicy.Ignored, QSizePolicy.Ignored)
        self.cam_label.setMinimumSize(1, 1)

        layout.addWidget(self.cam_label, stretch=1)

    def _start_clock(self):
        self._clock_timer = QTimer(self)
        self._clock_timer.timeout.connect(self._tick_clock)
        self._clock_timer.start(1000)
        self._tick_clock()

    def _tick_clock(self):
        now = datetime.now()
        self.clock_label.setText(now.strftime("%H:%M:%S"))
        thai_m = THAI_MONTHS[now.month] if 1 <= now.month <= 12 else ""
        day_name = THAI_DAYS[now.weekday()]
        self.date_label.setText(f"{day_name}ที่ {now.day} {thai_m} {now.year + 543}")

    def _start_worker(self):
        self.worker = RecognitionWorker(
            self.camera, self.detector, self.db, self.config, self.current_direction
        )
        self.worker.frame_ready.connect(self._update_frame)
        self.worker.face_recognized.connect(self._on_recognized)
        self.worker.unknown_detected.connect(self._on_unknown)
        self.worker.spoof_detected.connect(self._on_spoof_detected)
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

        # Center-Crop: ตัดส่วนเกินซ้าย-ขวาออก เพื่อให้ได้อัตราส่วนเต็มจอโดยหน้าไม่ยืด
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

    @pyqtSlot(str, float, str, str, str, str)
    def _on_recognized(self, name, conf, direction, emp_id, division, photo_path):
        """เมื่อสแกนเจอพนักงาน: แสดง Floating Glass Card ตรงกลางจอ พร้อมรูปโปรไฟล์"""
        dir_text = "เข้างาน" if direction == "IN" else "ออกงาน"
        now_time = datetime.now().strftime("%H:%M:%S")

        # ปิดป๊อปอัปเดิมถ้ายังค้างอยู่
        if self._active_popup:
            try:
                self._active_popup.close()
            except Exception:
                pass

        # แสดง Floating Glass Card ตรงกลางจอ
        self._active_popup = AttendanceSuccessPopup(
            self, name, dir_text, now_time, emp_id=emp_id, division=division, photo_path=photo_path
        )
        self._active_popup.show()

    @pyqtSlot()
    def _on_unknown(self):
        """เมื่อตรวจจับใบหน้าแล้วไม่พบในระบบ: ไม่แสดงข้อความกวนสายตา"""
        pass

    @pyqtSlot()
    def _on_spoof_detected(self):
        """เมื่อตรวจพบภาพถ่าย/จอมือถือ: ป้องกันเงียบๆ ไม่แสดงป๊อปอัปแจ้งเตือนกวนสายตา"""
        pass

    def closeEvent(self, event):
        if hasattr(self, 'worker') and self.worker:
            self.worker.stop()
        event.accept()


def create_app(config, camera, detector, database):
    app = QApplication(sys.argv)
    init_fonts()
    app.setApplicationName("FaceAttendance")
    app.setFont(QFont(FONT_FAMILY, 11))
    window = MainWindow(config, camera, detector, database)
    return app, window
