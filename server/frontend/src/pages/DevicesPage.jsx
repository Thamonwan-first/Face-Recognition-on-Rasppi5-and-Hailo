import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';

function formatThaiDateTime(dateStr) {
  if (!dateStr) return 'ไม่เคย';
  const utcStr = dateStr.endsWith('Z') || dateStr.includes('+') ? dateStr : `${dateStr}Z`;
  return new Date(utcStr).toLocaleString('th-TH');
}

export default function DevicesPage() {
  const { data: devices, isLoading, error } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices/status'),
    refetchInterval: 2000, // Real-time poll every 2s
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">🖥️ รายการอุปกรณ์ (Devices)</h1>
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
              <li>🌡️ อุณหภูมิ CPU: {device.cpu_temp != null ? `${device.cpu_temp}°C` : '-'}</li>
              <li>🎯 FPS กล้อง: {device.fps != null ? `${device.fps}` : '-'}</li>
              <li>
                ⏰ ออนไลน์ล่าสุด:{' '}
                {formatThaiDateTime(device.last_seen)}
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
