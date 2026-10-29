import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDateTime } from '../utils/date';

export default function AuditPage() {
  const { data: logs, isLoading, error } = useQuery({
    queryKey: ['audit_logs'],
    queryFn: () => api.get('/api/audit'),
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">📝 บันทึกประวัติการกระทำ (Audit Log)</h1>
      </div>

      {isLoading && <p>กำลังโหลดบันทึกการกระทำ...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>เวลาที่บันทึก</th>
              <th>ผู้ดำเนินการ (Admin)</th>
              <th>คำสั่ง / กิจกรรม (Action)</th>
              <th>ประเภทเป้าหมาย</th>
              <th>รหัสเป้าหมาย</th>
              <th>รายละเอียด (Snapshot)</th>
            </tr>
          </thead>
          <tbody>
            {logs?.map((l) => (
              <tr key={l.id}>
                <td>{formatThaiDateTime(l.created_at)}</td>
                <td>{l.admin_username || l.admin_id || 'System'}</td>
                <td>
                  <strong>{l.action}</strong>
                </td>
                <td>{l.target_type || '-'}</td>
                <td>{l.target_id || '-'}</td>
                <td>
                  <code style={{ fontSize: '0.8rem', background: '#f1f2f6', padding: '2px 4px', borderRadius: '4px' }}>
                    {typeof l.detail === 'object' ? JSON.stringify(l.detail) : l.detail || '-'}
                  </code>
                </td>
              </tr>
            ))}
            {logs?.length === 0 && (
              <tr>
                <td colSpan="6" style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
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
