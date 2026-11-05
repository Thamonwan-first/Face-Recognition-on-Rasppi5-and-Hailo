import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDateTime } from '../utils/date';

// ฟังก์ชันแปลงชื่อ Action เป็นภาษาไทย
function formatAction(action) {
  switch (action) {
    case 'admin.login':
      return 'เข้าสู่ระบบ';
    case 'user.create':
      return 'เพิ่มพนักงานใหม่';
    case 'user.update':
      return 'แก้ไขข้อมูลพนักงาน';
    case 'user.delete':
      return 'ลบพนักงาน';
    case 'device.register':
      return 'ลงทะเบียนอุปกรณ์';
    case 'device.delete':
      return 'ลบอุปกรณ์';
    case 'attendance.manual_add':
      return 'เพิ่มเวลาทำงานย้อนหลัง';
    case 'attendance.manual_edit':
      return 'แก้ไขเวลาทำงาน';
    case 'enroll_face':
    case 'ลงทะเบียนใบหน้า':
      return 'ลงทะเบียนใบหน้า';
    default:
      return action;
  }
}

// ฟังก์ชันจัดรูปแบบรายละเอียดให้อ่านง่าย
function formatDetail(detail) {
  if (!detail) return '-';

  // หากเป็น JSON string ให้แกะข้อมูลออกมาแสดงเป็นภาษาพูด
  if (typeof detail === 'string' && detail.trim().startsWith('{') && detail.trim().endsWith('}')) {
    try {
      const obj = JSON.parse(detail);
      const parts = [];
      if (obj.name) {
        const emp = obj.employee_id ? `[${obj.employee_id}] ` : '';
        parts.push(`พนักงาน: ${emp}${obj.name}`);
      }
      if (obj.username) {
        parts.push(`ผู้ดูแลระบบ: ${obj.username}`);
      }
      if (obj.device_name) {
        parts.push(`อุปกรณ์: ${obj.device_name}`);
      }
      if (obj.version && parts.length === 0) {
        parts.push(`เวอร์ชัน ${obj.version}`);
      }
      if (parts.length > 0) {
        return parts.join(' | ');
      }
    } catch {
      // fallback
    }
  }

  return typeof detail === 'object' ? JSON.stringify(detail) : detail;
}

export default function AuditPage() {
  const { data: logs, isLoading, error } = useQuery({
    queryKey: ['audit_logs'],
    queryFn: () => api.get('/api/audit'),
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">บันทึกประวัติการใช้งาน (Audit Log)</h1>
      </div>

      {isLoading && <p>กำลังโหลดบันทึกการกระทำ...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>วัน-เวลา</th>
              <th>ผู้ดำเนินการ</th>
              <th>เหตุการณ์ (Action)</th>
              <th>รายละเอียด (Details)</th>
              <th>สถานะ</th>
            </tr>
          </thead>
          <tbody>
            {logs?.map((l) => {
              const operatorName =
                l.admin_username && l.admin_username !== 'admin-default-uuid'
                  ? l.admin_username
                  : l.admin_id === 'admin-default-uuid'
                  ? 'admin'
                  : l.admin_id || 'System';

              return (
                <tr key={l.id}>
                  <td>{formatThaiDateTime(l.created_at)}</td>
                  <td>
                    <span
                      style={{
                        padding: '0.2rem 0.55rem',
                        borderRadius: '9999px',
                        background: '#eff6ff',
                        color: '#1d4ed8',
                        fontWeight: 600,
                        fontSize: '0.8rem',
                      }}
                    >
                      {operatorName}
                    </span>
                  </td>
                  <td>
                    <strong>{formatAction(l.action)}</strong>
                  </td>
                  <td style={{ color: '#334155', fontSize: '0.9rem' }}>
                    {formatDetail(l.detail)}
                  </td>
                  <td>
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '0.35rem',
                        padding: '0.2rem 0.6rem',
                        borderRadius: '9999px',
                        fontSize: '0.8rem',
                        fontWeight: 600,
                        background: '#dcfce7',
                        color: '#15803d',
                        border: '1px solid #bbf7d0',
                      }}
                    >
                      <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e' }}></span>
                      สำเร็จ
                    </span>
                  </td>
                </tr>
              );
            })}
            {logs?.length === 0 && (
              <tr>
                <td colSpan="5" style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '1.5rem' }}>
                  ยังไม่มีประวัติการบันทึกกิจกรรม
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
