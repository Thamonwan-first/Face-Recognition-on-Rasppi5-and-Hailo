import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import { formatThaiDate, formatThaiTime } from '../utils/date';

export default function AttendancePage() {
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [searchTerm, setSearchTerm] = useState('');

  const { data: pairs, isLoading, error } = useQuery({
    queryKey: ['attendance_pairs', startDate, endDate],
    queryFn: () => {
      const params = new URLSearchParams();
      if (startDate) params.append('start', startDate);
      if (endDate) params.append('end', endDate);
      const query = params.toString() ? `?${params.toString()}` : '';
      return api.get(`/api/attendance/pairs${query}`);
    },
    refetchInterval: 5000, // อัปเดตข้อมูลอัตโนมัติทุก 5 วินาที
  });

  const filteredPairs = pairs?.filter((p) => {
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    return (
      p.name?.toLowerCase().includes(term) ||
      p.employee_id?.toLowerCase().includes(term) ||
      p.department?.toLowerCase().includes(term)
    );
  });

  const handleExportCSV = () => {
    if (!filteredPairs || filteredPairs.length === 0) return;
    const headers = [
      'วันที่',
      'รหัสพนักงาน',
      'ชื่อ-นามสกุล',
      'ฝ่าย',
      'เวลาเข้างาน (Check-in)',
      'เวลาออกงาน (Check-out)',
      'รวมชั่วโมงทำงาน',
      'สถานะ',
    ];
    const rows = filteredPairs.map((p) => [
      p.work_date,
      p.employee_id,
      p.name,
      p.department,
      formatThaiTime(p.check_in_time),
      formatThaiTime(p.check_out_time),
      p.hours_worked !== null ? `${p.hours_worked} ชม.` : '-',
      p.status === 'open' ? 'กำลังทำงาน' : 'บันทึกครบ (ปิดรอบ)',
    ]);

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

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">📅 ประวัติการบันทึกเวลาเข้า-ออกงาน (Attendance Logs)</h1>
        <button onClick={handleExportCSV} className="btn btn-primary" disabled={!filteredPairs?.length}>
          📥 ส่งออกรายงาน CSV
        </button>
      </div>

      {/* แถบตัวกรองวันที่ และค้นหาพนักงาน */}
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
        {(startDate || endDate || searchTerm) && (
          <div>
            <button
              onClick={() => {
                setStartDate('');
                setEndDate('');
                setSearchTerm('');
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

      {/* ตารางแสดงการเข้า-ออกงาน วันที่ | รหัส | ชื่อ | แผนก | Check-in | Check-out | ชั่วโมง | สถานะ */}
      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>วันที่</th>
              <th>รหัสพนักงาน</th>
              <th>ชื่อ-นามสกุล</th>
              <th>ฝ่าย</th>
              <th>เวลาเข้างาน (Check-in)</th>
              <th>เวลาออกงาน (Check-out)</th>
              <th>รวมชั่วโมง</th>
              <th>สถานะ</th>
            </tr>
          </thead>
          <tbody>
            {filteredPairs?.map((p) => (
              <tr key={p.id}>
                <td>
                  <strong>{formatThaiDate(p.work_date)}</strong>
                </td>
                <td>
                  <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{p.employee_id}</span>
                </td>
                <td>{p.name}</td>
                <td>{p.department}</td>
                <td>
                  {p.check_in_time ? (
                    <span style={{ color: 'var(--success)', fontWeight: 'bold' }}>
                      🟢 {formatThaiTime(p.check_in_time)}
                    </span>
                  ) : (
                    <span style={{ color: 'var(--text-muted)' }}>-</span>
                  )}
                </td>
                <td>
                  {p.check_out_time ? (
                    <span style={{ color: '#d63031', fontWeight: 'bold' }}>
                      🔴 {formatThaiTime(p.check_out_time)}
                    </span>
                  ) : (
                    <span style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>ยังไม่ออกงาน</span>
                  )}
                </td>
                <td>
                  {p.hours_worked !== null ? (
                    <span style={{ fontWeight: 600 }}>{p.hours_worked} ชม.</span>
                  ) : (
                    <span style={{ color: 'var(--text-muted)' }}>-</span>
                  )}
                </td>
                <td>
                  {p.status === 'open' ? (
                    <span
                      style={{
                        padding: '0.25rem 0.6rem',
                        borderRadius: '20px',
                        fontSize: '0.8rem',
                        background: '#e3f2fd',
                        color: '#1976d2',
                        fontWeight: 600,
                      }}
                    >
                      กำลังทำงาน
                    </span>
                  ) : (
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
                      บันทึกครบ (ปิดรอบ)
                    </span>
                  )}
                </td>
              </tr>
            ))}
            {filteredPairs?.length === 0 && (
              <tr>
                <td colSpan="8" style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '2rem' }}>
                  ไม่พบข้อมูลการบันทึกเวลาในช่วงวันที่เลือก
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
