import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDate, formatThaiTime } from '../utils/date';
import { DIVISIONS, TITLE_PREFIXES } from './UsersPage';

function toTimeInputValue(dateStr) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';
  const hours = String(d.getHours()).padStart(2, '0');
  const minutes = String(d.getMinutes()).padStart(2, '0');
  return `${hours}:${minutes}`;
}

export default function AttendancePage() {
  const queryClient = useQueryClient();
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10)); // เริ่มต้นด้วยวันนี้
  const [endDate, setEndDate] = useState('');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedDevice, setSelectedDevice] = useState('');
  const [statusFilter, setStatusFilter] = useState('ALL'); // 'ALL' | 'RECOGNIZED' | 'UNRECOGNIZED' | 'ABSENT'
  const [previewImage, setPreviewImage] = useState(null); // URL ภาพสำหรับ Modal ขยายรูป

  // Modals state
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [editingPair, setEditingPair] = useState(null);

  // Unrecognized Scan Resolve Modal
  const [isResolveModalOpen, setIsResolveModalOpen] = useState(false);
  const [resolvingScan, setResolvingScan] = useState(null);
  const [resolveTab, setResolveTab] = useState('existing'); // 'existing' | 'new'
  const [resolveForm, setResolveForm] = useState({
    user_id: '',
    direction: 'IN',
    work_date: new Date().toISOString().slice(0, 10),
    scan_time: '08:30',
    reason: 'ผูกชื่อจากภาพใบหน้าที่ไม่รู้จัก',
  });
  const [newUserForm, setNewUserForm] = useState({
    employee_id: '',
    title: 'นาย',
    first_name: '',
    last_name: '',
    department: DIVISIONS[0],
    start_date: new Date().toISOString().slice(0, 10),
    direction: 'IN',
    scan_time: '08:30',
    reason: 'สร้างพนักงานใหม่จากภาพสแกนใบหน้า',
  });

  // Form state for Manual Add
  const [addForm, setAddForm] = useState({
    user_id: '',
    work_date: new Date().toISOString().slice(0, 10),
    check_in_time: '08:30',
    check_out_time: '17:30',
    reason: '',
  });

  // Form state for Manual Edit
  const [editForm, setEditForm] = useState({
    check_in_time: '',
    check_out_time: '',
    reason: '',
  });

  const [formError, setFormError] = useState('');

  // 1. Fetch Attendance Pairs
  const effectiveEnd = endDate || startDate;
  const { data: pairs, isLoading: isPairsLoading, error: pairsError } = useQuery({
    queryKey: ['attendance_pairs', startDate, effectiveEnd, selectedDevice],
    queryFn: () => {
      const params = new URLSearchParams();
      if (startDate) params.append('start', startDate);
      if (effectiveEnd) params.append('end', effectiveEnd);
      if (selectedDevice) params.append('device_id', selectedDevice);
      params.append('include_absent', 'true');
      const query = params.toString() ? `?${params.toString()}` : '';
      return api.get(`/api/attendance/pairs${query}`);
    },
  });

  // 2. Fetch Unrecognized Scans
  const { data: unrecognizedScans, isLoading: isUnrecLoading } = useQuery({
    queryKey: ['unrecognized_scans'],
    queryFn: () => api.get('/api/attendance/unrecognized?resolved=false'),
  });

  // 3. Fetch devices for dropdown filter
  const { data: devices } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices'),
  });

  // 4. Fetch users for dropdown in Manual Add & Resolve Modal
  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
    enabled: isAddModalOpen || isResolveModalOpen,
  });

  // Mutation: Manual Add Attendance
  const addMutation = useMutation({
    mutationFn: (data) => api.post('/api/attendance/manual', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
      setIsAddModalOpen(false);
      setAddForm({
        user_id: '',
        work_date: new Date().toISOString().slice(0, 10),
        check_in_time: '08:30',
        check_out_time: '17:30',
        reason: '',
      });
      setFormError('');
    },
    onError: (err) => {
      setFormError(err.message || 'ไม่สามารถบันทึกเวลาแทนได้');
    },
  });

  // Mutation: Manual Edit Attendance
  const editMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/api/attendance/pairs/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
      setIsEditModalOpen(false);
      setEditingPair(null);
      setFormError('');
    },
    onError: (err) => {
      setFormError(err.message || 'ไม่สามารถแก้ไขเวลาได้');
    },
  });

  // Mutation: Resolve Unrecognized Scan
  const resolveMutation = useMutation({
    mutationFn: ({ id, data }) => api.post(`/api/attendance/unrecognized/${id}/resolve`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
      queryClient.invalidateQueries({ queryKey: ['unrecognized_scans'] });
      setIsResolveModalOpen(false);
      setResolvingScan(null);
      setFormError('');
    },
    onError: (err) => {
      setFormError(err.message || 'ไม่สามารถผูกชื่อพนักงานได้');
    },
  });

  // Mutation: Create User and Resolve Unrecognized Scan
  const createUserAndResolveMutation = useMutation({
    mutationFn: ({ id, data }) => api.post(`/api/attendance/unrecognized/${id}/create-user`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
      queryClient.invalidateQueries({ queryKey: ['unrecognized_scans'] });
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setIsResolveModalOpen(false);
      setResolvingScan(null);
      setFormError('');
    },
    onError: (err) => {
      setFormError(err.message || 'ไม่สามารถสร้างพนักงานใหม่ได้');
    },
  });

  // Mutation: Delete Unrecognized Scan
  const deleteUnrecMutation = useMutation({
    mutationFn: (id) => api.del(`/api/attendance/unrecognized/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['unrecognized_scans'] });
    },
    onError: (err) => {
      alert('ไม่สามารถลบรายการได้: ' + (err.message || err));
    },
  });

  // Filter pairs
  const recognizedPairs = (pairs || []).filter((p) => p.status !== 'absent');
  const absentPairs = (pairs || []).filter((p) => p.status === 'absent');
  const unrecList = unrecognizedScans || [];

  // Search & date filtering helper for unrecognized scans
  const filteredUnrec = unrecList.filter((ur) => {
    // กรองตามช่วงวันที่เลือก
    if (ur.created_at) {
      const scanDate = ur.created_at.slice(0, 10);
      if (startDate && scanDate < startDate) return false;
      if (effectiveEnd && scanDate > effectiveEnd) return false;
    }
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    return 'ไม่พบใบหน้า'.includes(term) || 'ไม่รู้จัก'.includes(term) || 'unrecognized'.includes(term);
  });

  // Summary Counts (คำนวณตามวันที่เลือก)
  const counts = {
    total: recognizedPairs.length + filteredUnrec.length + absentPairs.length,
    recognized: recognizedPairs.length,
    unrecognized: filteredUnrec.length,
    absent: absentPairs.length,
  };

  // Search filtering helper
  const matchesSearch = (text) => {
    if (!searchTerm) return true;
    return (text || '').toLowerCase().includes(searchTerm.toLowerCase());
  };

  const filteredRecognized = recognizedPairs.filter(
    (p) => matchesSearch(p.name) || matchesSearch(p.employee_id)
  );

  const filteredAbsent = absentPairs.filter(
    (p) => matchesSearch(p.name) || matchesSearch(p.employee_id)
  );

  // Handlers
  const handleOpenEdit = (pair) => {
    setEditingPair(pair);
    setEditForm({
      check_in_time: toTimeInputValue(pair.check_in_time),
      check_out_time: toTimeInputValue(pair.check_out_time),
      reason: pair.adjustment_reason || '',
    });
    setFormError('');
    setIsEditModalOpen(true);
  };

  const handleQuickAddForAbsent = (pair) => {
    setAddForm({
      user_id: pair.user_id,
      work_date: pair.work_date,
      check_in_time: '08:30',
      check_out_time: '17:30',
      reason: 'บันทึกเวลาแทนเนื่องจากขาดการสแกน',
    });
    setFormError('');
    setIsAddModalOpen(true);
  };

  const handleOpenResolve = (unrec) => {
    setResolvingScan(unrec);
    setResolveTab('existing');
    const dateStr = unrec.created_at ? unrec.created_at.slice(0, 10) : new Date().toISOString().slice(0, 10);
    const timeStr = toTimeInputValue(unrec.created_at) || '08:30';
    setResolveForm({
      user_id: '',
      direction: 'IN',
      work_date: dateStr,
      scan_time: timeStr,
      reason: 'ผูกชื่อจากภาพใบหน้าที่ไม่รู้จัก',
    });
    setNewUserForm({
      employee_id: '',
      title: 'นาย',
      first_name: '',
      last_name: '',
      department: DIVISIONS[0],
      start_date: dateStr,
      direction: 'IN',
      scan_time: timeStr,
      reason: 'สร้างพนักงานใหม่จากภาพสแกนใบหน้า',
    });
    setFormError('');
    setIsResolveModalOpen(true);
  };

  const handleExportCSV = () => {
    if (!pairs || pairs.length === 0) return;
    const headers = ['วันที่', 'รหัสพนักงาน', 'ชื่อ-นามสกุล', 'เวลาเข้างาน', 'เวลาออกงาน', 'สถานะ'];
    const rows = pairs.map((p) => {
      let statusLabel = 'ตรงเวลา';
      if (p.status === 'absent') statusLabel = 'ยังไม่สแกน';
      else if (p.is_manual || p.status === 'adjusted') statusLabel = 'แก้ไขโดยผู้ดูแล';
      else if (p.status === 'late') statusLabel = 'มาสาย';
      else if (p.status === 'incomplete') statusLabel = 'ไม่ครบเวลา';
      else if (p.status === 'open') statusLabel = 'กำลังทำงาน';

      return [
        p.work_date,
        p.employee_id || '-',
        p.name,
        formatThaiTime(p.check_in_time),
        formatThaiTime(p.check_out_time),
        statusLabel,
      ];
    });

    const csvContent =
      'data:text/csv;charset=utf-8,\uFEFF' +
      [headers.join(','), ...rows.map((r) => r.map((f) => `"${f}"`).join(','))].join('\n');
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', `attendance_report_${new Date().toISOString().slice(0, 10)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const renderStatusBadge = (p) => {
    if (p.is_manual || p.status === 'adjusted') {
      return (
        <div>
          <span
            style={{
              padding: '0.25rem 0.6rem',
              borderRadius: '20px',
              fontSize: '0.8rem',
              background: '#f3e5f5',
              color: '#7b1fa2',
              fontWeight: 600,
              display: 'inline-block',
            }}
          >
            แก้ไขโดยผู้ดูแล
          </span>
          {p.adjustment_reason && (
            <div style={{ fontSize: '0.73rem', color: '#666', marginTop: '0.2rem', fontStyle: 'italic' }}>
              เหตุผล: {p.adjustment_reason}
            </div>
          )}
        </div>
      );
    }
    if (p.status === 'late') {
      return (
        <span
          style={{
            padding: '0.25rem 0.6rem',
            borderRadius: '20px',
            fontSize: '0.8rem',
            background: '#fff3e0',
            color: '#e65100',
            fontWeight: 600,
          }}
        >
          มาสาย
        </span>
      );
    }
    if (p.status === 'incomplete') {
      return (
        <span
          style={{
            padding: '0.25rem 0.6rem',
            borderRadius: '20px',
            fontSize: '0.8rem',
            background: '#ffebee',
            color: '#c62828',
            fontWeight: 600,
          }}
        >
          ไม่ครบเวลา
        </span>
      );
    }
    if (p.status === 'open') {
      return (
        <span
          style={{
            padding: '0.25rem 0.6rem',
            borderRadius: '20px',
            fontSize: '0.8rem',
            background: '#e3f2fd',
            color: '#1565c0',
            fontWeight: 600,
          }}
        >
          กำลังทำงาน
        </span>
      );
    }
    return (
      <span
        style={{
          padding: '0.25rem 0.6rem',
          borderRadius: '20px',
          fontSize: '0.8rem',
          background: '#e8f5e9',
          color: '#2e7d32',
          fontWeight: 600,
        }}
      >
        ตรงเวลา
      </span>
    );
  };

  return (
    <div>
      {/* Header */}
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '0.75rem' }}>
        <div>
          <h1 className="page-title">การสแกนวันนี้</h1>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.88rem', margin: '0.2rem 0 0' }}>
            ติดตามการสแกนหน้าแบบเรียลไทม์ พร้อมตรวจสอบภาพสแกนจริงและจัดการผู้ที่ไม่พบใบหน้า
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <button
            onClick={() => {
              setAddForm({
                user_id: '',
                work_date: startDate || new Date().toISOString().slice(0, 10),
                check_in_time: '08:30',
                check_out_time: '17:30',
                reason: '',
              });
              setFormError('');
              setIsAddModalOpen(true);
            }}
            className="btn btn-primary"
            style={{ background: '#2563eb', color: '#fff' }}
          >
            บันทึกเวลาแทน
          </button>
          <button onClick={handleExportCSV} className="btn" style={{ background: '#fff', border: '1px solid #d1d5db' }}>
            ส่งออก CSV
          </button>
        </div>
      </div>

      {/* 4 Summary / Filter Tab Buttons */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
          gap: '0.75rem',
          marginBottom: '1.25rem',
        }}
      >
        {/* ทั้งหมด */}
        <div
          onClick={() => setStatusFilter('ALL')}
          style={{
            background: '#fff',
            padding: '0.85rem 1.1rem',
            borderRadius: '10px',
            border: statusFilter === 'ALL' ? '2.5px solid #2563eb' : '1px solid #e5e7eb',
            cursor: 'pointer',
            boxShadow: statusFilter === 'ALL' ? '0 2px 8px rgba(37,99,235,0.15)' : 'none',
            transition: 'all 0.15s ease',
          }}
        >
          <div style={{ fontSize: '0.82rem', color: '#6b7280', fontWeight: 600 }}>ทั้งหมด</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#111827', marginTop: '0.15rem' }}>
            {counts.total}
          </div>
        </div>

        {/* สแกนผ่าน */}
        <div
          onClick={() => setStatusFilter('RECOGNIZED')}
          style={{
            background: '#fff',
            padding: '0.85rem 1.1rem',
            borderRadius: '10px',
            border: statusFilter === 'RECOGNIZED' ? '2.5px solid #16a34a' : '1px solid #e5e7eb',
            cursor: 'pointer',
            boxShadow: statusFilter === 'RECOGNIZED' ? '0 2px 8px rgba(22,163,74,0.15)' : 'none',
            transition: 'all 0.15s ease',
          }}
        >
          <div style={{ fontSize: '0.82rem', color: '#16a34a', fontWeight: 600 }}>สแกนผ่าน</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#16a34a', marginTop: '0.15rem' }}>
            {counts.recognized}
          </div>
        </div>

        {/* ไม่พบใบหน้า */}
        <div
          onClick={() => setStatusFilter('UNRECOGNIZED')}
          style={{
            background: counts.unrecognized > 0 ? '#fffbeb' : '#fff',
            padding: '0.85rem 1.1rem',
            borderRadius: '10px',
            border: statusFilter === 'UNRECOGNIZED' ? '2.5px solid #d97706' : counts.unrecognized > 0 ? '1.5px solid #fcd34d' : '1px solid #e5e7eb',
            cursor: 'pointer',
            boxShadow: statusFilter === 'UNRECOGNIZED' ? '0 2px 8px rgba(217,119,6,0.18)' : 'none',
            transition: 'all 0.15s ease',
          }}
        >
          <div style={{ fontSize: '0.82rem', color: '#b45309', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
            <span>ไม่พบใบหน้า</span>
            {counts.unrecognized > 0 && (
              <span style={{ fontSize: '0.7rem', background: '#f59e0b', color: '#fff', padding: '1px 6px', borderRadius: '10px' }}>
                รอตรวจสอบ
              </span>
            )}
          </div>
          <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#b45309', marginTop: '0.15rem' }}>
            {counts.unrecognized}
          </div>
        </div>

        {/* ยังไม่สแกน */}
        <div
          onClick={() => setStatusFilter('ABSENT')}
          style={{
            background: '#fff',
            padding: '0.85rem 1.1rem',
            borderRadius: '10px',
            border: statusFilter === 'ABSENT' ? '2.5px solid #dc2626' : '1px solid #e5e7eb',
            cursor: 'pointer',
            boxShadow: statusFilter === 'ABSENT' ? '0 2px 8px rgba(220,38,38,0.15)' : 'none',
            transition: 'all 0.15s ease',
          }}
        >
          <div style={{ fontSize: '0.82rem', color: '#dc2626', fontWeight: 600 }}>ยังไม่สแกน</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#dc2626', marginTop: '0.15rem' }}>
            {counts.absent}
          </div>
        </div>
      </div>

      {/* Date & Search Filters */}
      <div
        style={{
          display: 'flex',
          gap: '1rem',
          flexWrap: 'wrap',
          alignItems: 'flex-end',
          marginBottom: '1.25rem',
          background: '#fff',
          padding: '1rem 1.25rem',
          borderRadius: '10px',
          border: '1px solid #e5e7eb',
        }}
      >
        <div>
          <label style={{ fontSize: '0.82rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500, color: '#4b5563' }}>
            ตั้งแต่วันที่
          </label>
          <input
            type="date"
            className="input-field"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
          />
        </div>
        <div>
          <label style={{ fontSize: '0.82rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500, color: '#4b5563' }}>
            ถึงวันที่
          </label>
          <input
            type="date"
            className="input-field"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
          />
        </div>

        <div style={{ flex: '1', minWidth: '220px' }}>
          <label style={{ fontSize: '0.82rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500, color: '#4b5563' }}>
            ค้นหาพนักงาน
          </label>
          <input
            type="text"
            className="input-field"
            placeholder="พิมพ์ชื่อ หรือ รหัสพนักงาน..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        {(startDate || endDate || searchTerm || statusFilter !== 'ALL') && (
          <div>
            <button
              onClick={() => {
                setStartDate(new Date().toISOString().slice(0, 10));
                setEndDate('');
                setSearchTerm('');
                setStatusFilter('ALL');
              }}
              className="btn"
              style={{ background: '#f3f4f6', color: '#4b5563', padding: '0.55rem 0.9rem' }}
            >
              ล้างตัวกรอง
            </button>
          </div>
        )}
      </div>

      {/* Loading & Error */}
      {(isPairsLoading || isUnrecLoading) && (
        <div style={{ padding: '2rem', textAlign: 'center', color: '#6b7280' }}>กำลังโหลดข้อมูลการสแกน...</div>
      )}
      {pairsError && (
        <div style={{ padding: '1rem', background: '#fee2e2', color: '#b91c1c', borderRadius: '8px', marginBottom: '1rem' }}>
          เกิดข้อผิดพลาด: {pairsError.message}
        </div>
      )}

      {/* Main Table */}
      <div className="table-container" style={{ background: '#fff', borderRadius: '10px', overflow: 'hidden', border: '1px solid #e5e7eb' }}>
        <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ background: '#f8fafc', borderBottom: '1.5px solid #e2e8f0', textAlign: 'left' }}>
              <th style={{ padding: '0.85rem 1rem', width: '80px' }}>รูปสแกน</th>
              <th style={{ padding: '0.85rem 1rem', width: '120px' }}>รหัสพนักงาน</th>
              <th style={{ padding: '0.85rem 1rem', width: '180px' }}>ชื่อ-นามสกุล</th>
              <th style={{ padding: '0.85rem 1rem', width: '100px' }}>เวลาเข้า</th>
              <th style={{ padding: '0.85rem 1rem', width: '100px' }}>เวลาออก</th>
              <th style={{ padding: '0.85rem 1rem', width: '130px' }}>สถานะ</th>
              <th style={{ padding: '0.85rem 1rem', width: '150px' }}>หมายเหตุ</th>
              <th style={{ padding: '0.85rem 1rem', width: '110px', textAlign: 'center' }}>จัดการ</th>
            </tr>
          </thead>
          <tbody>
            {/* 1. Unrecognized Scans (Shown when ALL or UNRECOGNIZED) */}
            {(statusFilter === 'ALL' || statusFilter === 'UNRECOGNIZED') &&
              filteredUnrec.map((ur) => (
                <tr
                  key={`unrec-${ur.id}`}
                  style={{
                    backgroundColor: '#fffbeb',
                    borderBottom: '1px solid #fef3c7',
                  }}
                >
                  <td style={{ padding: '0.75rem 1rem' }}>
                    {ur.snapshot_url ? (
                      <img
                        src={ur.snapshot_url}
                        alt="ภาพไม่พบใบหน้า"
                        onClick={() => setPreviewImage(ur.snapshot_url)}
                        title="คลิกเพื่อดูรูปภาพขนาดใหญ่"
                        style={{
                          width: '56px',
                          height: '56px',
                          borderRadius: '8px',
                          objectFit: 'cover',
                          border: '1.5px solid #f59e0b',
                          background: '#fef3c7',
                          display: 'block',
                          cursor: 'pointer',
                        }}
                      />
                    ) : (
                      <div
                        style={{
                          width: '56px',
                          height: '56px',
                          borderRadius: '8px',
                          border: '1.5px dashed #f59e0b',
                          background: '#fffbeb',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          fontSize: '0.72rem',
                          color: '#b45309',
                        }}
                      >
                        ไม่มีรูป
                      </div>
                    )}
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ color: '#9ca3af' }}>-</span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <div style={{ color: '#b45309', fontWeight: 600 }}>
                      ไม่พบข้อมูลในระบบ
                    </div>
                    <div style={{ fontSize: '0.75rem', color: '#78716c' }}>
                      ใบหน้ายังไม่เคยลงทะเบียน
                    </div>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <div style={{ fontWeight: 600, color: '#b45309' }}>
                      {formatThaiTime(ur.created_at)} น.
                    </div>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ color: '#94a3b8' }}>-</span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span
                      style={{
                        padding: '0.25rem 0.6rem',
                        borderRadius: '20px',
                        fontSize: '0.8rem',
                        background: '#fef3c7',
                        color: '#b45309',
                        fontWeight: 700,
                        border: '1px solid #fde68a',
                        display: 'inline-block',
                      }}
                    >
                      ไม่พบใบหน้า
                    </span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ fontSize: '0.8rem', color: '#d97706', fontWeight: 600 }}>
                      บุคคลภายนอก/ยังไม่ลงทะเบียน
                    </span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem', textAlign: 'center' }}>
                    <div style={{ display: 'flex', gap: '0.4rem', justifyContent: 'center' }}>
                      <button
                        onClick={() => handleOpenResolve(ur)}
                        className="btn"
                        style={{
                          padding: '0.35rem 0.65rem',
                          fontSize: '0.8rem',
                          borderRadius: '6px',
                          background: '#2563eb',
                          color: '#fff',
                          fontWeight: 600,
                          border: 'none',
                          cursor: 'pointer',
                        }}
                      >
                        + ผูกชื่อ
                      </button>
                      <button
                        onClick={() => {
                          if (window.confirm('ต้องการลบรายการภาพใบหน้านี้หรือไม่?')) {
                            deleteUnrecMutation.mutate(ur.id);
                          }
                        }}
                        className="btn"
                        style={{
                          padding: '0.35rem 0.55rem',
                          fontSize: '0.8rem',
                          borderRadius: '6px',
                          background: '#fee2e2',
                          color: '#b91c1c',
                          border: '1px solid #fca5a5',
                          cursor: 'pointer',
                        }}
                      >
                        ลบ
                      </button>
                    </div>
                  </td>
                </tr>
              ))}

            {/* 2. Recognized Pairs (Shown when ALL or RECOGNIZED) */}
            {(statusFilter === 'ALL' || statusFilter === 'RECOGNIZED') &&
              filteredRecognized.map((p) => {
                const snapshotUrl = p.check_in_snapshot || p.check_out_snapshot;
                return (
                  <tr
                    key={`pair-${p.id}`}
                    style={{
                      borderBottom: '1px solid #f1f5f9',
                    }}
                  >
                    <td style={{ padding: '0.75rem 1rem' }}>
                      {snapshotUrl ? (
                        <img
                          src={snapshotUrl}
                          alt="สแกน"
                          onClick={() => setPreviewImage(snapshotUrl)}
                          title="คลิกเพื่อดูรูปภาพขนาดใหญ่"
                          style={{
                            width: '56px',
                            height: '56px',
                            borderRadius: '8px',
                            objectFit: 'cover',
                            border: '1px solid #e2e8f0',
                            background: '#f8fafc',
                            display: 'block',
                            cursor: 'pointer',
                          }}
                        />
                      ) : (
                        <div
                          style={{
                            width: '56px',
                            height: '56px',
                            borderRadius: '8px',
                            border: '1px dashed #cbd5e1',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            fontSize: '0.72rem',
                            color: '#94a3b8',
                            background: '#f8fafc',
                          }}
                        >
                          ไม่มีรูป
                        </div>
                      )}
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      <span style={{ fontFamily: 'monospace', fontWeight: 600, fontSize: '0.92rem', color: '#1e293b' }}>
                        {p.employee_id || '-'}
                      </span>
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      <div style={{ fontWeight: 600, color: '#0f172a' }}>{p.name}</div>
                      {p.department && (
                        <div style={{ fontSize: '0.75rem', color: '#64748b' }}>{p.department}</div>
                      )}
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      {p.check_in_time ? (
                        <span style={{ fontWeight: 600, color: '#16a34a' }}>
                          {formatThaiTime(p.check_in_time)} น.
                        </span>
                      ) : (
                        <span style={{ color: '#94a3b8' }}>-</span>
                      )}
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      {p.check_out_time ? (
                        <span style={{ fontWeight: 600, color: '#2563eb' }}>
                          {formatThaiTime(p.check_out_time)} น.
                        </span>
                      ) : (
                        <span style={{ color: '#94a3b8' }}>-</span>
                      )}
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      {renderStatusBadge(p)}
                    </td>
                    <td style={{ padding: '0.75rem 1rem' }}>
                      {p.adjustment_reason ? (
                        <span
                          style={{
                            fontSize: '0.8rem',
                            color: p.adjustment_reason.includes('Anti-Spoof') || p.adjustment_reason.includes('หมดอายุ') ? '#dc2626' : '#475569',
                            fontWeight: p.adjustment_reason.includes('Anti-Spoof') || p.adjustment_reason.includes('หมดอายุ') ? 600 : 400,
                            background: p.adjustment_reason.includes('Anti-Spoof') || p.adjustment_reason.includes('หมดอายุ') ? '#fef2f2' : 'transparent',
                            padding: p.adjustment_reason.includes('Anti-Spoof') || p.adjustment_reason.includes('หมดอายุ') ? '2px 8px' : 0,
                            borderRadius: '4px',
                            display: 'inline-block',
                          }}
                        >
                          {p.adjustment_reason}
                        </span>
                      ) : (
                        <span style={{ color: '#94a3b8' }}>-</span>
                      )}
                    </td>
                    <td style={{ padding: '0.75rem 1rem', textAlign: 'center' }}>
                      <button
                        onClick={() => handleOpenEdit(p)}
                        className="btn"
                        style={{
                          padding: '0.35rem 0.75rem',
                          fontSize: '0.8rem',
                          borderRadius: '6px',
                          background: '#f8fafc',
                          border: '1px solid #cbd5e1',
                          color: '#334155',
                          cursor: 'pointer',
                        }}
                      >
                        แก้ไข
                      </button>
                    </td>
                  </tr>
                );
              })}

            {/* 3. Absent Records (Shown when ALL or ABSENT) */}
            {(statusFilter === 'ALL' || statusFilter === 'ABSENT') &&
              filteredAbsent.map((p) => (
                <tr
                  key={`absent-${p.user_id}-${p.work_date}`}
                  style={{
                    backgroundColor: '#fff5f5',
                    borderBottom: '1px solid #fee2e2',
                  }}
                >
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <div
                      style={{
                        width: '56px',
                        height: '56px',
                        borderRadius: '8px',
                        border: '1.5px dashed #fca5a5',
                        background: '#fff1f2',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '0.72rem',
                        color: '#b91c1c',
                        fontWeight: 500,
                        textAlign: 'center',
                        padding: '2px',
                      }}
                    >
                      ยังไม่สแกน
                    </div>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ fontFamily: 'monospace', fontWeight: 600, fontSize: '0.92rem', color: '#1e293b' }}>
                      {p.employee_id || '-'}
                    </span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <div style={{ fontWeight: 600, color: '#0f172a' }}>{p.name}</div>
                    {p.department && (
                      <div style={{ fontSize: '0.75rem', color: '#64748b' }}>{p.department}</div>
                    )}
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ color: '#94a3b8' }}>-</span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ color: '#94a3b8' }}>-</span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span
                      style={{
                        padding: '0.25rem 0.6rem',
                        borderRadius: '20px',
                        fontSize: '0.8rem',
                        background: '#fee2e2',
                        color: '#b91c1c',
                        fontWeight: 700,
                        border: '1px solid #f87171',
                        display: 'inline-block',
                      }}
                    >
                      ยังไม่สแกน
                    </span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ color: '#94a3b8' }}>-</span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem', textAlign: 'center' }}>
                    <button
                      onClick={() => handleQuickAddForAbsent(p)}
                      className="btn"
                      style={{
                        padding: '0.35rem 0.65rem',
                        fontSize: '0.8rem',
                        borderRadius: '6px',
                        background: '#fff',
                        border: '1px solid #b91c1c',
                        color: '#b91c1c',
                        fontWeight: 600,
                        cursor: 'pointer',
                      }}
                    >
                      ลงเวลาแทน
                    </button>
                  </td>
                </tr>
              ))}

            {/* Empty State */}
            {((statusFilter === 'ALL' && counts.total === 0) ||
              (statusFilter === 'RECOGNIZED' && filteredRecognized.length === 0) ||
              (statusFilter === 'UNRECOGNIZED' && filteredUnrec.length === 0) ||
              (statusFilter === 'ABSENT' && filteredAbsent.length === 0)) && (
              <tr>
                <td colSpan="6" style={{ textAlign: 'center', color: '#94a3b8', padding: '3rem' }}>
                  ไม่พบข้อมูลการสแกนในหมวดนี้
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Modal: จัดการภาพใบหน้าที่ไม่รู้จัก (Resolve or Create User) */}
      {isResolveModalOpen && resolvingScan && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: '1rem',
          }}
        >
          <div
            style={{
              background: '#fff',
              borderRadius: '12px',
              padding: '1.75rem',
              width: '100%',
              maxWidth: '540px',
              maxHeight: '90vh',
              overflowY: 'auto',
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.15)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '0.5rem' }}>
              <div>
                <h2 style={{ margin: 0, fontSize: '1.25rem', color: '#0f172a' }}>
                  จัดการภาพใบหน้าที่ไม่รู้จัก
                </h2>
                <p style={{ fontSize: '0.85rem', color: '#64748b', margin: '0.25rem 0 0' }}>
                  ผูกกับพนักงานที่มีอยู่แล้ว หรือลงทะเบียนเป็นพนักงานใหม่ทันที
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsResolveModalOpen(false)}
                style={{
                  background: 'transparent',
                  border: 'none',
                  fontSize: '1.25rem',
                  color: '#94a3b8',
                  cursor: 'pointer',
                  padding: '0.25rem',
                  lineHeight: 1,
                }}
              >
                ✕
              </button>
            </div>

            {formError && (
              <div
                style={{
                  background: '#fee2e2',
                  color: '#b91c1c',
                  padding: '0.6rem 0.8rem',
                  borderRadius: '6px',
                  margin: '0.75rem 0',
                  fontSize: '0.85rem',
                }}
              >
                {formError}
              </div>
            )}

            {/* Face Snapshot Preview */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', margin: '1rem 0', padding: '0.75rem', background: '#f8fafc', borderRadius: '8px', border: '1px solid #e2e8f0' }}>
              {resolvingScan.snapshot_url && (
                <img
                  src={resolvingScan.snapshot_url}
                  alt="ใบหน้าที่ต้องการผูก"
                  style={{
                    width: '68px',
                    height: '68px',
                    borderRadius: '8px',
                    objectFit: 'cover',
                    border: '2px solid #3b82f6',
                  }}
                />
              )}
              <div style={{ flex: 1, fontSize: '0.85rem' }}>
                <div style={{ color: '#64748b', fontSize: '0.8rem' }}>เวลาที่กล้องตรวจจับ:</div>
                <div style={{ fontSize: '1.05rem', fontWeight: 700, color: '#0f172a' }}>
                  {formatThaiTime(resolvingScan.created_at)} น.
                </div>
                <div style={{ color: '#64748b', fontSize: '0.8rem', marginTop: '2px' }}>
                  วันที่: {formatThaiDate(resolvingScan.created_at)} | เครื่อง: {resolvingScan.device_id || '-'}
                </div>
              </div>
            </div>

            {/* Segmented Tab Control */}
            <div
              style={{
                display: 'flex',
                background: '#f1f5f9',
                padding: '4px',
                borderRadius: '8px',
                marginBottom: '1.25rem',
                gap: '4px',
              }}
            >
              <button
                type="button"
                onClick={() => { setResolveTab('existing'); setFormError(''); }}
                style={{
                  flex: 1,
                  padding: '0.5rem 0.75rem',
                  borderRadius: '6px',
                  border: 'none',
                  fontSize: '0.85rem',
                  fontWeight: resolveTab === 'existing' ? 600 : 500,
                  background: resolveTab === 'existing' ? '#ffffff' : 'transparent',
                  color: resolveTab === 'existing' ? '#1e293b' : '#64748b',
                  boxShadow: resolveTab === 'existing' ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease',
                }}
              >
                ผูกกับพนักงานเดิม
              </button>
              <button
                type="button"
                onClick={() => { setResolveTab('new'); setFormError(''); }}
                style={{
                  flex: 1,
                  padding: '0.5rem 0.75rem',
                  borderRadius: '6px',
                  border: 'none',
                  fontSize: '0.85rem',
                  fontWeight: resolveTab === 'new' ? 600 : 500,
                  background: resolveTab === 'new' ? '#ffffff' : 'transparent',
                  color: resolveTab === 'new' ? '#1e293b' : '#64748b',
                  boxShadow: resolveTab === 'new' ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease',
                }}
              >
                สร้างเป็นพนักงานใหม่ทันที
              </button>
            </div>

            {/* Tab 1: ผูกกับพนักงานเดิม */}
            {resolveTab === 'existing' && (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  if (!resolveForm.user_id) {
                    setFormError('กรุณาเลือกพนักงานที่ต้องการผูก');
                    return;
                  }
                  resolveMutation.mutate({
                    id: resolvingScan.id,
                    data: {
                      ...resolveForm,
                      action: 'assign',
                    },
                  });
                }}
              >
                <div style={{ marginBottom: '1rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                    เลือกพนักงาน <span style={{ color: 'red' }}>*</span>
                  </label>
                  <select
                    className="input-field"
                    value={resolveForm.user_id}
                    onChange={(e) => setResolveForm({ ...resolveForm, user_id: e.target.value })}
                    required
                  >
                    <option value="">-- ค้นหา / เลือกพนักงาน --</option>
                    {users?.map((u) => (
                      <option key={u.id} value={u.id}>
                        {u.employee_id ? `[${u.employee_id}] ` : ''}
                        {u.name} ({u.department || 'ไม่ระบุแผนก'})
                      </option>
                    ))}
                  </select>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      ประเภทการสแกน <span style={{ color: 'red' }}>*</span>
                    </label>
                    <select
                      className="input-field"
                      value={resolveForm.direction}
                      onChange={(e) => setResolveForm({ ...resolveForm, direction: e.target.value })}
                    >
                      <option value="IN">เข้างาน (IN)</option>
                      <option value="OUT">ออกงาน (OUT)</option>
                    </select>
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      เวลาสแกน
                    </label>
                    <input
                      type="time"
                      className="input-field"
                      value={resolveForm.scan_time}
                      onChange={(e) => setResolveForm({ ...resolveForm, scan_time: e.target.value })}
                      required
                    />
                  </div>
                </div>

                <div style={{ marginBottom: '1.25rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                    หมายเหตุ (บันทึกใน Audit Log)
                  </label>
                  <input
                    type="text"
                    className="input-field"
                    value={resolveForm.reason}
                    onChange={(e) => setResolveForm({ ...resolveForm, reason: e.target.value })}
                  />
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                  <button
                    type="button"
                    onClick={() => setIsResolveModalOpen(false)}
                    className="btn"
                    style={{ background: '#f3f4f6', color: '#4b5563' }}
                  >
                    ยกเลิก
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                    disabled={resolveMutation.isPending}
                    style={{ background: '#2563eb', color: '#fff', fontWeight: 600 }}
                  >
                    {resolveMutation.isPending ? 'กำลังบันทึก...' : 'ยืนยันผูกชื่อพนักงาน'}
                  </button>
                </div>
              </form>
            )}

            {/* Tab 2: สร้างเป็นพนักงานใหม่ทันที */}
            {resolveTab === 'new' && (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  if (!newUserForm.employee_id.trim()) {
                    setFormError('กรุณากรอกรหัสพนักงาน');
                    return;
                  }
                  if (!newUserForm.first_name.trim()) {
                    setFormError('กรุณากรอกชื่อจริง');
                    return;
                  }
                  if (!newUserForm.last_name.trim()) {
                    setFormError('กรุณากรอกนามสกุล');
                    return;
                  }
                  createUserAndResolveMutation.mutate({
                    id: resolvingScan.id,
                    data: newUserForm,
                  });
                }}
              >
                <div
                  style={{
                    background: '#eff6ff',
                    border: '1px solid #bfdbfe',
                    borderRadius: '8px',
                    padding: '0.65rem 0.85rem',
                    marginBottom: '1rem',
                    fontSize: '0.82rem',
                    color: '#1e40af',
                    lineHeight: 1.45,
                  }}
                >
                  <strong>ภาพใบหน้านี้จะถูกนำไปใช้เป็นรูปโปรไฟล์ของพนักงาน</strong> และระบบจะส่งงานเรียนรู้ใบหน้า (Enrollment Job) ไปยังตู้ Kiosk ทุกเครื่องโดยอัตโนมัติ เพื่อให้จดจำได้ในครั้งถัดไป
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr', gap: '0.75rem', marginBottom: '0.85rem' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      รหัสพนักงาน <span style={{ color: 'red' }}>*</span>
                    </label>
                    <input
                      type="text"
                      className="input-field"
                      placeholder="เช่น EMP00123"
                      value={newUserForm.employee_id}
                      onChange={(e) => setNewUserForm({ ...newUserForm, employee_id: e.target.value })}
                      required
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      คำนำหน้า <span style={{ color: 'red' }}>*</span>
                    </label>
                    <select
                      className="input-field"
                      value={newUserForm.title}
                      onChange={(e) => setNewUserForm({ ...newUserForm, title: e.target.value })}
                    >
                      {TITLE_PREFIXES.map((prefix) => (
                        <option key={prefix} value={prefix}>
                          {prefix}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '0.85rem' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      ชื่อ <span style={{ color: 'red' }}>*</span>
                    </label>
                    <input
                      type="text"
                      className="input-field"
                      placeholder="เช่น สมชาย"
                      value={newUserForm.first_name}
                      onChange={(e) => setNewUserForm({ ...newUserForm, first_name: e.target.value })}
                      required
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      นามสกุล <span style={{ color: 'red' }}>*</span>
                    </label>
                    <input
                      type="text"
                      className="input-field"
                      placeholder="เช่น ใจดี"
                      value={newUserForm.last_name}
                      onChange={(e) => setNewUserForm({ ...newUserForm, last_name: e.target.value })}
                      required
                    />
                  </div>
                </div>

                <div style={{ marginBottom: '0.85rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                    สังกัด / ฝ่ายงาน <span style={{ color: 'red' }}>*</span>
                  </label>
                  <select
                    className="input-field"
                    value={newUserForm.department}
                    onChange={(e) => setNewUserForm({ ...newUserForm, department: e.target.value })}
                  >
                    {DIVISIONS.map((div) => (
                      <option key={div} value={div}>
                        {div}
                      </option>
                    ))}
                  </select>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr 1fr', gap: '0.75rem', marginBottom: '0.85rem' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      วันที่เริ่มงาน
                    </label>
                    <input
                      type="date"
                      className="input-field"
                      value={newUserForm.start_date}
                      onChange={(e) => setNewUserForm({ ...newUserForm, start_date: e.target.value })}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      ประเภทการสแกน
                    </label>
                    <select
                      className="input-field"
                      value={newUserForm.direction}
                      onChange={(e) => setNewUserForm({ ...newUserForm, direction: e.target.value })}
                    >
                      <option value="IN">เข้างาน (IN)</option>
                      <option value="OUT">ออกงาน (OUT)</option>
                    </select>
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                      เวลาสแกน
                    </label>
                    <input
                      type="time"
                      className="input-field"
                      value={newUserForm.scan_time}
                      onChange={(e) => setNewUserForm({ ...newUserForm, scan_time: e.target.value })}
                    />
                  </div>
                </div>

                <div style={{ marginBottom: '1.25rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.35rem', color: '#334155' }}>
                    หมายเหตุ
                  </label>
                  <input
                    type="text"
                    className="input-field"
                    value={newUserForm.reason}
                    onChange={(e) => setNewUserForm({ ...newUserForm, reason: e.target.value })}
                  />
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                  <button
                    type="button"
                    onClick={() => setIsResolveModalOpen(false)}
                    className="btn"
                    style={{ background: '#f3f4f6', color: '#4b5563' }}
                  >
                    ยกเลิก
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                    disabled={createUserAndResolveMutation.isPending}
                    style={{ background: '#16a34a', color: '#fff', fontWeight: 600 }}
                  >
                    {createUserAndResolveMutation.isPending ? 'กำลังบันทึกและสร้างพนักงาน...' : 'สร้างพนักงานใหม่และบันทึกเวลา'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Modal: บันทึกเวลาแทน (Manual Add) */}
      {isAddModalOpen && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div
            style={{
              background: '#fff',
              borderRadius: '12px',
              padding: '1.75rem',
              width: '100%',
              maxWidth: '480px',
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.15)',
            }}
          >
            <h2 style={{ margin: '0 0 1rem', fontSize: '1.25rem' }}>บันทึกเวลาทำงานแทน (กรณีลืมสแกน/ขาดงาน)</h2>

            {formError && (
              <div
                style={{
                  background: '#fee2e2',
                  color: '#b91c1c',
                  padding: '0.6rem',
                  borderRadius: '6px',
                  marginBottom: '1rem',
                  fontSize: '0.85rem',
                }}
              >
                {formError}
              </div>
            )}

            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!addForm.user_id) {
                  setFormError('กรุณาเลือกพนักงาน');
                  return;
                }
                if (!addForm.reason.trim()) {
                  setFormError('กรุณาระบุเหตุผลในการบันทึกเวลาแทน');
                  return;
                }
                addMutation.mutate(addForm);
              }}
            >
              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                  เลือกพนักงาน <span style={{ color: 'red' }}>*</span>
                </label>
                <select
                  className="input-field"
                  value={addForm.user_id}
                  onChange={(e) => setAddForm({ ...addForm, user_id: e.target.value })}
                  required
                >
                  <option value="">-- กรุณาเลือกพนักงาน --</option>
                  {users?.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.employee_id ? `[${u.employee_id}] ` : ''}
                      {u.name} ({u.department || 'ไม่ระบุแผนก'})
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                  วันที่ทำงาน <span style={{ color: 'red' }}>*</span>
                </label>
                <input
                  type="date"
                  className="input-field"
                  value={addForm.work_date}
                  onChange={(e) => setAddForm({ ...addForm, work_date: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                    เวลาเข้างาน
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={addForm.check_in_time}
                    onChange={(e) => setAddForm({ ...addForm, check_in_time: e.target.value })}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                    เวลาออกงาน
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={addForm.check_out_time}
                    onChange={(e) => setAddForm({ ...addForm, check_out_time: e.target.value })}
                  />
                </div>
              </div>

              <div style={{ marginBottom: '1.25rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                  เหตุผลในการบันทึกแทน (บันทึกใน Audit Log) <span style={{ color: 'red' }}>*</span>
                </label>
                <textarea
                  className="input-field"
                  rows={3}
                  placeholder="เช่น พนักงานลืมสแกนช่วงเช้าเนื่องจากพบลูกค้าข้างนอก หรือไปอบรม"
                  value={addForm.reason}
                  onChange={(e) => setAddForm({ ...addForm, reason: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(false)}
                  className="btn"
                  style={{ background: '#f3f4f6' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={addMutation.isPending}
                  style={{ background: '#2563eb', color: '#fff' }}
                >
                  {addMutation.isPending ? 'กำลังบันทึก...' : 'บันทึกรายการ'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: แก้ไขเวลา (Manual Edit) */}
      {isEditModalOpen && editingPair && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div
            style={{
              background: '#fff',
              borderRadius: '12px',
              padding: '1.75rem',
              width: '100%',
              maxWidth: '480px',
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.15)',
            }}
          >
            <h2 style={{ margin: '0 0 0.5rem', fontSize: '1.25rem' }}>แก้ไขรายการลงเวลา</h2>
            <p style={{ fontSize: '0.85rem', color: '#6b7280', margin: '0 0 1rem' }}>
              พนักงาน: <strong>{editingPair.name}</strong> ({editingPair.employee_id || '-'}) | วันที่:{' '}
              {formatThaiDate(editingPair.work_date)}
            </p>

            {formError && (
              <div
                style={{
                  background: '#fee2e2',
                  color: '#b91c1c',
                  padding: '0.6rem',
                  borderRadius: '6px',
                  marginBottom: '1rem',
                  fontSize: '0.85rem',
                }}
              >
                {formError}
              </div>
            )}

            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!editForm.reason.trim()) {
                  setFormError('กรุณาระบุเหตุผลในการแก้ไขรายการลงเวลา');
                  return;
                }
                editMutation.mutate({
                  id: editingPair.id,
                  data: editForm,
                });
              }}
            >
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                    เวลาเข้างาน
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={editForm.check_in_time}
                    onChange={(e) => setEditForm({ ...editForm, check_in_time: e.target.value })}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                    เวลาออกงาน
                  </label>
                  <input
                    type="time"
                    className="input-field"
                    value={editForm.check_out_time}
                    onChange={(e) => setEditForm({ ...editForm, check_out_time: e.target.value })}
                  />
                </div>
              </div>

              <div style={{ marginBottom: '1.25rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.3rem' }}>
                  เหตุผลในการแก้ไข (บันทึกใน Audit Log) <span style={{ color: 'red' }}>*</span>
                </label>
                <textarea
                  className="input-field"
                  rows={3}
                  placeholder="ระบุเหตุผล เช่น ตรวจสอบภาพวงจรปิดพบสแกนติดแต่เครื่องดับ หรือลืมสแกนออก"
                  value={editForm.reason}
                  onChange={(e) => setEditForm({ ...editForm, reason: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                <button
                  type="button"
                  onClick={() => setIsEditModalOpen(false)}
                  className="btn"
                  style={{ background: '#f3f4f6' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={editMutation.isPending}
                  style={{ background: '#2563eb', color: '#fff' }}
                >
                  {editMutation.isPending ? 'กำลังบันทึก...' : 'บันทึกการแก้ไข'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal ดูภาพถ่ายสแกนขนาดใหญ่ (Zoom Image Modal) */}
      {previewImage && (
        <div
          onClick={() => setPreviewImage(null)}
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(15, 23, 42, 0.75)',
            backdropFilter: 'blur(4px)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 9999,
            cursor: 'zoom-out',
            padding: '1.5rem',
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              position: 'relative',
              background: '#fff',
              borderRadius: '12px',
              padding: '0.75rem',
              boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.35)',
              maxWidth: '90vw',
              maxHeight: '90vh',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
            }}
          >
            <button
              onClick={() => setPreviewImage(null)}
              style={{
                position: 'absolute',
                top: '-12px',
                right: '-12px',
                width: '32px',
                height: '32px',
                borderRadius: '50%',
                background: '#ef4444',
                color: '#fff',
                border: '2px solid #fff',
                fontSize: '1rem',
                fontWeight: 'bold',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
              }}
              title="ปิด"
            >
              ✕
            </button>
            <img
              src={previewImage}
              alt="ภาพสแกนขนาดใหญ่"
              style={{
                maxWidth: '80vw',
                maxHeight: '80vh',
                borderRadius: '8px',
                objectFit: 'contain',
                display: 'block',
              }}
            />
            <div style={{ marginTop: '0.5rem', fontSize: '0.85rem', color: '#64748b' }}>
              ภาพถ่ายหลักฐานขณะสแกนหน้าเครื่อง Kiosk (คลิกพื้นหลังเพื่อปิด)
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
