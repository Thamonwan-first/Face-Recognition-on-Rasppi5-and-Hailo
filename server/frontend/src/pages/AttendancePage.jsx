import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDate, formatThaiTime } from '../utils/date';

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
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10)); // เริ่มต้นด้วยวันนี้เพื่อให้เห็นยอดสรุปและคนขาดงานทันที
  const [endDate, setEndDate] = useState('');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedDevice, setSelectedDevice] = useState('');
  const [statusFilter, setStatusFilter] = useState('ALL');

  // Modals state
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [editingPair, setEditingPair] = useState(null);

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

  // Fetch Attendance Pairs
  const { data: pairs, isLoading, error } = useQuery({
    queryKey: ['attendance_pairs', startDate, endDate, selectedDevice],
    queryFn: () => {
      const params = new URLSearchParams();
      if (startDate) params.append('start', startDate);
      if (endDate) params.append('end', endDate);
      if (selectedDevice) params.append('device_id', selectedDevice);
      params.append('include_absent', 'true');
      const query = params.toString() ? `?${params.toString()}` : '';
      return api.get(`/api/attendance/pairs${query}`);
    },
  });

  // Fetch devices for dropdown filter
  const { data: devices } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices'),
  });

  // Fetch users for dropdown in Manual Add
  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get('/api/users'),
    enabled: isAddModalOpen,
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

  // Calculate status counts for Summary Cards
  const counts = {
    total: pairs?.length || 0,
    on_time: 0,
    late: 0,
    incomplete: 0,
    open: 0,
    adjusted: 0,
    absent: 0,
  };

  pairs?.forEach((p) => {
    if (p.status === 'absent') {
      counts.absent++;
    } else if (p.is_manual || p.status === 'adjusted') {
      counts.adjusted++;
    } else if (p.status === 'late') {
      counts.late++;
    } else if (p.status === 'incomplete') {
      counts.incomplete++;
    } else if (p.status === 'open') {
      counts.open++;
    } else {
      counts.on_time++;
    }
  });

  // Filtering
  const filteredPairs = pairs?.filter((p) => {
    // 1. Search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      const match =
        p.name?.toLowerCase().includes(term) ||
        p.employee_id?.toLowerCase().includes(term) ||
        p.department?.toLowerCase().includes(term);
      if (!match) return false;
    }

    // 2. Status filter
    if (statusFilter !== 'ALL') {
      if (statusFilter === 'ABSENT') {
        return p.status === 'absent';
      }
      if (statusFilter === 'ADJUSTED') {
        return p.is_manual || p.status === 'adjusted';
      }
      if (statusFilter === 'ON_TIME') {
        return !p.is_manual && (p.status === 'on_time' || p.status === 'closed' || p.status === 'complete');
      }
      if (statusFilter === 'LATE') {
        return !p.is_manual && p.status === 'late';
      }
      if (statusFilter === 'INCOMPLETE') {
        return !p.is_manual && p.status === 'incomplete';
      }
      if (statusFilter === 'OPEN') {
        return !p.is_manual && p.status === 'open';
      }
    }

    return true;
  });

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
      reason: 'บันทึกเวลาแทนเนื่องจากพบขาดการสแกน',
    });
    setFormError('');
    setIsAddModalOpen(true);
  };

  const handleExportCSV = () => {
    if (!filteredPairs || filteredPairs.length === 0) return;
    const headers = [
      'วันที่',
      'รหัสพนักงาน',
      'ชื่อ-นามสกุล',
      'ฝ่าย',
      'อุปกรณ์เข้า',
      'อุปกรณ์ออก',
      'เวลาเข้างาน',
      'เวลาออกงาน',
      'รวมชั่วโมงทำงาน',
      'สถานะ',
      'แก้ไขโดยผู้ดูแล',
      'เหตุผลการแก้ไข',
    ];
    const rows = filteredPairs.map((p) => {
      let statusLabel = 'ตรงเวลา';
      if (p.status === 'absent') statusLabel = 'ขาดงาน';
      else if (p.is_manual || p.status === 'adjusted') statusLabel = 'แก้ไขโดยผู้ดูแล';
      else if (p.status === 'late') statusLabel = 'มาสาย';
      else if (p.status === 'incomplete') statusLabel = 'ไม่ครบเวลา';
      else if (p.status === 'open') statusLabel = 'กำลังทำงาน';

      return [
        p.work_date,
        p.employee_id,
        p.name,
        p.department,
        p.check_in_device_id || '-',
        p.check_out_device_id || '-',
        formatThaiTime(p.check_in_time),
        formatThaiTime(p.check_out_time),
        p.hours_worked !== null ? `${p.hours_worked} ชม.` : '-',
        statusLabel,
        p.is_manual ? 'ใช่' : 'ไม่ใช่',
        p.adjustment_reason || '',
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
    if (p.status === 'absent') {
      return (
        <span
          style={{
            padding: '0.25rem 0.6rem',
            borderRadius: '20px',
            fontSize: '0.8rem',
            background: '#fee2e2',
            color: '#b91c1c',
            fontWeight: 700,
            border: '1px solid #f87171',
          }}
        >
          ขาดงาน
        </span>
      );
    }
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
            <div style={{ fontSize: '0.75rem', color: '#666', marginTop: '0.2rem', fontStyle: 'italic' }}>
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
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 className="page-title">ประวัติการบันทึกเวลาเข้า-ออกงาน</h1>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.88rem', margin: '0.2rem 0 0' }}>
            สรุปเวลาทำงานรายวัน พร้อมระบบตรวจจับคนมาสายและคนขาดงานอัตโนมัติ
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
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
            + บันทึกเวลาแทน (ลืมสแกน)
          </button>
          <button onClick={handleExportCSV} className="btn btn-primary" disabled={!filteredPairs?.length}>
            ส่งออกรายงาน CSV
          </button>
        </div>
      </div>

      {/* Summary Status Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))',
          gap: '0.75rem',
          marginBottom: '1.5rem',
        }}
      >
        <div
          onClick={() => setStatusFilter('ALL')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'ALL' ? '2px solid #2563eb' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#6b7280', fontWeight: 500 }}>ทั้งหมด</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#111827', marginTop: '0.2rem' }}>
            {counts.total}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('ON_TIME')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'ON_TIME' ? '2px solid #16a34a' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#16a34a', fontWeight: 500 }}>ตรงเวลา</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#16a34a', marginTop: '0.2rem' }}>
            {counts.on_time}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('LATE')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'LATE' ? '2px solid #d97706' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#d97706', fontWeight: 500 }}>มาสาย</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#d97706', marginTop: '0.2rem' }}>
            {counts.late}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('INCOMPLETE')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'INCOMPLETE' ? '2px solid #dc2626' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#dc2626', fontWeight: 500 }}>ไม่ครบเวลา</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#dc2626', marginTop: '0.2rem' }}>
            {counts.incomplete}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('ABSENT')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'ABSENT' ? '2px solid #b91c1c' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#b91c1c', fontWeight: 700 }}>ขาดงาน</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 800, color: '#b91c1c', marginTop: '0.2rem' }}>
            {counts.absent}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('OPEN')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'OPEN' ? '2px solid #2563eb' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#2563eb', fontWeight: 500 }}>กำลังทำงาน</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#2563eb', marginTop: '0.2rem' }}>
            {counts.open}
          </div>
        </div>

        <div
          onClick={() => setStatusFilter('ADJUSTED')}
          style={{
            background: '#fff',
            padding: '0.85rem 1rem',
            borderRadius: '8px',
            border: statusFilter === 'ADJUSTED' ? '2px solid #9333ea' : '1px solid #e5e7eb',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <div style={{ fontSize: '0.78rem', color: '#9333ea', fontWeight: 500 }}>แก้ไขโดยผู้ดูแล</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 700, color: '#9333ea', marginTop: '0.2rem' }}>
            {counts.adjusted}
          </div>
        </div>
      </div>

      {/* แถบตัวกรองวันที่, เครื่องสแกน และค้นหาพนักงาน */}
      <div
        style={{
          display: 'flex',
          gap: '1rem',
          flexWrap: 'wrap',
          alignItems: 'flex-end',
          marginBottom: '1.5rem',
          background: '#fff',
          padding: '1.25rem',
          borderRadius: '8px',
          boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
        }}
      >
        <div>
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500 }}>
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
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500 }}>
            ถึงวันที่
          </label>
          <input
            type="date"
            className="input-field"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
          />
        </div>

        {/* Dropdown กรองตามเครื่อง (Filter by Device) */}
        <div style={{ minWidth: '180px' }}>
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500 }}>
            อุปกรณ์ / เครื่องสแกน
          </label>
          <select
            className="input-field"
            value={selectedDevice}
            onChange={(e) => setSelectedDevice(e.target.value)}
          >
            <option value="">-- ทั้งหมดทุกเครื่อง --</option>
            {devices?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name || d.id} {d.location ? `(${d.location})` : ''}
              </option>
            ))}
          </select>
        </div>

        <div style={{ flex: '1', minWidth: '220px' }}>
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.3rem', fontWeight: 500 }}>
            ค้นหาพนักงาน / แผนก
          </label>
          <input
            type="text"
            className="input-field"
            placeholder="พิมพ์ชื่อ, รหัสพนักงาน หรือฝ่าย..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        {(startDate || endDate || searchTerm || selectedDevice || statusFilter !== 'ALL') && (
          <div>
            <button
              onClick={() => {
                setStartDate('');
                setEndDate('');
                setSearchTerm('');
                setSelectedDevice('');
                setStatusFilter('ALL');
              }}
              className="btn"
              style={{ background: '#f1f2f6', color: '#555' }}
            >
              ล้างตัวกรอง
            </button>
          </div>
        )}
      </div>

      {isLoading && <p>กำลังโหลดประวัติลงเวลา...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      {/* ตารางแสดงการเข้า-ออกงาน */}
      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>วันที่</th>
              <th>รหัสพนักงาน</th>
              <th>ชื่อ-นามสกุล</th>
              <th>ฝ่าย</th>
              <th>เครื่องสแกน</th>
              <th>เวลาเข้างาน</th>
              <th>เวลาออกงาน</th>
              <th>รวมชั่วโมง</th>
            </tr>
          </thead>
          <tbody>
            {filteredPairs?.map((p) => {
              const isAbsent = p.status === 'absent';

              return (
                <tr
                  key={p.id ? `pair-${p.id}` : `absent-${p.user_id}-${p.work_date}`}
                  style={{
                    backgroundColor: isAbsent ? '#fff5f5' : undefined,
                  }}
                >
                  <td>
                    <strong>{formatThaiDate(p.work_date)}</strong>
                  </td>
                  <td>
                    <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{p.employee_id}</span>
                  </td>
                  <td>{p.name}</td>
                  <td>{p.department}</td>
                  <td>
                    {p.check_in_device_id || p.check_out_device_id ? (
                      <span
                        style={{
                          fontSize: '0.78rem',
                          padding: '2px 6px',
                          background: '#f1f5f9',
                          borderRadius: '4px',
                          fontFamily: 'monospace',
                        }}
                      >
                        {p.check_in_device_id || p.check_out_device_id}
                      </span>
                    ) : (
                      <span style={{ color: 'var(--text-muted)' }}>-</span>
                    )}
                  </td>
                  <td>
                    {p.check_in_time ? (
                      <span style={{ color: 'var(--success)', fontWeight: 'bold' }}>
                        {formatThaiTime(p.check_in_time)}
                      </span>
                    ) : (
                      <span style={{ color: isAbsent ? '#b91c1c' : 'var(--text-muted)' }}>
                        {isAbsent ? 'ไม่พบการสแกน' : '-'}
                      </span>
                    )}
                  </td>
                  <td>
                    {p.check_out_time ? (
                      <span style={{ color: '#d63031', fontWeight: 'bold' }}>
                        {formatThaiTime(p.check_out_time)}
                      </span>
                    ) : (
                      <span style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>
                        {isAbsent ? '-' : 'ยังไม่ออกงาน'}
                      </span>
                    )}
                  </td>
                  <td>
                    {p.hours_worked !== null && !isAbsent ? (
                      <span style={{ fontWeight: 600 }}>{Number(p.hours_worked).toFixed(1)} ชม.</span>
                    ) : (
                      <span style={{ color: 'var(--text-muted)' }}>-</span>
                    )}
                  </td>
                </tr>
              );
            })}
            {filteredPairs?.length === 0 && (
              <tr>
                <td colSpan="8" style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '2rem' }}>
                  ไม่พบข้อมูลการบันทึกเวลาในช่วงที่เลือก
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

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
              borderRadius: '8px',
              padding: '1.75rem',
              width: '100%',
              maxWidth: '480px',
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1)',
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
              borderRadius: '8px',
              padding: '1.75rem',
              width: '100%',
              maxWidth: '480px',
              boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1)',
            }}
          >
            <h2 style={{ margin: '0 0 0.5rem', fontSize: '1.25rem' }}>แก้ไขรายการลงเวลา</h2>
            <p style={{ fontSize: '0.85rem', color: '#6b7280', margin: '0 0 1rem' }}>
              พนักงาน: <strong>{editingPair.name}</strong> ({editingPair.employee_id}) | วันที่:{' '}
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
    </div>
  );
}
