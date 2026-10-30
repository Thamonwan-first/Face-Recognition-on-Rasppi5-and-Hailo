import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import StatusBadge from '../components/StatusBadge';
import { formatThaiDateTime, formatThaiDate } from '../utils/date';

export const DIVISIONS = [
  'ฝ่ายระบบลำเลียงแสง',
  'ฝ่ายปฏิบัติการเครื่องกำเนิดแสงสยาม 1',
  'ฝ่ายวิจัยและพัฒนาเครื่องเร่งอนุภาค',
  'ฝ่ายพัฒนาเทคนิคและวิศวกรรม',
  'ฝ่ายพัฒนาระบบเชิงกลและสาธารณูปโภค',
  'ฝ่ายบริหารจัดการโครงการเครื่องกำเนิดแสงสยาม 2',
  'ฝ่ายพัฒนาเทคโนโลยีเครื่องกำเนิดแสงสยาม 2',
  'ฝ่ายบริหารทั่วไป',
  'ฝ่ายยุทธศาสตร์องค์กร',
];

export const TITLE_PREFIXES = [
  'นาย',
  'นาง',
  'นางสาว',
  'ดร.',
  'ผศ.ดร.',
  'รศ.ดร.',
  'ศ.ดร.',
  'ผศ.',
  'รศ.',
  'ศ.',
  'อาจารย์',
];

const INITIAL_FORM = {
  employee_id: '',
  title: 'นาย',
  first_name: '',
  last_name: '',
  department: DIVISIONS[0],
  status: 'active',
  start_date: new Date().toISOString().slice(0, 10),
  end_date: '',
};

export default function UsersPage() {
  const queryClient = useQueryClient();

  // State สำหรับ Modal เพิ่ม/แก้ไขพนักงาน
  const [showUserModal, setShowUserModal] = useState(false);
  const [editUser, setEditUser] = useState(null);
  const [formData, setFormData] = useState(INITIAL_FORM);

  // State สำหรับ Modal ลงทะเบียนใบหน้า
  const [enrollUser, setEnrollUser] = useState(null);
  const [imageFiles, setImageFiles] = useState([]);

  // State ค้นหาพนักงาน
  const [searchUser, setSearchUser] = useState('');

  // 1. ดึงรายชื่อพนักงานทั้งหมด
  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
  });

  // Mutation เพิ่มพนักงาน
  const createUserMutation = useMutation({
    mutationFn: (newUser) => api.post('/api/users', newUser),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setShowUserModal(false);
      setFormData(INITIAL_FORM);
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการเพิ่มพนักงาน: ' + (err.message || err));
    },
  });

  // Mutation แก้ไขพนักงาน
  const updateUserMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/api/users/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setEditUser(null);
      setShowUserModal(false);
      setFormData(INITIAL_FORM);
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการแก้ไขพนักงาน: ' + (err.message || err));
    },
  });

  // Mutation ลบพนักงาน
  const deleteUserMutation = useMutation({
    mutationFn: (id) => api.del(`/api/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
  });

  // Mutation อัปโหลดรูปภาพลงทะเบียนใบหน้า
  const uploadEnrollmentMutation = useMutation({
    mutationFn: async () => {
      const form = new FormData();
      form.append('user_id', enrollUser.id);
      for (const file of imageFiles) {
        form.append('images', file);
      }
      return api.post('/api/enrollment', form, true);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
      queryClient.invalidateQueries({ queryKey: ['users'] });
      alert(`ส่งภาพ (${imageFiles.length} รูป) ไปยังตู้ Kiosk สำเร็จ! ชิป AI กำลังคำนวณเวกเตอร์ใบหน้าเฉลี่ยและจะลบรูปทิ้งอัตโนมัติ`);
      setEnrollUser(null);
      setImageFiles([]);
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการส่งรูปภาพ: ' + (err.message || 'ไม่สามารถติดต่อเซิร์ฟเวอร์ได้'));
    },
  });

  const openCreateModal = () => {
    setEditUser(null);
    setFormData(INITIAL_FORM);
    setShowUserModal(true);
  };

  const openEditModal = (u) => {
    setEditUser(u);

    // แกะคำนำหน้า ชื่อ นามสกุล หากยังไม่มีฟิลด์แยก
    let title = u.title || '';
    let firstName = u.first_name || '';
    let lastName = u.last_name || '';

    if (!firstName && u.name) {
      const matched = TITLE_PREFIXES.find((p) => u.name.startsWith(p));
      let rest = u.name;
      if (matched) {
        title = matched;
        rest = u.name.slice(matched.length).trim();
      }
      const parts = rest.split(/\s+/);
      firstName = parts[0] || '';
      lastName = parts.slice(1).join(' ') || '';
    }

    setFormData({
      employee_id: u.employee_id || '',
      title: title || 'นาย',
      first_name: firstName,
      last_name: lastName,
      department: u.department || DIVISIONS[0],
      status: u.status || 'active',
      start_date: u.start_date || '',
      end_date: u.end_date || '',
    });
    setShowUserModal(true);
  };

  const handleUserSubmit = (e) => {
    e.preventDefault();
    const prefix = formData.title ? `${formData.title} ` : '';
    const fullName = `${prefix}${formData.first_name} ${formData.last_name}`.trim();
    const payload = {
      employee_id: formData.employee_id.trim(),
      title: formData.title,
      first_name: formData.first_name.trim(),
      last_name: formData.last_name.trim(),
      name: fullName,
      department: formData.department,
      status: formData.status,
      start_date: formData.start_date || null,
      end_date: formData.end_date || null,
    };

    if (editUser) {
      updateUserMutation.mutate({
        id: editUser.id,
        data: {
          ...payload,
          version: editUser.version,
        },
      });
    } else {
      createUserMutation.mutate(payload);
    }
  };

  const handleDelete = (id, name) => {
    if (window.confirm(`ยืนยันการลบผู้ใช้ "${name}"? ข้อมูลเวกเตอร์ใบหน้าจะถูกลบออกจาก Server และตู้ Kiosk ทั้งหมดทันที`)) {
      deleteUserMutation.mutate(id);
    }
  };

  const openEnrollModal = (u) => {
    setEnrollUser(u);
    setImageFiles([]);
  };

  const handleFileChange = (e) => {
    const files = Array.from(e.target.files);
    if (files.length > 5) {
      alert('สามารถเลือกได้สูงสุดไม่เกิน 5 รูป ระบบจะเลือกเฉพาะ 5 รูปแรกให้ครับ');
      setImageFiles(files.slice(0, 5));
    } else {
      setImageFiles(files);
    }
  };

  const handleEnrollSubmit = (e) => {
    e.preventDefault();
    if (!enrollUser) return;
    if (imageFiles.length === 0) {
      alert('กรุณาเลือกรูปถ่ายอย่างน้อย 1 รูป (แนะนำ 3-5 รูป)');
      return;
    }
    uploadEnrollmentMutation.mutate();
  };

  // กรองรายชื่อพนักงานตามช่องค้นหา
  const filteredUsers = users?.filter((u) => {
    if (!searchUser) return true;
    const term = searchUser.toLowerCase();
    return (
      u.name?.toLowerCase().includes(term) ||
      u.employee_id?.toLowerCase().includes(term) ||
      u.department?.toLowerCase().includes(term)
    );
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">👤 จัดการผู้ใช้งานและใบหน้า (Users & Face Enrollment)</h1>
        <button onClick={openCreateModal} className="btn btn-primary">
          ➕ เพิ่มผู้ใช้งานใหม่
        </button>
      </div>

      {/* ช่องค้นหาพนักงาน */}
      <div style={{ marginBottom: '1.25rem', maxWidth: '400px' }}>
        <input
          type="text"
          className="input-field"
          placeholder="🔍 ค้นหาชื่อ, รหัสพนักงาน หรือฝ่าย..."
          value={searchUser}
          onChange={(e) => setSearchUser(e.target.value)}
        />
      </div>

      {isLoading && <p>กำลังโหลดรายชื่อผู้ใช้...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      {/* ตารางรายชื่อพนักงาน */}
      <div className="table-container" style={{ marginBottom: '2.5rem' }}>
        <table className="data-table">
          <thead>
            <tr>
              <th>รหัสพนักงาน</th>
              <th>คำนำหน้า</th>
              <th>ชื่อ-นามสกุล</th>
              <th>ฝ่าย</th>
              <th>ช่วงสัญญาปฏิบัติงาน</th>
              <th>สถานะผู้ใช้</th>
              <th>ข้อมูลใบหน้า</th>
              <th>จัดการ</th>
            </tr>
          </thead>
          <tbody>
            {filteredUsers?.map((u) => {
              // ดึงคำนำหน้า และชื่อ-นามสกุลแยกกันอย่างชัดเจน
              let title = u.title || '';
              let displayName = '';
              if (u.first_name) {
                displayName = `${u.first_name} ${u.last_name || ''}`.trim();
              } else {
                displayName = u.name;
                if (title && displayName.startsWith(title)) {
                  displayName = displayName.slice(title.length).trim();
                } else if (!title) {
                  const matched = TITLE_PREFIXES.find((p) => displayName.startsWith(p));
                  if (matched) {
                    title = matched;
                    displayName = displayName.slice(matched.length).trim();
                  }
                }
              }

              return (
                <tr key={u.id}>
                  <td>
                    <strong>{u.employee_id || '-'}</strong>
                  </td>
                  <td>
                    <span style={{ fontWeight: 600, color: 'var(--primary)' }}>
                      {title || '-'}
                    </span>
                  </td>
                  <td>{displayName || u.name}</td>
                  <td>{u.department || '-'}</td>
                  <td>
                    <div style={{ fontSize: '0.85rem' }}>
                      <div>🟢 เริ่ม: {formatThaiDate(u.start_date)}</div>
                      {u.end_date ? (
                        <div>🔴 สิ้นสุด: {formatThaiDate(u.end_date)}</div>
                      ) : (
                        <div style={{ color: 'var(--text-muted)' }}>พนักงานประจำ</div>
                      )}
                    </div>
                  </td>
                  <td>
                    <StatusBadge status={u.status} />
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
                      {u.has_face ? (
                        <>
                          <span style={{ color: 'var(--success)', fontWeight: 'bold', fontSize: '0.9rem' }}>
                            ✅ มีข้อมูลใบหน้า
                          </span>
                          <button
                            onClick={() => openEnrollModal(u)}
                            className="btn"
                            title="อัปเดตรูปถ่ายใบหน้าใหม่"
                            style={{
                              padding: '0.2rem 0.5rem',
                              fontSize: '0.75rem',
                              background: '#e0f2fe',
                              color: '#0369a1',
                              borderRadius: '4px',
                              border: '1px solid #bae6fd',
                            }}
                          >
                            🔄 อัปเดตใหม่
                          </button>
                        </>
                      ) : (
                        <>
                          <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                            ❌ ยังไม่ลงทะเบียน
                          </span>
                          <button
                            onClick={() => openEnrollModal(u)}
                            className="btn btn-primary"
                            style={{ padding: '0.25rem 0.6rem', fontSize: '0.8rem' }}
                          >
                            📸 ลงทะเบียนใบหน้า
                          </button>
                        </>
                      )}
                    </div>
                  </td>
                  <td>v{u.version}</td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.4rem' }}>
                      <button
                        onClick={() => openEditModal(u)}
                        className="btn"
                        style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem', background: 'var(--warning)', color: '#000' }}
                      >
                        ✏️ แก้ไข
                      </button>
                      <button
                        onClick={() => handleDelete(u.id, u.name)}
                        className="btn btn-danger"
                        style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem' }}
                      >
                        🗑️ ลบ
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
            {filteredUsers?.length === 0 && (
              <tr>
                <td colSpan="8" style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '1.5rem' }}>
                  ไม่พบข้อมูลผู้ใช้ในระบบ
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Modal 1: เพิ่ม / แก้ไขผู้ใช้งาน */}
      {showUserModal && (
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
          <div className="login-card" style={{ maxWidth: '520px', width: '90%' }}>
            <h3 style={{ marginBottom: '1rem' }}>
              {editUser ? `✏️ แก้ไขข้อมูลพนักงาน (${editUser.name})` : '➕ เพิ่มผู้ใช้งานใหม่'}
            </h3>
            <form onSubmit={handleUserSubmit}>
              <div className="form-group">
                <label>รหัสพนักงาน *</label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="เช่น SLRI-001 หรือ B001"
                  value={formData.employee_id}
                  onChange={(e) => setFormData({ ...formData, employee_id: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr 1fr', gap: '0.6rem' }}>
                <div className="form-group">
                  <label>คำนำหน้า *</label>
                  <select
                    className="input-field"
                    value={formData.title}
                    onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                  >
                    {TITLE_PREFIXES.map((t) => (
                      <option key={t} value={t}>
                        {t}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="form-group">
                  <label>ชื่อ *</label>
                  <input
                    type="text"
                    className="input-field"
                    placeholder="เช่น สมชาย"
                    value={formData.first_name}
                    onChange={(e) => setFormData({ ...formData, first_name: e.target.value })}
                    required
                  />
                </div>

                <div className="form-group">
                  <label>นามสกุล *</label>
                  <input
                    type="text"
                    className="input-field"
                    placeholder="เช่น ใจดี"
                    value={formData.last_name}
                    onChange={(e) => setFormData({ ...formData, last_name: e.target.value })}
                    required
                  />
                </div>
              </div>

              <div className="form-group">
                <label>สังกัดฝ่าย *</label>
                <select
                  className="input-field"
                  value={formData.department}
                  onChange={(e) => setFormData({ ...formData, department: e.target.value })}
                >
                  {DIVISIONS.map((d) => (
                    <option key={d} value={d}>
                      {d}
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.8rem' }}>
                <div className="form-group">
                  <label>📅 วันที่เริ่มงาน *</label>
                  <input
                    type="date"
                    className="input-field"
                    value={formData.start_date}
                    onChange={(e) => setFormData({ ...formData, start_date: e.target.value })}
                    required
                  />
                </div>

                <div className="form-group">
                  <label>📅 วันสิ้นสุดสัญญา (ถ้ามี)</label>
                  <input
                    type="date"
                    className="input-field"
                    value={formData.end_date}
                    onChange={(e) => setFormData({ ...formData, end_date: e.target.value })}
                  />
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                    * เว้นว่างได้สำหรับพนักงานประจำ
                  </span>
                </div>
              </div>

              {editUser && (
                <div className="form-group">
                  <label>สถานะการทำงาน</label>
                  <select
                    className="input-field"
                    value={formData.status}
                    onChange={(e) => setFormData({ ...formData, status: e.target.value })}
                  >
                    <option value="active">Active (ปกติ)</option>
                    <option value="inactive">Inactive (ระงับชั่วคราว)</option>
                  </select>
                </div>
              )}

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={() => {
                    setShowUserModal(false);
                    setEditUser(null);
                  }}
                  className="btn"
                  style={{ background: '#eee', color: '#333' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={createUserMutation.isPending || updateUserMutation.isPending}
                >
                  {createUserMutation.isPending || updateUserMutation.isPending ? 'กำลังบันทึก...' : 'บันทึก'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal 2: ลงทะเบียน / อัปเดตรูปถ่ายใบหน้า */}
      {enrollUser && (
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
          <div className="login-card" style={{ maxWidth: '500px', width: '90%' }}>
            <h3 style={{ marginBottom: '0.5rem' }}>
              📸 ลงทะเบียนใบหน้าพนักงาน
            </h3>
            <p style={{ fontSize: '0.95rem', color: 'var(--primary)', fontWeight: 'bold', marginBottom: '1rem' }}>
              พนักงาน: {enrollUser.employee_id ? `[${enrollUser.employee_id}] ` : ''}{enrollUser.name} ({enrollUser.department || 'ไม่ระบุฝ่าย'})
            </p>

            <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1.25rem', lineHeight: '1.4' }}>
              * กรุณาเลือกรูปถ่ายใบหน้า <strong>1 ถึง 5 รูป</strong> (แนะนำมุมหน้าตรงและเอียงเล็กน้อย) ภาพจะถูกส่งไปยังชิป AI บน Kiosk เพื่อแปลงเป็นเวกเตอร์ใบหน้าเฉลี่ย และ <strong>ลบไฟล์ภาพถ่ายทิ้งทันที</strong> ตามกฎหมาย PDPA
            </p>

            <form onSubmit={handleEnrollSubmit}>
              <div className="form-group">
                <label>เลือกภาพถ่ายใบหน้า (สูงสุด 5 รูป)</label>
                <input
                  type="file"
                  accept="image/*"
                  multiple
                  className="input-field"
                  onChange={handleFileChange}
                  required
                />
                {imageFiles.length > 0 && (
                  <p style={{ fontSize: '0.85rem', color: 'var(--primary)', marginTop: '0.4rem', fontWeight: 'bold' }}>
                    📷 เลือกแล้ว {imageFiles.length} รูป (สูงสุด 5 รูป)
                  </p>
                )}
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={() => {
                    setEnrollUser(null);
                    setImageFiles([]);
                  }}
                  className="btn"
                  style={{ background: '#eee', color: '#333' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={uploadEnrollmentMutation.isPending || imageFiles.length === 0}
                >
                  {uploadEnrollmentMutation.isPending ? 'กำลังอัปโหลด...' : `🚀 ส่งประมวลผล (${imageFiles.length} รูป)`}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
