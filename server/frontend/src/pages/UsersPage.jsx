import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';

export default function UsersPage() {
  const queryClient = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [formData, setFormData] = useState({ employee_id: '', name: '', department: '' });

  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
  });

  const createUserMutation = useMutation({
    mutationFn: (newUser) => api.post('/api/users', newUser),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setShowModal(false);
      setFormData({ employee_id: '', name: '', department: '' });
    },
  });

  const deleteUserMutation = useMutation({
    mutationFn: (id) => api.del(`/api/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
  });

  const handleCreate = (e) => {
    e.preventDefault();
    createUserMutation.mutate(formData);
  };

  const handleDelete = (id, name) => {
    if (window.confirm(`ยืนยันการลบผู้ใช้ "${name}"? ข้อมูลจะถูกลบออกจากเครื่อง Kiosk ทั้งหมดในการซิงก์รอบถัดไป`)) {
      deleteUserMutation.mutate(id);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">👤 จัดการผู้ใช้งาน (Users)</h1>
        <button onClick={() => setShowModal(true)} className="btn btn-primary">
          ➕ เพิ่มผู้ใช้งานใหม่
        </button>
      </div>

      {isLoading && <p>กำลังโหลดรายชื่อผู้ใช้...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>รหัสพนักงาน</th>
              <th>ชื่อ-นามสกุล</th>
              <th>แผนก</th>
              <th>สถานะ</th>
              <th>Sync Version</th>
              <th>จัดการ</th>
            </tr>
          </thead>
          <tbody>
            {users?.map((u) => (
              <tr key={u.id}>
                <td>{u.employee_id || '-'}</td>
                <td>{u.name}</td>
                <td>{u.department || '-'}</td>
                <td>
                  <StatusBadge status={u.status} />
                </td>
                <td>v{u.version}</td>
                <td>
                  <button
                    onClick={() => handleDelete(u.id, u.name)}
                    className="btn btn-danger"
                    style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem' }}
                  >
                    ลบ
                  </button>
                </td>
              </tr>
            ))}
            {users?.length === 0 && (
              <tr>
                <td colSpan="6" style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                  ยังไม่มีผู้ใช้ในระบบ
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {showModal && (
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
          <div className="login-card" style={{ maxWidth: '450px' }}>
            <h3 style={{ marginBottom: '1rem' }}>เพิ่มผู้ใช้งานใหม่</h3>
            <form onSubmit={handleCreate}>
              <div className="form-group">
                <label>รหัสพนักงาน</label>
                <input
                  type="text"
                  className="input-field"
                  value={formData.employee_id}
                  onChange={(e) => setFormData({ ...formData, employee_id: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>ชื่อ-นามสกุล</label>
                <input
                  type="text"
                  className="input-field"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>แผนก / ฝ่าย</label>
                <input
                  type="text"
                  className="input-field"
                  value={formData.department}
                  onChange={(e) => setFormData({ ...formData, department: e.target.value })}
                />
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="btn"
                  style={{ background: '#eee' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={createUserMutation.isPending}
                >
                  {createUserMutation.isPending ? 'กำลังบันทึก...' : 'บันทึก'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
