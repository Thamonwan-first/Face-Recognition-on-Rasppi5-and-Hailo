"""
SQLAlchemy Database setup supporting SQLite (dev) and PostgreSQL (prod).
"""
import os
from pathlib import Path
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, declarative_base

DB_DIALECT = os.getenv("DB_DIALECT", "sqlite")
SQLITE_PATH = os.getenv("SQLITE_PATH", "./data/server.db")

if DB_DIALECT == "postgres":
    DB_USER = os.getenv("DB_USER", "attendance_app")
    DB_PASSWORD = os.getenv("DB_PASSWORD", "changeme")
    DB_HOST = os.getenv("DB_HOST", "localhost")
    DB_PORT = os.getenv("DB_PORT", "5432")
    DB_NAME = os.getenv("DB_NAME", "attendance")
    DATABASE_URL = f"postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"
    engine = create_engine(DATABASE_URL, pool_size=10, max_overflow=20)
else:
    db_file = Path(SQLITE_PATH)
    db_file.parent.mkdir(parents=True, exist_ok=True)
    DATABASE_URL = f"sqlite:///{db_file.resolve()}"
    engine = create_engine(DATABASE_URL, connect_args={"check_same_thread": False})

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()


def get_db():
    """FastAPI dependency for database session."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
