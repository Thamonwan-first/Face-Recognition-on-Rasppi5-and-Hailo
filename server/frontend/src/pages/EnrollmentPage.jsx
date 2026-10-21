import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';

export default function EnrollmentPage() {
  const queryClient = useQueryClient();
  const [selectedUserId, setSelectedUserId] = useState('');
  const [imageFile, setImageFile] = useState(null);

  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
  });

  const { data: jobs, isLoading } = useQuery({
    queryKey: ['enrollment_jobs'],
    queryFn: () => api.get('/api/enrollment/pending'),
    refetchInterval: 10000,
  });

  const uploadMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData();
      formData.append('user_id', selectedUserId);
      formData.append('image', imageFile);
      return api.post('/api/enrollment', formData, true);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
      setImageFile(null);
      setSelectedUserId('');
      alert('สร้างคำขอลงทะเบียนสำเร็จ — เครื่อง Kiosk ที่ออนไลน์จะดึงภาพไปคำนวณ Embedding บนชิป Hailo และลบรูปภาพทิ้งอัตโนมัติ');
    },
  });

  const handleSubmit = (e) => {
    e.preventDefault();
    if (!selectedUserId || !imageFile) return;
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
          * ระบบจะไม่เก็บภาพถ่ายนี้ถาวร ภาพจะถูกส่งไปยังชิป Hailo บน Kiosk เพื่อแปลงเป็น Face Embedding และลบภาพทิ้งภายใน 24 ชม. ตามนโยบาย PDPA
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
                  {u.employee_id ? `[${u.employee_id}] ` : ''}{u.name}
                </option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <label>เลือกรูปถ่ายใบหน้า (หน้าตรง ชัดเจน แสงเพียงพอ)</label>
            <input
              type="file"
              accept="image/*"
              className="input-field"
              onChange={(e) => setImageFile(e.target.files[0])}
              required
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={uploadMutation.isPending || !selectedUserId || !imageFile}
          >
            {uploadMutation.isPending ? 'กำลังอัปโหลด...' : 'ส่งคำขอสร้าง Embedding'}
          </button>
        </form>
      </div>

      <h3 style={{ marginBottom: '1rem' }}>สถานะคิวงานประมวลผล (Enrollment Jobs Queue)</h3>
      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>Job ID</th>
              <th>พนักงาน</th>
              <th>สถานะ</th>
              <th>Kiosk ที่รับงาน</th>
              <th>สร้างเมื่อ</th>
              <th>เสร็จสิ้นเมื่อ</th>
            </tr>
          </thead>
          <tbody>
            {jobs?.map((j) => (
              <tr key={j.id}>
                <td>{j.id.slice(0, 8)}...</td>
                <td>{j.user_name || j.user_id}</td>
                <td>
                  <StatusBadge status={j.status} />
                </td>
                <td>{j.assigned_device_id || 'รอเครื่อง Kiosk ดึงงาน'}</td>
                <td>{new Date(j.created_at).toLocaleString('th-TH')}</td>
                <td>{j.processed_at ? new Date(j.processed_at).toLocaleString('th-TH') : '-'}</td>
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
