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

  // State สำหรับรูปถ่ายใบหน้า (แนบตอนสร้าง หรือแก้ไขพนักงาน)
  const [imageFiles, setImageFiles] = useState([]);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // State ค้นหาพนักงาน
  const [searchUser, setSearchUser] = useState('');

  // 1. ดึงรายชื่อพนักงานทั้งหมด
  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
  });

  // Mutation ลบพนักงาน
  const deleteUserMutation = useMutation({
    mutationFn: (id) => api.del(`/api/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
    },
    onError: (err) => {
      alert('เกิดข้อผิดพลาดในการลบพนักงาน: ' + (err.message || err));
    },
  });

  const openCreateModal = () => {
    setEditUser(null);
    setFormData(INITIAL_FORM);
    setImageFiles([]);
    setShowUserModal(true);
  };

  const openEditModal = (u) => {
    setEditUser(u);
    setImageFiles([]);

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

  const handleFileChange = (e) => {
    const files = Array.from(e.target.files);
    if (files.length > 5) {
      alert('สามารถเลือกได้สูงสุดไม่เกิน 5 รูป ระบบจะเลือกเฉพาะ 5 รูปแรกให้ครับ');
      setImageFiles(files.slice(0, 5));
    } else {
      setImageFiles(files);
    }
  };

  const handleUserSubmit = async (e) => {
    e.preventDefault();
    if (!formData.first_name.trim()) {
      alert('กรุณากรอกชื่อพนักงาน');
      return;
    }

    const prefix = formData.title ? `${formData.title} ` : '';
    const fullName = `${prefix}${formData.first_name.trim()} ${formData.last_name.trim()}`.trim();
    const payload = {
      employee_id: formData.employee_id.trim() || null,
      title: formData.title,
      first_name: formData.first_name.trim(),
      last_name: formData.last_name.trim(),
      name: fullName,
      department: formData.department,
      status: formData.status,
      start_date: formData.start_date || null,
      end_date: formData.end_date || null,
    };

    setIsSubmitting(true);
    try {
      let targetUserId = editUser ? editUser.id : null;
      if (editUser) {
        await api.put(`/api/users/${editUser.id}`, {
          ...payload,
          version: editUser.version,
        });
      } else {
        const created = await api.post('/api/users', payload);
        targetUserId = created?.id;
      }

      // หากมีรูปภาพแนบมา ให้ส่งคำขอ Enrollment ไปคำนวณเวกเตอร์ใบหน้าบนตู้ Kiosk
      if (imageFiles.length > 0 && targetUserId) {
        const form = new FormData();
        form.append('user_id', targetUserId);
        for (const file of imageFiles) {
          form.append('images', file);
          form.append('image', file);
        }
        await api.post('/api/enrollment', form, true);
        alert(
          editUser
            ? `บันทึกข้อมูลและส่งภาพถ่าย (${imageFiles.length} รูป) ไปคำนวณเวกเตอร์ใบหน้าเฉลี่ยบนตู้ Kiosk เรียบร้อยแล้ว!`
            : `เพิ่มพนักงานและส่งภาพถ่าย (${imageFiles.length} รูป) ไปคำนวณเวกเตอร์ใบหน้าเฉลี่ยบนตู้ Kiosk เรียบร้อยแล้ว!`
        );
      } else {
        alert(editUser ? 'บันทึกการแก้ไขพนักงานเรียบร้อยแล้ว' : 'เพิ่มพนักงานใหม่เรียบร้อยแล้ว');
      }

      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
      setShowUserModal(false);
      setEditUser(null);
      setImageFiles([]);
      setFormData(INITIAL_FORM);
    } catch (err) {
      alert('เกิดข้อผิดพลาด: ' + (err.message || 'ไม่สามารถดำเนินการได้'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDelete = (id, name) => {
    if (window.confirm(`ยืนยันการลบผู้ใช้ "${name}"? ข้อมูลเวกเตอร์ใบหน้าจะถูกลบออกจาก Server และตู้ Kiosk ทั้งหมดทันที`)) {
      deleteUserMutation.mutate(id);
    }
  };

  // กรองรายชื่อพนักงานตามช่องค้นหา
  const filteredUsers = users?.filter((u) => {
    if (!searchUser) return true;
    const term = searchUser.toLowerCase();
    return (
      u.name?.toLowerCase().includes(term) ||
      u.first_name?.toLowerCase().includes(term) ||
      u.last_name?.toLowerCase().includes(term) ||
      u.employee_id?.toLowerCase().includes(term) ||
      u.department?.toLowerCase().includes(term)
    );
  });

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">ข้อมูลพนักงาน</h1>
        <button onClick={openCreateModal} className="btn btn-primary">
          เพิ่มพนักงาน
        </button>
      </div>

      {/* ช่องค้นหาพนักงาน */}
      <div style={{ marginBottom: '1.25rem', maxWidth: '400px' }}>
        <input
          type="text"
          className="input-field"
          placeholder="ค้นหาชื่อ, รหัสพนักงาน หรือฝ่าย..."
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
              <th>ชื่อ</th>
              <th>นามสกุล</th>
              <th>ฝ่าย</th>
              <th>วันเริ่มงาน</th>
              <th>วันสิ้นสุดสัญญา</th>
              <th>สถานะผู้ใช้</th>
              <th>สถานะใบหน้า</th>
              <th>จัดการ</th>
            </tr>
          </thead>
          <tbody>
            {filteredUsers?.map((u) => {
              // ดึงคำนำหน้า และชื่อ-นามสกุลแยกกันอย่างชัดเจน
              let title = u.title || '';
              let firstName = u.first_name || '';
              let lastName = u.last_name || '';

              if (!firstName) {
                let rest = u.name || '';
                if (title && rest.startsWith(title)) {
                  rest = rest.slice(title.length).trim();
                } else if (!title) {
                  const matched = TITLE_PREFIXES.find((p) => rest.startsWith(p));
                  if (matched) {
                    title = matched;
                    rest = rest.slice(matched.length).trim();
                  }
                }
                const parts = rest.split(/\s+/);
                firstName = parts[0] || '';
                lastName = parts.slice(1).join(' ') || '';
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
                  <td>{firstName || '-'}</td>
                  <td>{lastName || '-'}</td>
                  <td>{u.department || '-'}</td>
                  <td>{formatThaiDate(u.start_date)}</td>
                  <td>
                    {u.end_date ? (
                      formatThaiDate(u.end_date)
                    ) : (
                      <span style={{ color: 'var(--text-muted)' }}>พนักงานประจำ</span>
                    )}
                  </td>
                  <td>
                    <StatusBadge status={u.status} />
                  </td>
                  <td>
                    {u.has_embedding ? (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '0.35rem',
                          padding: '0.25rem 0.65rem',
                          borderRadius: '9999px',
                          fontSize: '0.8rem',
                          fontWeight: 600,
                          background: '#dcfce7',
                          color: '#15803d',
                          border: '1px solid #bbf7d0',
                        }}
                      >
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#22c55e' }}></span>
                        ลงทะเบียนแล้ว
                      </span>
                    ) : (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '0.35rem',
                          padding: '0.25rem 0.65rem',
                          borderRadius: '9999px',
                          fontSize: '0.8rem',
                          fontWeight: 500,
                          background: '#f1f5f9',
                          color: '#64748b',
                          border: '1px solid #e2e8f0',
                        }}
                      >
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#94a3b8' }}></span>
                        ยังไม่ลงทะเบียน
                      </span>
                    )}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.4rem' }}>
                      <button
                        onClick={() => openEditModal(u)}
                        className="btn"
                        style={{
                          padding: '0.3rem 0.65rem',
                          fontSize: '0.8rem',
                          background: '#f3f4f6',
                          color: '#374151',
                          border: '1px solid #d1d5db',
                          borderRadius: '6px',
                        }}
                      >
                        แก้ไข
                      </button>
                      <button
                        onClick={() => handleDelete(u.id, displayName || u.name)}
                        className="btn btn-danger"
                        style={{
                          padding: '0.3rem 0.65rem',
                          fontSize: '0.8rem',
                          borderRadius: '6px',
                        }}
                        disabled={deleteUserMutation.isPending}
                      >
                        ลบ
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })}
            {filteredUsers?.length === 0 && (
              <tr>
                <td colSpan="9" style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '1.5rem' }}>
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
              {editUser ? `แก้ไขข้อมูลพนักงาน: ${editUser.name}` : 'เพิ่มพนักงานใหม่'}
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
                  <label>วันที่เริ่มงาน *</label>
                  <input
                    type="date"
                    className="input-field"
                    value={formData.start_date}
                    onChange={(e) => setFormData({ ...formData, start_date: e.target.value })}
                    required
                  />
                </div>

                <div className="form-group">
                  <label>วันสิ้นสุดสัญญา (ถ้ามี)</label>
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
                    <option value="active">ปกติ</option>
                    <option value="inactive">ระงับชั่วคราว</option>
                  </select>
                </div>
              )}

              {/* ส่วนลงทะเบียน / อัปเดตภาพถ่ายใบหน้า */}
              <div style={{ marginTop: '1.25rem', paddingTop: '1.25rem', borderTop: '1px solid #e2e8f0' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
                  <label style={{ fontWeight: 600, fontSize: '0.9rem', margin: 0 }}>
                    ข้อมูลและภาพถ่ายใบหน้า
                  </label>
                  {editUser && (
                    <span style={{ fontSize: '0.8rem', fontWeight: 600, color: editUser.has_embedding ? '#16a34a' : '#64748b' }}>
                      {editUser.has_embedding ? '🟢 ลงทะเบียนแล้ว' : '⚪ ยังไม่ลงทะเบียน'}
                    </span>
                  )}
                </div>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.6rem', lineHeight: '1.4' }}>
                  {editUser?.has_embedding
                    ? 'หากต้องการเปลี่ยนรูปใบหน้า สามารถเลือกรูปถ่ายใหม่ 1–5 รูป (ภาพจะส่งไปคำนวณเวกเตอร์เฉลี่ยบน Kiosk และลบทิ้งทันทีตาม PDPA)'
                    : 'เลือกรูปถ่ายใบหน้า 1–5 รูป เพื่อลงทะเบียนเข้าสู่ระบบ (สามารถแนบตอนนี้หรืออัปเดตภายหลังได้)'}
                </p>
                <input
                  type="file"
                  accept="image/*"
                  multiple
                  className="input-field"
                  onChange={handleFileChange}
                />
                {imageFiles.length > 0 && (
                  <p style={{ fontSize: '0.85rem', color: 'var(--primary)', marginTop: '0.4rem', fontWeight: 'bold' }}>
                    ✓ เลือกรูปถ่ายแล้ว {imageFiles.length} รูป (จะส่งประมวลผลเมื่อกดบันทึก)
                  </p>
                )}
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={() => {
                    setShowUserModal(false);
                    setEditUser(null);
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
                  disabled={isSubmitting}
                >
                  {isSubmitting ? 'กำลังบันทึก...' : 'บันทึก'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
