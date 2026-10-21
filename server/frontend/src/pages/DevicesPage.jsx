import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';

export default function DevicesPage() {
  const { data: devices, isLoading, error, refetch } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices/status'),
    refetchInterval: 30000, // Refresh every 30s
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">🖥️ รายการอุปกรณ์ (Devices)</h1>
        <button onClick={() => refetch()} className="btn btn-primary">
          🔄 รีเฟรชข้อมูล
        </button>
      </div>

      {isLoading && <p>กำลังโหลดข้อมูลอุปกรณ์...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="cards-grid">
        {devices?.map((device) => (
          <div key={device.id} className={`device-card ${device.status}`}>
            <div className="card-header">
              <div>
                <h3 className="card-title">{device.name || device.id}</h3>
                <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  ID: {device.id}
                </span>
              </div>
              <StatusBadge status={device.status} />
            </div>

            <ul className="card-details">
              <li>📍 สถานที่: {device.location || '-'}</li>
              <li>↔️ ทิศทาง: {device.direction || 'AUTO'}</li>
              <li>🌡️ อุณหภูมิ CPU: {device.cpu_temp ? `${device.cpu_temp}°C` : '-'}</li>
              <li>🎯 FPS กล้อง: {device.fps ? `${device.fps}` : '-'}</li>
              <li>📤 รายการรอส่ง (Outbox): {device.outbox_pending ?? 0}</li>
              <li>🔄 เวอร์ชันข้อมูลที่ซิงก์: v{device.applied_version ?? 0}</li>
              <li>
                ⏰ ออนไลน์ล่าสุด:{' '}
                {device.last_seen ? new Date(device.last_seen).toLocaleString('th-TH') : 'ไม่เคย'}
              </li>
            </ul>
          </div>
        ))}

        {devices?.length === 0 && (
          <p style={{ color: 'var(--text-muted)' }}>ยังไม่มีอุปกรณ์ลงทะเบียนในระบบ</p>
        )}
      </div>
    </div>
  );
}
