"""
Face matcher — cosine similarity search against enrolled embeddings.
"""
import numpy as np
from typing import Optional


def cosine_similarity(a: np.ndarray, b: np.ndarray) -> float:
    """Compute cosine similarity between two L2-normalized vectors."""
    return float(np.dot(a, b))


def find_best_match(
    query_embedding: np.ndarray,
    gallery: list[tuple[str, np.ndarray]],   # [(user_id, embedding), ...]
    threshold: float = 0.55,
) -> Optional[tuple[str, float]]:
    """
    Find the best matching user from the gallery.

    Args:
        query_embedding: 512-d L2-normalized embedding
        gallery: List of (user_id, embedding) tuples
        threshold: Minimum cosine similarity to accept

    Returns:
        (user_id, confidence) if match found above threshold, else None
    """
    if not gallery:
        return None

    best_score = -1.0
    best_user_id = None

    for user_id, emb in gallery:
        score = cosine_similarity(query_embedding, emb)
        if score > best_score:
            best_score = score
            best_user_id = user_id

    if best_score >= threshold:
        return (best_user_id, best_score)
    return None
