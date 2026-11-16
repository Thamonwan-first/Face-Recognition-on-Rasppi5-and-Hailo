import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';
import { formatThaiDateTime } from '../utils/date';
import { compressImage } from '../utils/imageCompress';

export default function EnrollmentPage() {
  const queryClient = useQueryClient();
  const [selectedUserId, setSelectedUserId] = useState('');
  const [imageFiles, setImageFiles] = useState([]);
  const [isCompressing, setIsCompressing] = useState(false);

  // ดึงรายชื่อพนักงานทั้งหมด เพื่อให้สามารถลงทะเบียนใหม่ หรืออัปเดตรูปเดิมได้
  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
  });

  const { data: jobs } = useQuery({
    queryKey: ['enrollment_jobs'],
    queryFn: () => api.get('/api/enrollment/pending'),
  });

  const handleFileChange = async (e) => {
    const rawFiles = Array.from(e.target.files);
    if (rawFiles.length > 5) {
      alert('สามารถเลือกได้สูงสุดไม่เกิน 5 รูปครับ ระบบจะเลือกเฉพาะ 5 รูปแรกให้');
    }
    const selected = rawFiles.slice(0, 5);
    if (selected.length === 0) {
      setImageFiles([]);
      return;
    }
    setIsCompressing(true);
    try {
      const compressed = await Promise.all(selected.map((f) => compressImage(f)));
      setImageFiles(compressed);
    } catch (err) {
      console.warn('Image compression fallback:', err);
      setImageFiles(selected);
    } finally {
      setIsCompressing(false);
    }
  };

  const uploadMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData();
      formData.append('user_id', selectedUserId);
      for (const file of imageFiles) {
        formData.append('images', file);
      }
      if (imageFiles.length > 0) {
        formData.append('image', imageFiles[0]);
      }
      return api.post('/api/enrollment', formData, true);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setImageFiles([]);
      setSelectedUserId('');
      alert(`ส่งคำขอลงทะเบียนใบหน้าสำเร็จ (${imageFiles.length} รูป) — ตู้ Kiosk จะนำภาพไปคำนวณเวกเตอร์ใบหน้าเฉลี่ยบนชิป AI และลบภาพทิ้งอัตโนมัติตาม PDPA`);
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
        <h1 className="page-title">ลงทะเบียนใบหน้าผ่านแดชบอร์ด</h1>
      </div>

      <div style={{ background: '#fff', padding: '1.5rem', borderRadius: '8px', marginBottom: '2rem', maxWidth: '600px' }}>
        <h3 style={{ marginBottom: '1rem', fontSize: '1.1rem' }}>อัปโหลดภาพถ่ายพนักงาน</h3>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1.25rem' }}>
          * ระบบจะไม่เก็บภาพถ่ายนี้ถาวร ภาพจะถูกส่งไปยังชิป AI บน Kiosk เพื่อแปลงเป็น Face Embedding เฉลี่ย และลบภาพทิ้งทันทีตามนโยบาย PDPA
        </p>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>เลือกพนักงาน</label>
            <select
              className="input-field"
              value={selectedUserId}
              onChange={(e) => setSelectedUserId(e.target.value)}
              required
            >
              <option value="">-- กรุณาเลือกพนักงาน --</option>
              {users?.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.employee_id ? `[${u.employee_id}] ` : ''}{u.name} {u.has_embedding ? '✓ (มีรูปแล้ว - เลือกเพื่อเปลี่ยนรูป)' : '• (ยังไม่มีรูป)'}
                </option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <label>เลือกรูปถ่ายใบหน้า (เลือกได้ 1–5 รูป — มุมหน้าตรง / เอียงเล็กน้อย)</label>
            <input
              type="file"
              accept="image/*"
              multiple
              className="input-field"
              onChange={handleFileChange}
            />
            {imageFiles.length > 0 && (
              <p style={{ fontSize: '0.85rem', color: 'var(--primary)', marginTop: '0.4rem', fontWeight: 'bold' }}>
                ✓ เลือกรูปถ่ายแล้ว {imageFiles.length} รูป (พร้อมส่งประมวลผล)
              </p>
            )}
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={uploadMutation.isPending || isCompressing || !selectedUserId || imageFiles.length === 0}
          >
            {isCompressing
              ? 'กำลังเตรียมรูปภาพ...'
              : uploadMutation.isPending
              ? 'กำลังอัปโหลด...'
              : `ส่งคำขอสร้าง Embedding (${imageFiles.length} รูป)`}
          </button>
        </form>
      </div>

      <h3 style={{ marginBottom: '1rem' }}>สถานะคิวงานประมวลผล</h3>
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
