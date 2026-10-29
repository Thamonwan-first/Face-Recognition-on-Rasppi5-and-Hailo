"""
Shared utility functions for the backend API.
"""
from datetime import timezone


def format_utc_iso(dt):
    """Format a datetime as ISO string, ensuring UTC timezone info."""
    if not dt:
        return None
    aware = dt if dt.tzinfo else dt.replace(tzinfo=timezone.utc)
    return aware.isoformat()
