import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDateTime, formatThaiDate } from '../utils/date';

export const AVAILABLE_MENUS = [
  { key: 'devices', label: 'รายการอุปกรณ์' },
  { key: 'users', label: 'ข้อมูลพนักงาน' },
  { key: 'enrollment', label: 'คิวลงทะเบียนใบหน้า' },
  { key: 'attendance', label: 'ประวัติการลงเวลา' },
  { key: 'audit', label: 'บันทึกประวัติการใช้งาน' },
  { key: 'admins', label: 'จัดการผู้ดูแลระบบ' },
];

export const PRESETS = [
  {
    name: 'เลือกทั้งหมด',
    perms: ['devices', 'users', 'enrollment', 'attendance', 'audit', 'admins'],
  },
  {
    name: 'ฝ่ายบุคคล',
    perms: ['users', 'enrollment', 'attendance'],
  },
  {
    name: 'ฝ่ายเทคนิคและไอที',
    perms: ['devices', 'audit'],
  },
  {
    name: 'ผู้เข้าชมรายงาน',
    perms: ['attendance'],
  },
];

const INITIAL_FORM = {
  username: '',
  password: '',
  role: 'viewer',
  permissions: ['attendance'],
};

export default function AdminsPage() {
  const queryClient = useQueryClient();
  const [showModal, setShowModal] = useState(false);
  const [editAdmin, setEditAdmin] = useState(null);
  const [formData, setFormData] = useState(INITIAL_FORM);

  let currentLoggedInUser = null;
  try {
    const stored = localStorage.getItem('user');
    if (stored) currentLoggedInUser = JSON.parse(stored);
  } catch (e) {
    // fallback
  }

  const { data: admins, isLoading, error } = useQuery({
    queryKey: ['admins'],
    queryFn: () => api.get('/api/admins'),
  });

  const createAdminMutation = useMutation({
    mutationFn: (newAdmin) => api.post('/api/admins', newAdmin),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admins'] });
      setShowModal(false);
      setFormData(INITIAL_FORM);
      alert('สร้างบัญชีผู้ดูแลระบบเรียบร้อยแล้ว');
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาด: ' + (err.message || err));
    },
  });

  const updateAdminMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/api/admins/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admins'] });
      setEditAdmin(null);
      setShowModal(false);
      setFormData(INITIAL_FORM);
      alert('บันทึกการแก้ไขสิทธิ์เรียบร้อยแล้ว');
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาด: ' + (err.message || err));
    },
  });

  const deleteAdminMutation = useMutation({
    mutationFn: (id) => api.del(`/api/admins/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admins'] });
      alert('ลบบัญชีผู้ใช้งานเรียบร้อยแล้ว');
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการลบ: ' + (err.message || err));
    },
  });

  const openCreateModal = () => {
    setEditAdmin(null);
    setFormData(INITIAL_FORM);
    setShowModal(true);
  };

  const openEditModal = (a) => {
    setEditAdmin(a);
    setFormData({
      username: a.username,
      password: '',
      role: a.role || 'viewer',
      permissions: a.role === 'admin' || a.permissions?.includes('*')
        ? AVAILABLE_MENUS.map((m) => m.key)
        : a.permissions || [],
    });
    setShowModal(true);
  };

  const handleDelete = (id, username) => {
    if (window.confirm(`ยืนยันการลบบัญชีผู้ดูแล "${username}"?`)) {
      deleteAdminMutation.mutate(id);
    }
  };

  const handleToggleMenu = (menuKey) => {
    setFormData((prev) => {
      const exists = prev.permissions.includes(menuKey);
      const nextPerms = exists
        ? prev.permissions.filter((k) => k !== menuKey)
        : [...prev.permissions, menuKey];
      return { ...prev, permissions: nextPerms };
    });
  };

  const applyPreset = (presetPerms) => {
    setFormData((prev) => ({
      ...prev,
      permissions: [...presetPerms],
    }));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    if (editAdmin) {
      updateAdminMutation.mutate({
        id: editAdmin.id,
        data: {
          role: formData.role,
          permissions: formData.role === 'admin' ? ['*'] : formData.permissions,
        },
      });
    } else {
      createAdminMutation.mutate({
        username: formData.username.trim(),
        password: formData.password,
        role: formData.role,
        permissions: formData.role === 'admin' ? ['*'] : formData.permissions,
      });
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">จัดการผู้ดูแลระบบ</h1>
        <button onClick={openCreateModal} className="btn btn-primary">
          เพิ่มผู้ดูแลระบบ
        </button>
      </div>

      <p style={{ color: 'var(--text-muted)', marginBottom: '1.5rem', fontSize: '0.9rem' }}>
        กำหนดรายชื่อผู้ใช้งานและเลือกเปิด-ปิดเมนูที่แต่ละคนสามารถเข้าถึงได้อย่างอิสระ
      </p>

      {isLoading && <p>กำลังโหลดรายชื่อผู้ดูแลระบบ...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>ชื่อผู้ใช้งาน</th>
              <th>บทบาทหลัก</th>
              <th>เมนูที่ได้รับสิทธิ์เข้าถึง</th>
              <th>เข้าใช้งานล่าสุด</th>
              <th>วันที่สร้าง</th>
              <th>จัดการ</th>
            </tr>
          </thead>
          <tbody>
            {admins?.map((a) => {
              const isSuper = a.role === 'admin' || a.permissions?.includes('*');
              const isSelf = currentLoggedInUser && currentLoggedInUser.id === a.id;
              const isDefaultAdmin = a.username === 'admin';

              return (
                <tr key={a.id}>
                  <td>
                    <strong style={{ color: 'var(--primary)' }}>{a.username}</strong>
                    {isSelf && (
                      <span
                        style={{
                          marginLeft: '0.5rem',
                          fontSize: '0.75rem',
                          padding: '2px 6px',
                          background: '#e0f2fe',
                          color: '#0369a1',
                          borderRadius: '10px',
                          fontWeight: 600,
                        }}
                      >
                        บัญชีของคุณ
                      </span>
                    )}
                  </td>
                  <td>
                    <span
                      style={{
                        padding: '0.2rem 0.6rem',
                        borderRadius: '12px',
                        fontSize: '0.8rem',
                        fontWeight: 600,
                        background: isSuper ? '#dbeafe' : '#f1f5f9',
                        color: isSuper ? '#1e40af' : '#475569',
                      }}
                    >
                      {isSuper ? 'ผู้ดูแลระบบสูงสุด' : 'ผู้ใช้งานแดชบอร์ด'}
                    </span>
                  </td>
                  <td>
                    {isSuper ? (
                      <span style={{ fontSize: '0.85rem', color: '#16a34a', fontWeight: 600 }}>
                        เข้าถึงได้ทุกเมนู
                      </span>
                    ) : (
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
                        {AVAILABLE_MENUS.map((m) => {
                          const hasIt = a.permissions?.includes(m.key);
                          if (!hasIt) return null;
                          return (
                            <span
                              key={m.key}
                              style={{
                                fontSize: '0.75rem',
                                padding: '2px 8px',
                                background: '#f8fafc',
                                border: '1px solid #cbd5e1',
                                borderRadius: '4px',
                                color: '#334155',
                              }}
                            >
                              {m.label}
                            </span>
                          );
                        })}
                        {(!a.permissions || a.permissions.length === 0) && (
                          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>-</span>
                        )}
                      </div>
                    )}
                  </td>
                  <td>{formatThaiDateTime(a.last_login, 'ยังไม่เคยเข้าสู่ระบบ')}</td>
                  <td>{formatThaiDate(a.created_at)}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.4rem' }}>
                      <button
                        onClick={() => openEditModal(a)}
                        className="btn"
                        style={{
                          padding: '0.3rem 0.6rem',
                          fontSize: '0.8rem',
                          background: 'var(--warning)',
                          color: '#000',
                        }}
                      >
                        แก้ไขสิทธิ์
                      </button>
                      <button
                        onClick={() => handleDelete(a.id, a.username)}
                        className="btn btn-danger"
                        disabled={isSelf || isDefaultAdmin}
                        title={isDefaultAdmin ? 'ไม่สามารถลบบัญชีหลักได้' : isSelf ? 'ไม่สามารถลบบัญชีของตนเองได้' : ''}
                        style={{
                          padding: '0.3rem 0.6rem',
                          fontSize: '0.8rem',
                          opacity: isSelf || isDefaultAdmin ? 0.4 : 1,
                          cursor: isSelf || isDefaultAdmin ? 'not-allowed' : 'pointer',
                        }}
                      >
                        ลบ
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Modal: เพิ่ม / แก้ไขสิทธิ์ผู้ดูแลระบบ */}
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
          <div className="login-card" style={{ maxWidth: '580px', width: '92%', maxHeight: '90vh', overflowY: 'auto' }}>
            <h3 style={{ marginBottom: '1rem' }}>
              {editAdmin ? `แก้ไขสิทธิ์ผู้ใช้งาน: ${editAdmin.username}` : 'เพิ่มผู้ดูแลระบบใหม่'}
            </h3>

            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label>ชื่อผู้ใช้งาน (สำหรับล็อกอิน) *</label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น hr_somsee หรือ it_support"
                  value={formData.username}
                  onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                  disabled={!!editAdmin}
                  required
                />
              </div>

              {!editAdmin && (
                <div className="form-group">
                  <label>รหัสผ่านเริ่มต้น *</label>
                  <input
                    type="password"
                    className="input-field"
                    placeholder="ความยาวอย่างน้อย 6 ตัวอักษร"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    required
                  />
                </div>
              )}

              <div className="form-group">
                <label>บทบาทบัญชี</label>
                <select
                  className="input-field"
                  value={formData.role}
                  onChange={(e) => setFormData({ ...formData, role: e.target.value })}
                >
                  <option value="viewer">ผู้ใช้งานแดชบอร์ด (กำหนดสิทธิ์ตามเมนูด้านล่าง)</option>
                  <option value="admin">ผู้ดูแลระบบสูงสุด (เข้าถึงได้ทุกเมนูโดยอัตโนมัติ)</option>
                </select>
              </div>

              {formData.role !== 'admin' && (
                <div style={{ marginTop: '1.25rem', marginBottom: '1.25rem' }}>
                  <label style={{ display: 'block', fontWeight: 600, marginBottom: '0.5rem', fontSize: '0.9rem' }}>
                    เลือกเปิด-ปิดเมนูที่อนุญาตให้เข้าถึง:
                  </label>

                  {/* ปุ่มลัดเลือกสิทธิ์ด่วน */}
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginBottom: '0.85rem' }}>
                    <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', alignSelf: 'center' }}>
                      เลือกด่วน:
                    </span>
                    {PRESETS.map((p) => (
                      <button
                        key={p.name}
                        type="button"
                        onClick={() => applyPreset(p.perms)}
                        className="btn"
                        style={{
                          padding: '0.2rem 0.55rem',
                          fontSize: '0.75rem',
                          background: '#f1f5f9',
                          border: '1px solid #cbd5e1',
                          color: '#334155',
                        }}
                      >
                        {p.name}
                      </button>
                    ))}
                  </div>

                  {/* รายการ Checkbox 6 เมนู */}
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr 1fr',
                      gap: '0.6rem',
                      background: '#f8fafc',
                      padding: '1rem',
                      borderRadius: '6px',
                      border: '1px solid #e2e8f0',
                    }}
                  >
                    {AVAILABLE_MENUS.map((m) => {
                      const isChecked = formData.permissions.includes(m.key);
                      return (
                        <label
                          key={m.key}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: '0.5rem',
                            fontSize: '0.88rem',
                            cursor: 'pointer',
                          }}
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => handleToggleMenu(m.key)}
                            style={{ width: '16px', height: '16px', cursor: 'pointer' }}
                          />
                          <span>{m.label}</span>
                        </label>
                      );
                    })}
                  </div>
                </div>
              )}

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={() => {
                    setShowModal(false);
                    setEditAdmin(null);
                  }}
                  className="btn"
                  style={{ background: '#eee', color: '#333' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={createAdminMutation.isPending || updateAdminMutation.isPending}
                >
                  {createAdminMutation.isPending || updateAdminMutation.isPending
                    ? 'กำลังบันทึก...'
                    : 'บันทึก'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
