import React from 'react';
import { NavLink, useNavigate } from 'react-router-dom';

export default function Sidebar() {
  const navigate = useNavigate();

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
          <NavLink to="/users">👤 ผู้ใช้ (Users)</NavLink>
        </li>
        <li>
          <NavLink to="/attendance">🕐 บันทึกเวลา (Attendance)</NavLink>
        </li>
        <li>
          <NavLink to="/enrollment">📸 ลงทะเบียนใบหน้า (Enrollment)</NavLink>
        </li>
        <li>
          <NavLink to="/audit">📝 บันทึกประวัติ (Audit Log)</NavLink>
        </li>
      </ul>

      <div className="sidebar-footer">
        <button onClick={handleLogout} className="btn-logout">
          🚪 ออกจากระบบ
        </button>
      </div>
    </aside>
  );
}
