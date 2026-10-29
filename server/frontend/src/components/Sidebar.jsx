import React from 'react';
import { NavLink, useNavigate } from 'react-router-dom';

export default function Sidebar() {
  const navigate = useNavigate();

  let currentUser = { username: 'admin', role: 'admin' };
  try {
    const stored = localStorage.getItem('user');
    if (stored) currentUser = JSON.parse(stored);
  } catch (e) {
    // fallback
  }

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2>📋 Face Attendance</h2>
      </div>

      <ul className="sidebar-nav">
        <li>
          <NavLink to="/devices">🖥️ อุปกรณ์ (Devices)</NavLink>
        </li>
        <li>
          <NavLink to="/users">👤 ข้อมูลพนักงาน (Users)</NavLink>
        </li>
        <li>
          <NavLink to="/enrollment">⚡ คิวลงทะเบียนใบหน้า (Enrollment)</NavLink>
        </li>
        <li>
          <NavLink to="/attendance">📅 ประวัติลงเวลา (Attendance)</NavLink>
        </li>
        <li>
          <NavLink to="/audit">📝 บันทึกประวัติ (Audit Log)</NavLink>
        </li>
      </ul>

      <div className="sidebar-footer">
        {/* กล่องแสดงสิทธิ์ผู้ใช้งานปัจจุบัน */}
        <div
          style={{
            padding: '0.65rem 0.75rem',
            backgroundColor: 'rgba(255, 255, 255, 0.08)',
            borderRadius: '6px',
            marginBottom: '0.75rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.65rem',
          }}
        >
          <div
            style={{
              width: '32px',
              height: '32px',
              borderRadius: '50%',
              background: currentUser.role === 'admin' ? '#0984e3' : '#636e72',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: '#fff',
              fontSize: '1rem',
              fontWeight: 'bold',
              flexShrink: 0,
            }}
          >
            {currentUser.username ? currentUser.username.charAt(0).toUpperCase() : 'A'}
          </div>
          <div style={{ overflow: 'hidden', flex: 1 }}>
            <div
              style={{
                fontSize: '0.88rem',
                fontWeight: 600,
                color: '#fff',
                whiteSpace: 'nowrap',
                textOverflow: 'ellipsis',
                overflow: 'hidden',
              }}
              title={currentUser.username}
            >
              {currentUser.username || 'admin'}
            </div>
            <div
              style={{
                fontSize: '0.74rem',
                color: currentUser.role === 'admin' ? '#2ed573' : '#70a1ff',
                fontWeight: 500,
                marginTop: '1px',
                display: 'flex',
                alignItems: 'center',
                gap: '0.25rem',
              }}
            >
              <span>{currentUser.role === 'admin' ? '🛡️ ผู้ดูแลระบบ (Admin)' : '👁️ ผู้ใช้งานทั่วไป (Viewer)'}</span>
            </div>
          </div>
        </div>

        <button onClick={handleLogout} className="btn-logout">
          🚪 ออกจากระบบ
        </button>
      </div>
    </aside>
  );
}
