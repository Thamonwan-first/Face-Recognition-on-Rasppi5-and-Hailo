import React, { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';

export default function SettingsPage() {
  const queryClient = useQueryClient();

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

  const canEditSystemSettings = hasPerm('settings');

  // Active tab: 'system' or 'password'
  const [activeTab, setActiveTab] = useState(canEditSystemSettings ? 'system' : 'password');

  // System Settings Form
  const [form, setForm] = useState({
    work_start_time: '08:30',
    work_end_time: '17:30',
    min_work_hours: 8.0,
    debounce_seconds: 60,
    recognition_threshold: 0.65,
  });
  const [systemSuccess, setSystemSuccess] = useState('');
  const [systemError, setSystemError] = useState('');

  // Change Password Form
  const [passwordForm, setPasswordForm] = useState({
    old_password: '',
    new_password: '',
    confirm_password: '',
  });
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordLoading, setPasswordLoading] = useState(false);

  // Fetch System Settings
  const { data: settings, isLoading, error } = useQuery({
    queryKey: ['system_settings'],
    queryFn: () => api.get('/api/settings'),
    enabled: canEditSystemSettings,
  });

  useEffect(() => {
    if (settings) {
      setForm({
        work_start_time: settings.work_start_time || '08:30',
        work_end_time: settings.work_end_time || '17:30',
        min_work_hours: settings.min_work_hours ?? 8.0,
        debounce_seconds: settings.debounce_seconds ?? 60,
        recognition_threshold: settings.recognition_threshold ?? 0.65,
      });
    }
  }, [settings]);

  const updateSettingsMutation = useMutation({
    mutationFn: (data) => api.put('/api/settings', data),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['system_settings'] });
      queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
      setSystemSuccess(res.message || 'บันทึกการตั้งค่าระบบเรียบร้อยแล้ว');
      setSystemError('');
      setTimeout(() => setSystemSuccess(''), 4000);
    },
    onError: (err) => {
      setSystemError(err.message || 'ไม่สามารถบันทึกการตั้งค่าได้');
      setSystemSuccess('');
    },
  });

  const handleSystemSubmit = (e) => {
    e.preventDefault();
    setSystemSuccess('');
    setSystemError('');

    updateSettingsMutation.mutate({
      work_start_time: form.work_start_time,
      work_end_time: form.work_end_time,
      min_work_hours: parseFloat(form.min_work_hours),
      debounce_seconds: parseInt(form.debounce_seconds, 10),
      recognition_threshold: parseFloat(form.recognition_threshold),
    });
  };

  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');

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
      setPasswordSuccess('เปลี่ยนรหัสผ่านสำเร็จเรียบร้อยแล้ว');
      setPasswordForm({ old_password: '', new_password: '', confirm_password: '' });
      setTimeout(() => setPasswordSuccess(''), 4000);
    } catch (err) {
      setPasswordError(err.message || 'เปลี่ยนรหัสผ่านไม่สำเร็จ');
    } finally {
      setPasswordLoading(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">การตั้งค่า</h1>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.9rem', margin: '0.25rem 0 0' }}>
          จัดการข้อมูลความปลอดภัยบัญชีผู้ดูแลระบบ และกำหนดค่าพารามิเตอร์การทำงานของระบบ
        </p>
      </div>

      {/* แท็บเมนูการตั้งค่า */}
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.5rem', borderBottom: '1px solid #e2e8f0', paddingBottom: '0.5rem' }}>
        {canEditSystemSettings && (
          <button
            onClick={() => setActiveTab('system')}
            style={{
              padding: '0.6rem 1.25rem',
              borderRadius: '6px',
              border: 'none',
              cursor: 'pointer',
              fontWeight: 600,
              fontSize: '0.9rem',
              backgroundColor: activeTab === 'system' ? '#2563eb' : '#f1f5f9',
              color: activeTab === 'system' ? '#ffffff' : '#475569',
              transition: 'all 0.2s',
            }}
          >
            ตั้งค่าระบบเวลาทำงาน & AI
          </button>
        )}

        <button
          onClick={() => setActiveTab('password')}
          style={{
            padding: '0.6rem 1.25rem',
            borderRadius: '6px',
            border: 'none',
            cursor: 'pointer',
            fontWeight: 600,
            fontSize: '0.9rem',
            backgroundColor: activeTab === 'password' ? '#2563eb' : '#f1f5f9',
            color: activeTab === 'password' ? '#ffffff' : '#475569',
            transition: 'all 0.2s',
          }}
        >
          บัญชีของฉัน (เปลี่ยนรหัสผ่าน)
        </button>
      </div>

      {/* แท็บที่ 1: ตั้งค่าระบบเวลาทำงาน & AI */}
      {activeTab === 'system' && canEditSystemSettings && (
        <div style={{ maxWidth: '680px' }}>
          {systemSuccess && (
            <div
              style={{
                background: '#dcfce7',
                color: '#166534',
                padding: '0.75rem 1rem',
                borderRadius: '6px',
                marginBottom: '1.25rem',
                fontSize: '0.9rem',
                fontWeight: 500,
              }}
            >
              ✓ {systemSuccess}
            </div>
          )}

          {systemError && (
            <div
              style={{
                background: '#fee2e2',
                color: '#b91c1c',
                padding: '0.75rem 1rem',
                borderRadius: '6px',
                marginBottom: '1.25rem',
                fontSize: '0.9rem',
              }}
            >
              {systemError}
            </div>
          )}

          {isLoading && <p>กำลังโหลดการตั้งค่าระบบ...</p>}
          {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

          <form onSubmit={handleSystemSubmit}>
            {/* กล่องเกณฑ์เวลาทำงาน */}
            <div
              style={{
                background: '#fff',
                borderRadius: '8px',
                padding: '1.5rem',
                marginBottom: '1.5rem',
                boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
              }}
            >
              <h2 style={{ fontSize: '1.1rem', margin: '0 0 1rem', color: '#111827' }}>
                เกณฑ์เวลาทำงานและการลงเวลา
              </h2>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1.25rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                    เวลาเข้างานมาตรฐาน *
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={form.work_start_time}
                    onChange={(e) => setForm({ ...form, work_start_time: e.target.value })}
                    required
                  />
                  <span style={{ fontSize: '0.78rem', color: '#6b7280', marginTop: '0.25rem', display: 'block' }}>
                    เข้างานหลังเวลานี้จะถูกจัดสถานะเป็น "มาสาย"
                  </span>
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                    เวลาเลิกงานมาตรฐาน *
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={form.work_end_time}
                    onChange={(e) => setForm({ ...form, work_end_time: e.target.value })}
                    required
                  />
                  <span style={{ fontSize: '0.78rem', color: '#6b7280', marginTop: '0.25rem', display: 'block' }}>
                    เวลาสิ้นสุดกะการทำงานปกติ
                  </span>
                </div>
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                  ชั่วโมงทำงานขั้นต่ำต่อวัน (ชั่วโมง) *
                </label>
                <input
                  type="number"
                  step="0.5"
                  min="1"
                  max="24"
                  className="input-field"
                  style={{ maxWidth: '200px' }}
                  value={form.min_work_hours}
                  onChange={(e) => setForm({ ...form, min_work_hours: e.target.value })}
                  required
                />
                <span style={{ fontSize: '0.78rem', color: '#6b7280', marginTop: '0.25rem', display: 'block' }}>
                  หากลงเวลาออกงานแล้วชั่วโมงรวมน้อยกว่าเกณฑ์นี้ จะถูกจัดเป็น "ไม่ครบเวลา"
                </span>
              </div>
            </div>

            {/* กล่องตั้งค่าเครื่อง Kiosk */}
            <div
              style={{
                background: '#fff',
                borderRadius: '8px',
                padding: '1.5rem',
                marginBottom: '1.5rem',
                boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
              }}
            >
              <h2 style={{ fontSize: '1.1rem', margin: '0 0 1rem', color: '#111827' }}>
                พารามิเตอร์การจดจำใบหน้าของเครื่องสแกน (Edge Kiosk)
              </h2>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                    ช่วงห้ามสแกนซ้ำ (Debounce) (วินาที) *
                  </label>
                  <input
                    type="number"
                    min="5"
                    max="600"
                    className="input-field"
                    value={form.debounce_seconds}
                    onChange={(e) => setForm({ ...form, debounce_seconds: e.target.value })}
                    required
                  />
                  <span style={{ fontSize: '0.78rem', color: '#6b7280', marginTop: '0.25rem', display: 'block' }}>
                    ป้องกันการบันทึกซ้ำซ้อนเมื่อพนักงานยืนหน้ากล้องต่อเนื่อง
                  </span>
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                    เกณฑ์ความแม่นยำใบหน้า (Cosine Threshold) *
                  </label>
                  <input
                    type="number"
                    step="0.01"
                    min="0.30"
                    max="0.95"
                    className="input-field"
                    value={form.recognition_threshold}
                    onChange={(e) => setForm({ ...form, recognition_threshold: e.target.value })}
                    required
                  />
                  <span style={{ fontSize: '0.78rem', color: '#6b7280', marginTop: '0.25rem', display: 'block' }}>
                    ค่าระหว่าง 0.50 - 0.70 (ค่ามาตรฐานคือ 0.65)
                  </span>
                </div>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
              <button
                type="submit"
                style={{
                  backgroundColor: '#2563eb',
                  color: '#ffffff',
                  border: 'none',
                  borderRadius: '8px',
                  padding: '0.65rem 1.5rem',
                  fontSize: '0.9rem',
                  fontWeight: 500,
                  cursor: 'pointer',
                  boxShadow: '0 2px 4px rgba(37, 99, 235, 0.2)',
                }}
                disabled={updateSettingsMutation.isPending}
              >
                {updateSettingsMutation.isPending ? 'กำลังบันทึก...' : 'บันทึกการตั้งค่าระบบ'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* แท็บที่ 2: บัญชีของฉัน (เปลี่ยนรหัสผ่าน) */}
      {activeTab === 'password' && (
        <div style={{ maxWidth: '520px' }}>
          <div
            style={{
              background: '#fff',
              borderRadius: '8px',
              padding: '1.5rem',
              boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
            }}
          >
            <h2 style={{ fontSize: '1.1rem', margin: '0 0 0.5rem', color: '#111827' }}>
              เปลี่ยนรหัสผ่านบัญชีผู้ใช้งาน
            </h2>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1.25rem' }}>
              บัญชีปัจจุบัน: <strong>{currentUser.username}</strong> ({currentUser.role === 'admin' ? 'ผู้ดูแลระบบสูงสุด' : 'ผู้ใช้งานแดชบอร์ด'})
            </p>

            {passwordSuccess && (
              <div
                style={{
                  background: '#dcfce7',
                  color: '#166534',
                  padding: '0.75rem 1rem',
                  borderRadius: '6px',
                  marginBottom: '1.25rem',
                  fontSize: '0.9rem',
                  fontWeight: 500,
                }}
              >
                ✓ {passwordSuccess}
              </div>
            )}

            {passwordError && (
              <div
                style={{
                  background: '#fee2e2',
                  color: '#b91c1c',
                  padding: '0.75rem 1rem',
                  borderRadius: '6px',
                  marginBottom: '1.25rem',
                  fontSize: '0.9rem',
                }}
              >
                {passwordError}
              </div>
            )}

            <form onSubmit={handlePasswordSubmit}>
              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                  รหัสผ่านปัจจุบัน *
                </label>
                <input
                  type="password"
                  className="input-field"
                  placeholder="กรอกรหัสผ่านปัจจุบันของคุณ"
                  value={passwordForm.old_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, old_password: e.target.value })}
                  required
                />
              </div>

              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                  รหัสผ่านใหม่ *
                </label>
                <input
                  type="password"
                  className="input-field"
                  placeholder="ความยาวอย่างน้อย 6 ตัวอักษร"
                  value={passwordForm.new_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, new_password: e.target.value })}
                  required
                />
              </div>

              <div className="form-group" style={{ marginBottom: '1.5rem' }}>
                <label style={{ display: 'block', fontSize: '0.88rem', fontWeight: 500, marginBottom: '0.4rem' }}>
                  ยืนยันรหัสผ่านใหม่ *
                </label>
                <input
                  type="password"
                  className="input-field"
                  placeholder="กรอกรหัสผ่านใหม่อีกครั้งให้ตรงกัน"
                  value={passwordForm.confirm_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, confirm_password: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                <button
                  type="submit"
                  style={{
                    backgroundColor: '#2563eb',
                    color: '#ffffff',
                    border: 'none',
                    borderRadius: '8px',
                    padding: '0.65rem 1.5rem',
                    fontSize: '0.9rem',
                    fontWeight: 500,
                    cursor: 'pointer',
                    boxShadow: '0 2px 4px rgba(37, 99, 235, 0.2)',
                  }}
                  disabled={passwordLoading}
                >
                  {passwordLoading ? 'กำลังบันทึก...' : 'บันทึกรหัสผ่านใหม่'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
