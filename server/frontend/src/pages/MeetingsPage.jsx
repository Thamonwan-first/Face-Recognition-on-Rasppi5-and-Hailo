import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api';
import { useRealtime } from '../hooks/useRealtime';
import { formatThaiDateTime } from '../utils/date';

const INITIAL_MEETING_FORM = {
  title: '',
  description: '',
  location: '',
  device_id: '',
  meeting_date: new Date().toISOString().slice(0, 10),
  start_time: '09:00',
  end_time: '12:00',
  late_threshold_minutes: 15,
  status: 'scheduled',
};

export default function MeetingsPage() {
  const queryClient = useQueryClient();
  useRealtime();

  const [selectedMeetingId, setSelectedMeetingId] = useState(null);
  const [showModal, setShowModal] = useState(false);
  const [editMeetingData, setEditMeetingData] = useState(null);
  const [formData, setFormData] = useState(INITIAL_MEETING_FORM);
  const [previewImage, setPreviewImage] = useState(null);

  // Filters
  const [dateFilter, setDateFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('ALL');

  // 1. Fetch meetings list
  const { data: rawMeetings, isLoading: isMeetingsLoading } = useQuery({
    queryKey: ['meetings', dateFilter, statusFilter],
    queryFn: () => {
      const params = new URLSearchParams();
      if (dateFilter) params.append('date', dateFilter);
      if (statusFilter && statusFilter !== 'ALL') params.append('status', statusFilter);
      const queryStr = params.toString() ? `?${params.toString()}` : '';
      return api.get(`/api/meetings${queryStr}`);
    },
    refetchInterval: 3000,
  });
  const meetings = Array.isArray(rawMeetings) ? rawMeetings : [];

  // 2. Fetch devices list for dropdown
  const { data: rawDevices } = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.get('/api/devices/status'),
  });
  const devices = Array.isArray(rawDevices) ? rawDevices : [];

  // 3. Fetch selected meeting details & attendees if a meeting is open
  const selectedMeeting = meetings.find((m) => m.id === selectedMeetingId);

  const { data: rawAttendees, isLoading: isAttendeesLoading } = useQuery({
    queryKey: ['meeting_attendees', selectedMeetingId],
    queryFn: () => api.get(`/api/meetings/${selectedMeetingId}/attendees`),
    enabled: !!selectedMeetingId,
    refetchInterval: 2500,
  });
  const attendees = Array.isArray(rawAttendees) ? rawAttendees : [];

  // Mutations
  const createMutation = useMutation({
    mutationFn: (newMeeting) => api.post('/api/meetings', newMeeting),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['meetings'] });
      setShowModal(false);
      setFormData(INITIAL_MEETING_FORM);
    },
    onError: (err) => alert('เกิดข้อผิดพลาดในการสร้างการประชุม: ' + (err.message || err)),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }) => api.put(`/api/meetings/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['meetings'] });
      setShowModal(false);
      setEditMeetingData(null);
      setFormData(INITIAL_MEETING_FORM);
    },
    onError: (err) => alert('เกิดข้อผิดพลาดในการแก้ไขการประชุม: ' + (err.message || err)),
  });

  const deleteMutation = useMutation({
    mutationFn: (id) => api.del(`/api/meetings/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['meetings'] });
      if (selectedMeetingId) setSelectedMeetingId(null);
    },
    onError: (err) => alert('เกิดข้อผิดพลาดในการลบการประชุม: ' + (err.message || err)),
  });

  const handleOpenCreate = () => {
    setEditMeetingData(null);
    setFormData(INITIAL_MEETING_FORM);
    setShowModal(true);
  };

  const handleOpenEdit = (m) => {
    setEditMeetingData(m);
    setFormData({
      title: m.title || '',
      description: m.description || '',
      location: m.location || '',
      device_id: m.device_id || '',
      meeting_date: m.meeting_date || '',
      start_time: m.start_time || '09:00',
      end_time: m.end_time || '12:00',
      late_threshold_minutes: m.late_threshold_minutes ?? 15,
      status: m.status || 'scheduled',
    });
    setShowModal(true);
  };

  const handleFormSubmit = (e) => {
    e.preventDefault();
    if (!formData.title.trim()) {
      alert('กรุณากรอกหัวข้อการประชุม');
      return;
    }
    const payload = {
      ...formData,
      device_id: formData.device_id ? formData.device_id : null,
      late_threshold_minutes: Number(formData.late_threshold_minutes) || 0,
    };
    if (editMeetingData) {
      updateMutation.mutate({ id: editMeetingData.id, data: payload });
    } else {
      createMutation.mutate(payload);
    }
  };

  const handleDeleteMeeting = (m) => {
    if (window.confirm(`ยืนยันการลบการประชุม "${m.title}" หรือไม่?`)) {
      deleteMutation.mutate(m.id);
    }
  };

  const getStatusBadge = (status) => {
    switch (status) {
      case 'ongoing':
        return (
          <span style={{ padding: '0.25rem 0.6rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#dcfce7', color: '#15803d', border: '1px solid #86efac' }}>
            กำลังประชุม
          </span>
        );
      case 'completed':
        return (
          <span style={{ padding: '0.25rem 0.6rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#f1f5f9', color: '#475569', border: '1px solid #cbd5e1' }}>
            เสร็จสิ้น
          </span>
        );
      case 'cancelled':
        return (
          <span style={{ padding: '0.25rem 0.6rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#fee2e2', color: '#b91c1c', border: '1px solid #fca5a5' }}>
            ยกเลิก
          </span>
        );
      case 'scheduled':
      default:
        return (
          <span style={{ padding: '0.25rem 0.6rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#eff6ff', color: '#1d4ed8', border: '1px solid #bfdbfe' }}>
            กำหนดการ
          </span>
        );
    }
  };

  // Export functions
  const handleExportCSV = () => {
    if (!attendees || attendees.length === 0) {
      alert('ไม่มีข้อมูลผู้เข้าร่วมประชุมที่จะส่งออก');
      return;
    }
    const headers = ['ลำดับ', 'รูปสแกน (URL)', 'รหัสพนักงาน', 'ชื่อ-นามสกุล', 'สังกัด/ฝ่าย', 'เวลาที่สแกน', 'สถานะ', 'มาสาย (นาที)'];
    const rows = attendees.map((a, idx) => [
      idx + 1,
      a.snapshot_url || '-',
      a.employee_id || '-',
      a.user_name || '-',
      a.department || '-',
      formatThaiDateTime(a.scanned_at),
      a.status === 'on_time' ? 'ตรงเวลา' : 'มาสาย',
      a.minutes_late || 0,
    ]);
    const csvContent = '\uFEFF' + [headers.join(','), ...rows.map((r) => r.map((c) => `"${c}"`).join(','))].join('\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', `meeting_${selectedMeeting?.title || 'attendees'}_${selectedMeeting?.meeting_date || ''}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const handleExportPDF = () => {
    if (!attendees || attendees.length === 0) {
      alert('ไม่มีข้อมูลผู้เข้าร่วมประชุมที่จะส่งออก');
      return;
    }
    const printWin = window.open('', '_blank');
    if (!printWin) {
      alert('กรุณาอนุญาตป๊อปอัปเพื่อพิมพ์รายงาน');
      return;
    }

    const rowsHtml = attendees
      .map(
        (a, idx) => `
        <tr>
          <td style="text-align: center; vertical-align: middle;">${idx + 1}</td>
          <td style="text-align: center; vertical-align: middle; padding: 4px;">
            ${
              a.snapshot_url
                ? `<img src="${a.snapshot_url}" alt="รูปสแกน" style="width: 44px; height: 44px; border-radius: 4px; object-fit: cover; border: 1px solid #cbd5e1; display: block; margin: 0 auto;" />`
                : `<span style="font-size: 11px; color: #94a3b8;">ไม่มีรูป</span>`
            }
          </td>
          <td style="text-align: center; vertical-align: middle;">${a.employee_id || '-'}</td>
          <td style="vertical-align: middle; font-weight: 500;">${a.user_name || '-'}</td>
          <td style="vertical-align: middle;">${a.department || '-'}</td>
          <td style="text-align: center; vertical-align: middle;">${formatThaiDateTime(a.scanned_at)}</td>
          <td style="text-align: center; vertical-align: middle; color: ${a.status === 'on_time' ? '#15803d' : '#b91c1c'}; font-weight: bold;">
            ${a.status === 'on_time' ? 'ตรงเวลา' : `มาสาย (${a.minutes_late} นาที)`}
          </td>
        </tr>
      `
      )
      .join('');

    const html = `
      <!DOCTYPE html>
      <html>
      <head>
        <meta charset="utf-8">
        <title>รายงานผู้เข้าร่วมประชุม: ${selectedMeeting?.title || ''}</title>
        <style>
          body { font-family: 'Sarabun', -apple-system, BlinkMacSystemFont, sans-serif; padding: 2rem; color: #1e293b; }
          h1 { margin-bottom: 0.25rem; font-size: 1.5rem; text-align: center; }
          .sub { text-align: center; color: #64748b; margin-bottom: 1.5rem; font-size: 0.95rem; }
          table { width: 100%; border-collapse: collapse; margin-top: 1rem; font-size: 0.9rem; }
          th, td { border: 1px solid #cbd5e1; padding: 8px 12px; }
          th { background: #f8fafc; font-weight: 600; text-align: left; }
          .summary { display: flex; justify-content: space-around; margin: 1rem 0; padding: 1rem; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; }
          @media print { .no-print { display: none; } }
        </style>
      </head>
      <body>
        <div class="no-print" style="margin-bottom: 1rem; text-align: right;">
          <button onclick="window.print()" style="padding: 8px 16px; background: #2563eb; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 14px;">พิมพ์เอกสาร / บันทึก PDF</button>
        </div>
        <h1>รายงานสรุปการเข้าร่วมประชุม</h1>
        <div class="sub">
          <strong>หัวข้อ:</strong> ${selectedMeeting?.title || '-'} | 
          <strong>วันที่:</strong> ${selectedMeeting?.meeting_date || '-'} | 
          <strong>เวลา:</strong> ${selectedMeeting?.start_time || ''} - ${selectedMeeting?.end_time || ''} น.<br>
          <strong>สถานที่:</strong> ${selectedMeeting?.location || '-'} (${selectedMeeting?.device_name || 'ไม่ระบุเครื่อง'})
        </div>
        <div class="summary">
          <div><strong>ผู้เข้าร่วมทั้งหมด:</strong> ${attendees.length} คน</div>
          <div><strong>ตรงเวลา:</strong> ${attendees.filter((a) => a.status === 'on_time').length} คน</div>
          <div><strong>มาสาย:</strong> ${attendees.filter((a) => a.status === 'late').length} คน</div>
        </div>
        <table>
          <thead>
            <tr>
              <th style="width: 45px; text-align: center;">ลำดับ</th>
              <th style="width: 65px; text-align: center;">รูปสแกน</th>
              <th style="width: 95px; text-align: center;">รหัสพนักงาน</th>
              <th>ชื่อ-นามสกุล</th>
              <th>สังกัด/ฝ่าย</th>
              <th style="width: 130px; text-align: center;">เวลาที่สแกน</th>
              <th style="width: 130px; text-align: center;">สถานะ</th>
            </tr>
          </thead>
          <tbody>
            ${rowsHtml}
          </tbody>
        </table>
      </body>
      </html>
    `;
    printWin.document.write(html);
    printWin.document.close();
  };

  // Compute live stats
  const totalMeetings = meetings.length;
  const ongoingMeetings = meetings.filter((m) => m.status === 'ongoing').length;
  const totalAttendees = meetings.reduce((acc, m) => acc + (m.attendee_count || 0), 0);

  return (
    <div style={{ paddingBottom: '3rem' }}>
      {/* ----------------- VIEW 1: MEETINGS LIST ----------------- */}
      {!selectedMeetingId && (
        <>
          <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '1rem' }}>
            <div>
              <h1 className="page-title">การประชุม</h1>
              <p style={{ margin: '0.25rem 0 0', color: 'var(--text-muted)', fontSize: '0.9rem' }}>
                จัดการวาระการประชุมและตรวจสอบรายชื่อผู้เข้าร่วมประชุมด้วยการสแกนใบหน้า
              </p>
            </div>
            <button
              onClick={handleOpenCreate}
              style={{
                backgroundColor: '#2563eb',
                color: '#ffffff',
                border: 'none',
                borderRadius: '6px',
                padding: '0.55rem 1.1rem',
                fontSize: '0.9rem',
                fontWeight: 600,
                cursor: 'pointer',
              }}
            >
              + สร้างการประชุมใหม่
            </button>
          </div>

          {/* Quick Stats Bar */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem', margin: '1.25rem 0' }}>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#64748b', fontWeight: 600 }}>การประชุมทั้งหมด</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#0f172a', marginTop: '0.25rem' }}>{totalMeetings}</div>
            </div>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#15803d', fontWeight: 600 }}>กำลังประชุมอยู่</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#15803d', marginTop: '0.25rem' }}>{ongoingMeetings}</div>
            </div>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#1d4ed8', fontWeight: 600 }}>ผู้เข้าร่วมทั้งหมด</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#1d4ed8', marginTop: '0.25rem' }}>{totalAttendees} คน</div>
            </div>
          </div>

          {/* Filters Bar */}
          <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '0.75rem 1rem', display: 'flex', gap: '1rem', flexWrap: 'wrap', alignItems: 'center', marginBottom: '1.25rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <label style={{ fontSize: '0.85rem', color: '#475569', fontWeight: 600 }}>วันที่:</label>
              <input
                type="date"
                value={dateFilter}
                onChange={(e) => setDateFilter(e.target.value)}
                style={{ padding: '0.35rem 0.6rem', fontSize: '0.85rem', borderRadius: '6px', border: '1px solid #cbd5e1' }}
              />
              {dateFilter && (
                <button
                  onClick={() => setDateFilter('')}
                  style={{ background: 'none', border: 'none', color: '#64748b', cursor: 'pointer', fontSize: '0.85rem' }}
                >
                  ล้าง
                </button>
              )}
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <label style={{ fontSize: '0.85rem', color: '#475569', fontWeight: 600 }}>สถานะ:</label>
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                style={{ padding: '0.35rem 0.6rem', fontSize: '0.85rem', borderRadius: '6px', border: '1px solid #cbd5e1' }}
              >
                <option value="ALL">ทั้งหมด</option>
                <option value="scheduled">กำหนดการ</option>
                <option value="ongoing">กำลังประชุม</option>
                <option value="completed">เสร็จสิ้น</option>
                <option value="cancelled">ยกเลิก</option>
              </select>
            </div>
          </div>

          {/* Meetings Table */}
          {isMeetingsLoading ? (
            <p style={{ textAlign: 'center', padding: '2rem', color: '#64748b' }}>กำลังโหลดข้อมูลการประชุม...</p>
          ) : meetings.length === 0 ? (
            <div style={{ background: '#fff', border: '1px dashed #cbd5e1', borderRadius: '8px', padding: '3rem', textAlign: 'center', color: '#64748b' }}>
              ไม่พบรายการการประชุมตามเงื่อนไขที่เลือก คลิกปุ่ม <strong>"+ สร้างการประชุมใหม่"</strong> เพื่อเพิ่มวาระการประชุม
            </div>
          ) : (
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.9rem' }}>
                <thead>
                  <tr style={{ background: '#f8fafc', borderBottom: '1px solid #e2e8f0', color: '#475569', fontWeight: 600 }}>
                    <th style={{ padding: '0.75rem 1rem' }}>หัวข้อการประชุม</th>
                    <th style={{ padding: '0.75rem 1rem' }}>สถานที่ / เครื่อง Kiosk</th>
                    <th style={{ padding: '0.75rem 1rem' }}>วันที่</th>
                    <th style={{ padding: '0.75rem 1rem' }}>ช่วงเวลา</th>
                    <th style={{ padding: '0.75rem 1rem' }}>สถานะ</th>
                    <th style={{ padding: '0.75rem 1rem', textAlign: 'center' }}>ผู้เข้าร่วม</th>
                    <th style={{ padding: '0.75rem 1rem', textAlign: 'center' }}>จัดการ</th>
                  </tr>
                </thead>
                <tbody>
                  {meetings.map((m) => (
                    <tr key={m.id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ padding: '0.85rem 1rem', fontWeight: 600, color: '#0f172a' }}>
                        <div>{m.title}</div>
                        {m.description && <div style={{ fontSize: '0.75rem', color: '#64748b', fontWeight: 400 }}>{m.description}</div>}
                      </td>
                      <td style={{ padding: '0.85rem 1rem' }}>
                        <div>{m.location || '-'}</div>
                        <div style={{ fontSize: '0.75rem', color: '#64748b' }}>{m.device_name ? `เครื่อง: ${m.device_name}` : 'ไม่ผูกเครื่อง'}</div>
                      </td>
                      <td style={{ padding: '0.85rem 1rem' }}>{m.meeting_date}</td>
                      <td style={{ padding: '0.85rem 1rem' }}>
                        {m.start_time} - {m.end_time} น.
                      </td>
                      <td style={{ padding: '0.85rem 1rem' }}>{getStatusBadge(m.status)}</td>
                      <td style={{ padding: '0.85rem 1rem', textAlign: 'center', fontWeight: 600, color: '#1d4ed8' }}>
                        {m.attendee_count} คน
                      </td>
                      <td style={{ padding: '0.85rem 1rem', textAlign: 'center' }}>
                        <div style={{ display: 'inline-flex', gap: '0.4rem' }}>
                          <button
                            onClick={() => setSelectedMeetingId(m.id)}
                            style={{ padding: '0.3rem 0.65rem', fontSize: '0.8rem', borderRadius: '4px', background: '#2563eb', color: '#fff', border: 'none', cursor: 'pointer', fontWeight: 600 }}
                          >
                            เช็คชื่อ/ดูรายชื่อ
                          </button>
                          <button
                            onClick={() => handleOpenEdit(m)}
                            style={{ padding: '0.3rem 0.65rem', fontSize: '0.8rem', borderRadius: '4px', background: '#f1f5f9', color: '#334155', border: '1px solid #cbd5e1', cursor: 'pointer' }}
                          >
                            แก้ไข
                          </button>
                          <button
                            onClick={() => handleDeleteMeeting(m)}
                            style={{ padding: '0.3rem 0.65rem', fontSize: '0.8rem', borderRadius: '4px', background: '#fee2e2', color: '#b91c1c', border: '1px solid #fca5a5', cursor: 'pointer' }}
                          >
                            ลบ
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* ----------------- VIEW 2: LIVE ATTENDEES MONITOR ----------------- */}
      {selectedMeetingId && selectedMeeting && (
        <>
          <div style={{ marginBottom: '1.25rem' }}>
            <button
              onClick={() => setSelectedMeetingId(null)}
              style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.9rem', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '0.35rem', padding: 0 }}
            >
              &larr; กลับหน้ารายการประชุม
            </button>
          </div>

          <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '1rem', background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1.25rem' }}>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
                <h1 style={{ fontSize: '1.35rem', fontWeight: 700, margin: 0, color: '#0f172a' }}>{selectedMeeting.title}</h1>
                {getStatusBadge(selectedMeeting.status)}
              </div>
              <div style={{ fontSize: '0.85rem', color: '#64748b', marginTop: '0.35rem' }}>
                <strong>วันที่:</strong> {selectedMeeting.meeting_date} | <strong>เวลา:</strong> {selectedMeeting.start_time} - {selectedMeeting.end_time} น. | <strong>สถานที่:</strong> {selectedMeeting.location || '-'} ({selectedMeeting.device_name || 'ไม่ระบุเครื่อง'}) | <strong>เกณฑ์สาย:</strong> หลัง {selectedMeeting.late_threshold_minutes} นาที
              </div>
            </div>

            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button
                onClick={handleExportPDF}
                style={{ background: '#0284c7', color: '#fff', border: 'none', borderRadius: '6px', padding: '0.45rem 0.85rem', fontSize: '0.85rem', fontWeight: 600, cursor: 'pointer' }}
              >
                ส่งออก PDF
              </button>
              <button
                onClick={handleExportCSV}
                style={{ background: '#16a34a', color: '#fff', border: 'none', borderRadius: '6px', padding: '0.45rem 0.85rem', fontSize: '0.85rem', fontWeight: 600, cursor: 'pointer' }}
              >
                ส่งออก Excel
              </button>
            </div>
          </div>

          {/* Quick Attendee Stats */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem', margin: '1.25rem 0' }}>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#64748b', fontWeight: 600 }}>เข้าร่วมแล้ว</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#0f172a', marginTop: '0.25rem' }}>{attendees.length} คน</div>
            </div>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#15803d', fontWeight: 600 }}>ตรงเวลา</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#15803d', marginTop: '0.25rem' }}>
                {attendees.filter((a) => a.status === 'on_time').length} คน
              </div>
            </div>
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '1rem' }}>
              <div style={{ fontSize: '0.8rem', color: '#b91c1c', fontWeight: 600 }}>มาสาย</div>
              <div style={{ fontSize: '1.5rem', fontWeight: 700, color: '#b91c1c', marginTop: '0.25rem' }}>
                {attendees.filter((a) => a.status === 'late').length} คน
              </div>
            </div>
          </div>

          {/* Attendees Real-time Table */}
          {isAttendeesLoading ? (
            <p style={{ textAlign: 'center', padding: '2rem', color: '#64748b' }}>กำลังโหลดรายชื่อผู้เข้าประชุม...</p>
          ) : attendees.length === 0 ? (
            <div style={{ background: '#fff', border: '1px dashed #cbd5e1', borderRadius: '8px', padding: '3rem', textAlign: 'center', color: '#64748b' }}>
              ยังไม่มีผู้สแกนหน้าลงทะเบียนเข้าประชุม เมื่อพนักงานเดินสแกนที่เครื่อง Kiosk ข้อมูลจะแสดงที่นี่แบบเรียลไทม์
            </div>
          ) : (
            <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: '8px', overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.9rem' }}>
                <thead>
                  <tr style={{ background: '#f8fafc', borderBottom: '1px solid #e2e8f0', color: '#475569', fontWeight: 600 }}>
                    <th style={{ padding: '0.75rem 1rem', width: '70px', textAlign: 'center' }}>รูปสแกน</th>
                    <th style={{ padding: '0.75rem 1rem', width: '110px' }}>รหัสพนักงาน</th>
                    <th style={{ padding: '0.75rem 1rem' }}>ชื่อ-นามสกุล</th>
                    <th style={{ padding: '0.75rem 1rem' }}>สังกัด/ฝ่าย</th>
                    <th style={{ padding: '0.75rem 1rem', width: '140px' }}>เวลาที่สแกน</th>
                    <th style={{ padding: '0.75rem 1rem', width: '130px' }}>สถานะ</th>
                  </tr>
                </thead>
                <tbody>
                  {attendees.map((a) => (
                    <tr key={a.id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                      <td style={{ padding: '0.65rem 1rem', textAlign: 'center' }}>
                        {a.snapshot_url ? (
                          <img
                            src={a.snapshot_url}
                            alt="ภาพสแกน"
                            onClick={() =>
                              setPreviewImage({
                                url: a.snapshot_url,
                                title: `ภาพสแกนเข้าประชุม: ${a.user_name || ''} (${a.employee_id || '-'}) เวลา ${formatThaiDateTime(a.scanned_at)}`,
                              })
                            }
                            title="คลิกเพื่อดูรูปภาพขนาดใหญ่"
                            style={{
                              width: '44px',
                              height: '44px',
                              borderRadius: '6px',
                              objectFit: 'cover',
                              border: '1px solid #cbd5e1',
                              cursor: 'pointer',
                              display: 'inline-block',
                            }}
                          />
                        ) : (
                          <div
                            style={{
                              width: '44px',
                              height: '44px',
                              borderRadius: '6px',
                              background: '#f1f5f9',
                              color: '#94a3b8',
                              fontSize: '0.65rem',
                              display: 'inline-flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              border: '1px dashed #cbd5e1',
                            }}
                          >
                            ไม่มีรูป
                          </div>
                        )}
                      </td>
                      <td style={{ padding: '0.75rem 1rem', fontFamily: 'monospace', fontWeight: 600, color: '#1e293b' }}>
                        {a.employee_id || '-'}
                      </td>
                      <td style={{ padding: '0.75rem 1rem', fontWeight: 600, color: '#0f172a' }}>{a.user_name}</td>
                      <td style={{ padding: '0.75rem 1rem', color: '#475569' }}>{a.department || '-'}</td>
                      <td style={{ padding: '0.75rem 1rem', fontWeight: 600, color: '#0f172a' }}>
                        {formatThaiDateTime(a.scanned_at)}
                      </td>
                      <td style={{ padding: '0.75rem 1rem' }}>
                        {a.status === 'on_time' ? (
                          <span style={{ padding: '0.2rem 0.5rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#dcfce7', color: '#15803d', border: '1px solid #86efac' }}>
                            ตรงเวลา
                          </span>
                        ) : (
                          <span style={{ padding: '0.2rem 0.5rem', borderRadius: '4px', fontSize: '0.8rem', fontWeight: 600, background: '#ffedd5', color: '#c2410c', border: '1px solid #fdba74' }}>
                            มาสาย {a.minutes_late > 0 ? `(${a.minutes_late} นาที)` : ''}
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* ----------------- MODAL: CREATE / EDIT MEETING ----------------- */}
      {showModal && (
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
              borderRadius: '8px',
              width: '100%',
              maxWidth: '540px',
              padding: '1.5rem',
              boxShadow: '0 10px 25px rgba(0,0,0,0.2)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', borderBottom: '1px solid #e2e8f0', paddingBottom: '0.75rem' }}>
              <h3 style={{ margin: 0, fontSize: '1.15rem', fontWeight: 700, color: '#0f172a' }}>
                {editMeetingData ? 'แก้ไขการประชุม' : 'สร้างการประชุมใหม่'}
              </h3>
              <button
                onClick={() => setShowModal(false)}
                style={{ background: 'none', border: 'none', fontSize: '1.2rem', cursor: 'pointer', color: '#64748b' }}
              >
                &times;
              </button>
            </div>

            <form onSubmit={handleFormSubmit}>
              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                  หัวข้อการประชุม *
                </label>
                <input
                  type="text"
                  required
                  placeholder="เช่น ประชุมสรุปยอดขายประจำสัปดาห์"
                  value={formData.title}
                  onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                  style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    สถานที่ / ห้องประชุม
                  </label>
                  <input
                    type="text"
                    placeholder="เช่น ห้องประชุมใหญ่ 1"
                    value={formData.location}
                    onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    ผูกกับเครื่อง Kiosk
                  </label>
                  <select
                    value={formData.device_id}
                    onChange={(e) => setFormData({ ...formData, device_id: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  >
                    <option value="">ไม่ระบุเครื่อง</option>
                    {devices.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name || d.id} {d.mode === 'meeting' ? '(โหมดห้องประชุม)' : ''}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    วันที่ประชุม *
                  </label>
                  <input
                    type="date"
                    required
                    value={formData.meeting_date}
                    onChange={(e) => setFormData({ ...formData, meeting_date: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    เวลาเริ่ม *
                  </label>
                  <input
                    type="time"
                    required
                    value={formData.start_time}
                    onChange={(e) => setFormData({ ...formData, start_time: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    เวลาสิ้นสุด *
                  </label>
                  <input
                    type="time"
                    required
                    value={formData.end_time}
                    onChange={(e) => setFormData({ ...formData, end_time: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  />
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    เกณฑ์อนุโลมสาย (นาที)
                  </label>
                  <input
                    type="number"
                    min="0"
                    max="120"
                    value={formData.late_threshold_minutes}
                    onChange={(e) => setFormData({ ...formData, late_threshold_minutes: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                    สถานะการประชุม
                  </label>
                  <select
                    value={formData.status}
                    onChange={(e) => setFormData({ ...formData, status: e.target.value })}
                    style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                  >
                    <option value="scheduled">กำหนดการ</option>
                    <option value="ongoing">กำลังประชุม</option>
                    <option value="completed">เสร็จสิ้น</option>
                    <option value="cancelled">ยกเลิก</option>
                  </select>
                </div>
              </div>

              <div style={{ marginBottom: '1.25rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, color: '#334155', marginBottom: '0.35rem' }}>
                  รายละเอียด / วาระการประชุม
                </label>
                <textarea
                  rows="2"
                  placeholder="รายละเอียดเพิ่มเติมเกี่ยวกับการประชุม..."
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  style={{ width: '100%', padding: '0.5rem 0.75rem', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '0.9rem', boxSizing: 'border-box' }}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', borderTop: '1px solid #e2e8f0', paddingTop: '1rem' }}>
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  style={{ padding: '0.5rem 1rem', borderRadius: '6px', border: '1px solid #cbd5e1', background: '#f8fafc', color: '#475569', fontSize: '0.85rem', fontWeight: 600, cursor: 'pointer' }}
                >
                  ยกเลิก
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending || updateMutation.isPending}
                  style={{ padding: '0.5rem 1.25rem', borderRadius: '6px', border: 'none', background: '#2563eb', color: '#fff', fontSize: '0.85rem', fontWeight: 600, cursor: 'pointer' }}
                >
                  {createMutation.isPending || updateMutation.isPending ? 'กำลังบันทึก...' : 'บันทึกข้อมูล'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ----------------- MODAL: IMAGE PREVIEW ----------------- */}
      {previewImage && (
        <div
          onClick={() => setPreviewImage(null)}
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.7)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1100,
            padding: '1rem',
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: '#fff',
              borderRadius: '8px',
              padding: '1rem',
              maxWidth: '85vw',
              maxHeight: '85vh',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              boxShadow: '0 10px 25px rgba(0,0,0,0.3)',
            }}
          >
            <img
              src={previewImage.url}
              alt="ภาพสแกนขยายใหญ่"
              style={{ maxWidth: '80vw', maxHeight: '70vh', borderRadius: '6px', objectFit: 'contain' }}
            />
            <div style={{ marginTop: '0.65rem', fontSize: '0.88rem', fontWeight: 600, color: '#334155', textAlign: 'center' }}>
              {previewImage.title}
            </div>
            <button
              onClick={() => setPreviewImage(null)}
              style={{ marginTop: '0.75rem', padding: '0.35rem 1rem', borderRadius: '4px', border: '1px solid #cbd5e1', background: '#f8fafc', cursor: 'pointer', fontSize: '0.82rem' }}
            >
              ปิดหน้าต่าง
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
