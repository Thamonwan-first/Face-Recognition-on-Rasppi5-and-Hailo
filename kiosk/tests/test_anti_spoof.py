"""
Unit tests for MiniFASNet Passive Anti-Spoofing Detector.
"""
import unittest
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

import numpy as np
from anti_spoof import AntiSpoofDetector


class TestAntiSpoof(unittest.TestCase):
    def test_anti_spoof_initialization(self):
        detector = AntiSpoofDetector()
        self.assertTrue(detector.enabled)
        self.assertIsNotNone(detector.net)

    def test_anti_spoof_fake_crop(self):
        detector = AntiSpoofDetector(threshold=0.85)
        dummy_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        bbox = [100, 100, 250, 250]
        is_real, score = detector.check_liveness(dummy_frame, bbox)
        self.assertFalse(is_real)
        self.assertTrue(0.0 <= score <= 1.0)

    def test_anti_spoof_disabled_fallback(self):
        detector = AntiSpoofDetector(model_path="non_existent.onnx")
        self.assertFalse(detector.enabled)
        dummy_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        is_real, score = detector.check_liveness(dummy_frame, [50, 50, 100, 100])
        self.assertTrue(is_real)
        self.assertEqual(score, 1.0)


if __name__ == "__main__":
    unittest.main()
