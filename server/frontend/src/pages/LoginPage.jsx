import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const data = await api.post('/auth/login', { username, password });
      const token = data.token || data.access_token;
      if (token) {
        localStorage.setItem('token', token);
      }
      const admin = data.admin || data.user;
      if (admin) {
        localStorage.setItem('user', JSON.stringify(admin));
      }
      navigate('/devices');
    } catch (err) {
      setError(err.message || 'เข้าสู่ระบบล้มเหลว กรุณาตรวจสอบชื่อผู้ใช้และรหัสผ่าน');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-card">
        <div className="login-header">
          <h2>เข้าสู่ระบบ</h2>
          <p style={{ color: 'var(--text-muted)', marginTop: '0.25rem' }}>
            แดชบอร์ดจัดการระบบบันทึกเวลา
          </p>
        </div>

        {error && (
          <div
            style={{
              padding: '0.75rem',
              backgroundColor: '#fde8e8',
              color: 'var(--danger)',
              borderRadius: '6px',
              marginBottom: '1rem',
              fontSize: '0.9rem',
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>ชื่อผู้ใช้งาน</label>
            <input
              type="text"
              className="input-field"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
            />
          </div>

          <div className="form-group">
            <label>รหัสผ่าน</label>
            <input
              type="password"
              className="input-field"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%', justifyContent: 'center', marginTop: '0.5rem' }}
            disabled={loading}
          >
            {loading ? 'กำลังเข้าสู่ระบบ...' : 'เข้าสู่ระบบ'}
          </button>
        </form>
      </div>
    </div>
  );
}
