import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';
import { formatThaiDateTime } from '../utils/date';
import { useRealtime } from '../hooks/useRealtime';

const INITIAL_DEVICE_FORM = {
  id: '',
  name: '',
  location: '',
  direction: 'AUTO',
  ip_address: '',
};

export default function DevicesPage() {
  const queryClient = useQueryClient();
  useRealtime();

  const [showAddModal, setShowAddModal] = useState(false);
  const [formData, setFormData] = useState(INITIAL_DEVICE_FORM);
  const [newKeyData, setNewKeyData] = useState(null);
  const [copied, setCopied] = useState(false);

  // Fetch devices status list with auto-refresh every 3 seconds
  const { data: devices, isLoading, error } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices/status'),
    refetchInterval: 3000,
  });

  // Mutation: Register new device
  const registerMutation = useMutation({
    mutationFn: (newDevice) => api.post('/api/devices', newDevice),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['devices'] });
      setShowAddModal(false);
      setFormData(INITIAL_DEVICE_FORM);
      setNewKeyData({
        deviceId: res.device.id,
        apiKey: res.api_key,
        ipAddress: res.device.ip_address || '',
      });
      setCopied(false);
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการลงทะเบียนอุปกรณ์: ' + (err.message || err));
    },
  });

  // Mutation: Delete device
  const deleteMutation = useMutation({
    mutationFn: (id) => api.del(`/api/devices/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] });
      alert('ลบอุปกรณ์ออกจากระบบเรียบร้อยแล้ว');
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการลบอุปกรณ์: ' + (err.message || err));
    },
  });

  const handleRegisterSubmit = (e) => {
    e.preventDefault();
    if (!formData.id.trim()) {
      alert('กรุณากรอกรหัสอุปกรณ์ (ID)');
      return;
    }
    registerMutation.mutate(formData);
  };

  const handleDeleteDevice = (device) => {
    if (window.confirm(`ยืนยันการลบอุปกรณ์ "${device.name || device.id}" (ID: ${device.id}) หรือไม่?`)) {
      deleteMutation.mutate(device.id);
    }
  };

  const copyApiKey = () => {
    if (newKeyData?.apiKey) {
      navigator.clipboard.writeText(newKeyData.apiKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
    }
  };

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '1rem' }}>
        <div>
          <h1 className="page-title">รายการอุปกรณ์</h1>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.9rem', margin: '0.25rem 0 0' }}>
            ติดตามสถานะและที่อยู่ IP ของเครื่องสแกนใบหน้า (Kiosk) ทุกจุดในเครือข่าย
          </p>
        </div>
        <button
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
            backgroundColor: '#2563eb',
            color: '#ffffff',
            border: 'none',
            outline: 'none',
            borderRadius: '8px',
            padding: '0.6rem 1.25rem',
            fontSize: '0.9rem',
            fontWeight: 500,
            cursor: 'pointer',
            boxShadow: '0 2px 4px rgba(37, 99, 235, 0.2)',
            transition: 'background-color 0.2s',
          }}
          onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = '#1d4ed8')}
          onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = '#2563eb')}
          onClick={() => setShowAddModal(true)}
        >
          <span>+</span>
          <span>ลงทะเบียนอุปกรณ์ใหม่</span>
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

            <ul className="card-details" style={{ margin: '1rem 0' }}>
              <li>
                <strong>ที่อยู่ IP:</strong>{' '}
                {device.ip_address ? (
                  <code
                    style={{
                      background: '#eff6ff',
                      color: '#1d4ed8',
                      padding: '2px 8px',
                      borderRadius: '4px',
                      fontSize: '0.9rem',
                      fontWeight: 600,
                      border: '1px solid #bfdbfe',
                    }}
                  >
                    {device.ip_address}
                  </code>
                ) : (
                  <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                    รอการเชื่อมต่อ
                  </span>
                )}
              </li>
              <li>
                <strong>อุณหภูมิ CPU:</strong>{' '}
                {device.cpu_temp != null ? `${device.cpu_temp.toFixed(1)}°C` : '-'}
              </li>
              <li>
                <strong>อัตราเฟรมเรตกล้อง:</strong>{' '}
                {device.fps != null ? `${device.fps.toFixed(1)} FPS` : '-'}
              </li>
              <li>
                <strong>คิวรอส่ง (Outbox):</strong> {device.outbox_pending || 0} รายการ
              </li>
              <li>
                <strong>ออนไลน์ล่าสุด:</strong> {formatThaiDateTime(device.last_seen, 'ไม่เคย')}
              </li>
            </ul>

            <div style={{ borderTop: '1px solid var(--border-color, #e5e7eb)', paddingTop: '0.75rem', display: 'flex', justifyContent: 'flex-end' }}>
              <button
                className="btn-danger"
                style={{ fontSize: '0.8rem', padding: '0.35rem 0.75rem' }}
                onClick={() => handleDeleteDevice(device)}
              >
                ลบอุปกรณ์
              </button>
            </div>
          </div>
        ))}

        {devices?.length === 0 && (
          <p style={{ color: 'var(--text-muted)', gridColumn: '1 / -1', padding: '2rem 0', textAlign: 'center' }}>
            ยังไม่มีอุปกรณ์ลงทะเบียนในระบบ คลิกปุ่ม <strong>"+ ลงทะเบียนอุปกรณ์ใหม่"</strong> ด้านบนเพื่อเพิ่มเครื่องสแกน
          </p>
        )}
      </div>

      {/* Modal: ลงทะเบียนอุปกรณ์ใหม่ */}
      {showAddModal && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 100,
          }}
        >
          <div className="login-card" style={{ maxWidth: '520px', width: '92%', maxHeight: '90vh', overflowY: 'auto' }}>
            <h3 style={{ marginBottom: '1rem' }}>ลงทะเบียนอุปกรณ์ Kiosk ใหม่</h3>

            <form onSubmit={handleRegisterSubmit}>
              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', marginBottom: '0.35rem', fontWeight: 500 }}>
                  รหัสอุปกรณ์ (Device ID) *
                </label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น kiosk-01 หรือ gate-front-01"
                  value={formData.id}
                  onChange={(e) => setFormData({ ...formData, id: e.target.value })}
                  required
                />
                <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  ต้องตรงกับค่า DEVICE_ID ในไฟล์ kiosk/.env
                </span>
              </div>

              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', marginBottom: '0.35rem', fontWeight: 500 }}>
                  ชื่อเรียกอุปกรณ์
                </label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น จุดสแกน ประตูทางเข้าหลัก"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', marginBottom: '0.35rem', fontWeight: 500 }}>
                  ที่อยู่ IP ของอุปกรณ์ (ระบุล่วงหน้า หรือเว้นว่างได้)
                </label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น 172.22.1.33 หรือ 192.168.1.105"
                  value={formData.ip_address}
                  onChange={(e) => setFormData({ ...formData, ip_address: e.target.value })}
                />
                <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                  หากเว้นว่าง ระบบจะตรวจจับและบันทึก IP ให้อัตโนมัติเมื่อเครื่อง Kiosk ออนไลน์
                </span>
              </div>

              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', marginBottom: '0.35rem', fontWeight: 500 }}>
                  ตำแหน่งติดตั้ง
                </label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น อาคาร A ชั้น 1 ประตูทางเข้า"
                  value={formData.location}
                  onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ marginBottom: '1.5rem' }}>
                <label style={{ display: 'block', marginBottom: '0.35rem', fontWeight: 500 }}>
                  ทิศทางการลงเวลา
                </label>
                <select
                  className="input-field"
                  value={formData.direction}
                  onChange={(e) => setFormData({ ...formData, direction: e.target.value })}
                >
                  <option value="AUTO">อัตโนมัติ (AUTO - รองรับทั้งเข้าและออกงาน)</option>
                  <option value="IN">บันทึกเข้างานเท่านั้น (IN)</option>
                  <option value="OUT">บันทึกออกงานเท่านั้น (OUT)</option>
                </select>
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={() => setShowAddModal(false)}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn-primary"
                  disabled={registerMutation.isPending}
                >
                  {registerMutation.isPending ? 'กำลังบันทึก...' : 'ลงทะเบียน'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: แสดง API Key ที่สร้างเสร็จใหม่ */}
      {newKeyData && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 110,
          }}
        >
          <div className="login-card" style={{ maxWidth: '580px', width: '92%' }}>
            <h3 style={{ color: 'var(--primary, #2563eb)', marginBottom: '0.5rem' }}>
              ลงทะเบียนอุปกรณ์สำเร็จ!
            </h3>
            <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
              ระบบได้สร้างรหัส <strong>API Key</strong> สำหรับเครื่อง Kiosk แล้ว กรุณาคัดลอกไปใส่ในไฟล์ <code>kiosk/.env</code> (รหัสนี้จะแสดงเพียงครั้งเดียวเท่านั้น)
            </p>

            <div
              style={{
                background: '#f8fafc',
                border: '1px solid #cbd5e1',
                borderRadius: '6px',
                padding: '0.75rem 1rem',
                marginBottom: '1rem',
              }}
            >
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
                API KEY:
              </div>
              <div
                style={{
                  fontFamily: 'monospace',
                  fontSize: '0.95rem',
                  fontWeight: 600,
                  wordBreak: 'break-all',
                  color: '#0f172a',
                  background: '#e2e8f0',
                  padding: '6px 10px',
                  borderRadius: '4px',
                }}
              >
                {newKeyData.apiKey}
              </div>
            </div>

            <div
              style={{
                background: '#eff6ff',
                border: '1px solid #bfdbfe',
                borderRadius: '6px',
                padding: '0.75rem 1rem',
                marginBottom: '1.25rem',
                fontSize: '0.85rem',
              }}
            >
              <strong>คำแนะนำการตั้งค่าใน <code>kiosk/.env</code>:</strong>
              <pre
                style={{
                  margin: '0.5rem 0 0',
                  fontFamily: 'monospace',
                  color: '#1e3a8a',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {`DEVICE_ID=${newKeyData.deviceId}\nSERVER_URL=http://172.22.1.33:8443\nAPI_KEY=${newKeyData.apiKey}`}
              </pre>
            </div>

            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
              <button
                className="btn-primary"
                onClick={copyApiKey}
                style={{ minWidth: '120px' }}
              >
                {copied ? '✓ คัดลอกแล้ว' : 'คัดลอก API Key'}
              </button>
              <button
                className="btn-secondary"
                onClick={() => setNewKeyData(null)}
              >
                ปิดหน้าต่าง
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
