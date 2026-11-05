import React from 'react';
import { NavLink, useNavigate } from 'react-router-dom';

export default function Sidebar() {
  const navigate = useNavigate();

  let currentUser = { username: 'admin', role: 'admin', permissions: ['*'] };
  try {
    const stored = localStorage.getItem('user');
    if (stored) currentUser = JSON.parse(stored);
  } catch (e) {
    // fallback
  }

  const hasPerm = (key) => {
    if (currentUser.role === 'admin') return true;
    if (!currentUser.permissions || !Array.isArray(currentUser.permissions)) return false;
    return currentUser.permissions.includes(key) || currentUser.permissions.includes('*');
  };

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2>ระบบบันทึกเวลาทำงาน</h2>
      </div>

      <ul className="sidebar-nav">
        {hasPerm('devices') && (
          <li>
            <NavLink to="/devices">รายการอุปกรณ์</NavLink>
          </li>
        )}
        {hasPerm('users') && (
          <li>
            <NavLink to="/users">ข้อมูลพนักงาน</NavLink>
          </li>
        )}
        {hasPerm('attendance') && (
          <li>
            <NavLink to="/attendance">ประวัติการลงเวลา</NavLink>
          </li>
        )}
        {hasPerm('audit') && (
          <li>
            <NavLink to="/audit">บันทึกประวัติการใช้งาน</NavLink>
          </li>
        )}
        {hasPerm('admins') && (
          <li>
            <NavLink to="/admins">จัดการผู้ดูแลระบบ</NavLink>
          </li>
        )}
      </ul>

      <div className="sidebar-footer">
        {/* กล่องแสดงสิทธิ์ผู้ใช้งานปัจจุบัน พร้อมไอคอนฟันเฟืองตรงกลางกรอบด้านขวา */}
        <div
          style={{
            padding: '0.65rem 0.75rem',
            backgroundColor: 'rgba(255, 255, 255, 0.08)',
            borderRadius: '6px',
            marginBottom: '0.75rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '0.65rem',
          }}
        >
          {/* ส่วนซ้าย: รูปโปรไฟล์ + ชื่อ + บทบาท */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem', overflow: 'hidden', flex: 1 }}>
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
            <div style={{ overflow: 'hidden' }}>
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
                  marginTop: '2px',
                }}
              >
                {currentUser.role === 'admin' ? 'ผู้ดูแลระบบสูงสุด' : 'ผู้ใช้งานแดชบอร์ด'}
              </div>
            </div>
          </div>

          {/* ส่วนขวา: ไอคอนฟันเฟืองลอยตัวตรงกลางความสูงของกรอบ โดยไม่มีกรอบสี่เหลี่ยม */}
          <button
            onClick={() => navigate('/settings')}
            title="ตั้งค่า (เปลี่ยนรหัสผ่าน / ตั้งค่าระบบ)"
            style={{
              background: 'none',
              border: 'none',
              padding: '4px',
              margin: '0',
              color: '#94a3b8',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              transition: 'color 0.2s, transform 0.2s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = '#ffffff';
              e.currentTarget.style.transform = 'rotate(45deg)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = '#94a3b8';
              e.currentTarget.style.transform = 'none';
            }}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <circle cx="12" cy="12" r="3"></circle>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
            </svg>
          </button>
        </div>

        <button onClick={handleLogout} className="btn-logout">
          ออกจากระบบ
        </button>
      </div>
    </aside>
  );
}
