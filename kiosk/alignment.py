"""
Face alignment — 5-point landmark affine transform to 112x112 for ArcFace.
"""
import cv2
import numpy as np

# Standard ArcFace alignment reference points for 112x112 output
ARCFACE_REF_LANDMARKS = np.array([
    [38.2946, 51.6963],   # left eye
    [73.5318, 51.5014],   # right eye
    [56.0252, 71.7366],   # nose tip
    [41.5493, 92.3655],   # left mouth corner
    [70.7299, 92.2041],   # right mouth corner
], dtype=np.float32)


def align_face(image: np.ndarray, landmarks: np.ndarray, output_size: int = 112) -> np.ndarray:
    """
    Align a face crop using 5 landmark points and affine transform.

    Args:
        image: Full frame (H, W, 3) uint8
        landmarks: 5x2 array of facial landmark coordinates [[x,y], ...]
        output_size: Output image size (default 112 for ArcFace)

    Returns:
        Aligned face image (output_size, output_size, 3) uint8
    """
    src = landmarks.astype(np.float32)
    dst = ARCFACE_REF_LANDMARKS.copy()

    # Compute similarity transform (rotation + scale + translation)
    tform = cv2.estimateAffinePartial2D(src, dst)[0]

    aligned = cv2.warpAffine(
        image, tform, (output_size, output_size),
        borderValue=0,
    )
    return aligned
