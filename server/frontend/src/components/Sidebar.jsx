import React, { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { api } from '../api';

export default function Sidebar() {
  const navigate = useNavigate();
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [passwordForm, setPasswordForm] = useState({ old_password: '', new_password: '', confirm_password: '' });
  const [passwordError, setPasswordError] = useState('');
  const [passwordLoading, setPasswordLoading] = useState(false);

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

  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    setPasswordError('');

    if (passwordForm.new_password.length < 6) {
      setPasswordError('รหัสผ่านใหม่ต้องมีความยาวอย่างน้อย 6 ตัวอักษร');
      return;
    }
    if (passwordForm.new_password !== passwordForm.confirm_password) {
      setPasswordError('รหัสผ่านใหม่กับยืนยันรหัสผ่านไม่ตรงกัน');
      return;
    }

    setPasswordLoading(true);
    try {
      await api.post('/api/auth/change-password', {
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password,
      });
      alert('เปลี่ยนรหัสผ่านสำเร็จเรียบร้อยแล้ว');
      setShowPasswordModal(false);
      setPasswordForm({ old_password: '', new_password: '', confirm_password: '' });
    } catch (err) {
      setPasswordError(err.message || 'เปลี่ยนรหัสผ่านไม่สำเร็จ');
    } finally {
      setPasswordLoading(false);
    }
  };

  return (
    <>
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
          {hasPerm('enrollment') && (
            <li>
              <NavLink to="/enrollment">คิวลงทะเบียนใบหน้า</NavLink>
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
                <span>{currentUser.role === 'admin' ? 'ผู้ดูแลระบบสูงสุด' : 'ผู้ใช้งานแดชบอร์ด'}</span>
              </div>
            </div>
          </div>

          <button
            onClick={() => {
              setPasswordError('');
              setPasswordForm({ old_password: '', new_password: '', confirm_password: '' });
              setShowPasswordModal(true);
            }}
            className="btn"
            style={{
              width: '100%',
              marginBottom: '0.5rem',
              background: 'rgba(255, 255, 255, 0.12)',
              color: '#fff',
              fontSize: '0.8rem',
              padding: '0.45rem',
            }}
          >
            เปลี่ยนรหัสผ่าน
          </button>

          <button onClick={handleLogout} className="btn-logout">
            ออกจากระบบ
          </button>
        </div>
      </aside>

      {/* Modal เปลี่ยนรหัสผ่านของตนเอง */}
      {showPasswordModal && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 200,
          }}
        >
          <div className="login-card" style={{ maxWidth: '420px', width: '90%' }}>
            <h3 style={{ marginBottom: '1rem' }}>เปลี่ยนรหัสผ่านของตนเอง</h3>

            {passwordError && (
              <div
                style={{
                  padding: '0.6rem 0.8rem',
                  backgroundColor: '#fde8e8',
                  color: 'var(--danger)',
                  borderRadius: '6px',
                  marginBottom: '1rem',
                  fontSize: '0.85rem',
                }}
              >
                {passwordError}
              </div>
            )}

            <form onSubmit={handlePasswordSubmit}>
              <div className="form-group">
                <label>รหัสผ่านปัจจุบัน *</label>
                <input
                  type="password"
                  className="input-field"
                  value={passwordForm.old_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, old_password: e.target.value })}
                  required
                />
              </div>

              <div className="form-group">
                <label>รหัสผ่านใหม่ *</label>
                <input
                  type="password"
                  className="input-field"
                  placeholder="ความยาวอย่างน้อย 6 ตัวอักษร"
                  value={passwordForm.new_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, new_password: e.target.value })}
                  required
                />
              </div>

              <div className="form-group">
                <label>ยืนยันรหัสผ่านใหม่ *</label>
                <input
                  type="password"
                  className="input-field"
                  placeholder="กรอกรหัสผ่านใหม่อีกครั้ง"
                  value={passwordForm.confirm_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, confirm_password: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.25rem' }}>
                <button
                  type="button"
                  onClick={() => setShowPasswordModal(false)}
                  className="btn"
                  style={{ background: '#eee', color: '#333' }}
                >
                  ยกเลิก
                </button>
                <button type="submit" className="btn btn-primary" disabled={passwordLoading}>
                  {passwordLoading ? 'กำลังบันทึก...' : 'บันทึกรหัสผ่านใหม่'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
