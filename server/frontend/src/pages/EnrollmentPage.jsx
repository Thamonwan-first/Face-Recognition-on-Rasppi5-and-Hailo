import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';
import { formatThaiDateTime } from '../utils/date';

export default function EnrollmentPage() {
  const queryClient = useQueryClient();
  const [selectedUserId, setSelectedUserId] = useState('');
  const [imageFiles, setImageFiles] = useState([]);

  // ดึงเฉพาะพนักงานที่ "ยังไม่ได้ลงทะเบียนใบหน้า"
  const { data: users } = useQuery({
    queryKey: ['users', 'unenrolled'],
    queryFn: () => api.get('/api/users?unenrolled_only=true'),
  });

  const { data: jobs, isLoading } = useQuery({
    queryKey: ['enrollment_jobs'],
    queryFn: () => api.get('/api/enrollment/pending'),
    refetchInterval: 2000, // Real-time poll every 2s
  });

  const handleFileChange = (e) => {
    const files = Array.from(e.target.files);
    if (files.length > 5) {
      alert('สามารถเลือกได้สูงสุดไม่เกิน 5 รูปครับ ระบบจะเลือกเฉพาะ 5 รูปแรกให้');
      setImageFiles(files.slice(0, 5));
    } else {
      setImageFiles(files);
    }
  };

  const uploadMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData();
      formData.append('user_id', selectedUserId);
      for (const file of imageFiles) {
        formData.append('images', file);
      }
      return api.post('/api/enrollment', formData, true);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setImageFiles([]);
      setSelectedUserId('');
      alert(`สร้างคำขอลงทะเบียนสำเร็จ (${imageFiles.length} รูป) — เครื่อง Kiosk จะนำภาพไปคำนวณเวกเตอร์ใบหน้าเฉลี่ยบนชิป AI และลบรูปภาพทิ้งอัตโนมัติ`);
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการสร้างคำขอ: ' + (err.message || 'ไม่สามารถเชื่อมต่อเซิร์ฟเวอร์ได้'));
    },
  });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!selectedUserId) {
      alert('กรุณาเลือกพนักงานก่อน');
      return;
    }
    if (imageFiles.length === 0) {
      alert('กรุณาเลือกรูปถ่ายอย่างน้อย 1 รูป (แนะนำ 3-5 รูป)');
      return;
    }
    uploadMutation.mutate();
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">📸 ลงทะเบียนใบหน้าผ่านแดชบอร์ด (Web Enrollment)</h1>
      </div>

      <div style={{ background: '#fff', padding: '1.5rem', borderRadius: '8px', marginBottom: '2rem', maxWidth: '600px' }}>
        <h3 style={{ marginBottom: '1rem', fontSize: '1.1rem' }}>อัปโหลดภาพถ่ายพนักงาน</h3>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1.25rem' }}>
          * ระบบจะไม่เก็บภาพถ่ายนี้ถาวร ภาพจะถูกส่งไปยังชิป AI บน Kiosk เพื่อแปลงเป็น Face Embedding เฉลี่ย และลบภาพทิ้งทันทีตามนโยบาย PDPA
        </p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>เลือกพนักงาน (แสดงเฉพาะคนที่ยังไม่ลงทะเบียน)</label>
            <select
              className="input-field"
              value={selectedUserId}
              onChange={(e) => setSelectedUserId(e.target.value)}
              required
            >
              <option value="">-- กรุณาเลือกพนักงาน --</option>
              {users?.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.employee_id ? `[${u.employee_id}] ` : ''}{u.name}
                </option>
              ))}
            </select>
            {users?.length === 0 && (
              <p style={{ fontSize: '0.85rem', color: 'var(--success)', marginTop: '0.4rem', fontWeight: 'bold' }}>
                🎉 พนักงานทุกคนในระบบลงทะเบียนใบหน้าครบเรียบร้อยแล้ว
              </p>
            )}
          </div>

          <div className="form-group">
            <label>เลือกรูปถ่ายใบหน้า (เลือกได้สูงสุด 5 รูป — มุมหน้าตรง / เอียงเล็กน้อย)</label>
            <input
              type="file"
              accept="image/*"
              multiple
              className="input-field"
              onChange={handleFileChange}
            />
            {imageFiles.length > 0 && (
              <p style={{ fontSize: '0.85rem', color: 'var(--primary)', marginTop: '0.4rem', fontWeight: 'bold' }}>
                📷 เลือกแล้ว {imageFiles.length} รูป จากสูงสุด 5 รูป
              </p>
            )}
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={uploadMutation.isPending || users?.length === 0}
          >
            {uploadMutation.isPending ? 'กำลังอัปโหลด...' : `ส่งคำขอสร้าง Embedding (${imageFiles.length} รูป)`}
          </button>
        </form>
      </div>

      <h3 style={{ marginBottom: '1rem' }}>สถานะคิวงานประมวลผล (Enrollment Jobs Queue)</h3>
      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th style={{ width: '90px' }}>ลำดับคิว</th>
              <th>พนักงาน</th>
              <th>สถานะ</th>
              <th>Kiosk ที่รับงาน</th>
              <th>สร้างเมื่อ</th>
              <th>เสร็จสิ้นเมื่อ</th>
            </tr>
          </thead>
          <tbody>
            {jobs?.map((j, idx) => (
              <tr key={j.id}>
                <td>
                  <span
                    style={{
                      display: 'inline-block',
                      padding: '2px 8px',
                      background: '#e0f2fe',
                      color: 'var(--primary)',
                      borderRadius: '12px',
                      fontWeight: 'bold',
                      fontSize: '0.85rem',
                    }}
                    title={`รหัสอ้างอิง: ${j.id}`}
                  >
                    #{idx + 1}
                  </span>
                </td>
                <td>{j.user_name || j.user_id}</td>
                <td>
                  <StatusBadge status={j.status} />
                </td>
                <td>{j.assigned_device_id || 'รอเครื่อง Kiosk ดึงงาน'}</td>
                <td>{formatThaiDateTime(j.created_at)}</td>
                <td>{formatThaiDateTime(j.processed_at)}</td>
              </tr>
            ))}
            {jobs?.length === 0 && (
              <tr>
                <td colSpan="6" style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                  ไม่มีคิวงานค้างอยู่
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
