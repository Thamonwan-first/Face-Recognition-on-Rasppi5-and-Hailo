import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';
import { Bar } from 'react-chartjs-2';

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend);

export default function AttendancePage() {
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  const { data: events, isLoading, error } = useQuery({
    queryKey: ['attendance', startDate, endDate],
    queryFn: () => {
      const params = new URLSearchParams();
      if (startDate) params.append('start', startDate);
      if (endDate) params.append('end', endDate);
      const query = params.toString() ? `?${params.toString()}` : '';
      return api.get(`/api/attendance${query}`);
    },
  });

  const handleExportCSV = () => {
    if (!events || events.length === 0) return;
    const headers = ['Event ID', 'User ID', 'Name', 'Device ID', 'Direction', 'Confidence', 'Timestamp'];
    const rows = events.map((e) => [
      e.event_id,
      e.user_id,
      e.user_name || '',
      e.device_id,
      e.direction,
      e.confidence ? (e.confidence * 100).toFixed(1) + '%' : '',
      e.created_at,
    ]);
    const csvContent =
      'data:text/csv;charset=utf-8,\uFEFF' +
      [headers.join(','), ...rows.map((r) => r.map((f) => `"${f}"`).join(','))].join('\n');
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', `attendance_export_${new Date().toISOString().slice(0, 10)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  // Mock bar chart aggregate data from events
  const chartData = {
    labels: ['08:00', '09:00', '10:00', '16:00', '17:00', '18:00'],
    datasets: [
      {
        label: 'จำนวนการสแกน (ครั้ง)',
        data: [15, 65, 12, 10, 58, 20],
        backgroundColor: '#0984e3',
      },
    ],
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">🕐 ประวัติการบันทึกเวลา (Attendance Logs)</h1>
        <button onClick={handleExportCSV} className="btn btn-primary" disabled={!events?.length}>
          📥 ส่งออก CSV
        </button>
      </div>

      <div style={{ display: 'flex', gap: '1rem', marginBottom: '1.5rem', background: '#fff', padding: '1rem', borderRadius: '8px' }}>
        <div>
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.2rem' }}>ตั้งแต่วันที่</label>
          <input
            type="date"
            className="input-field"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
          />
        </div>
        <div>
          <label style={{ fontSize: '0.85rem', display: 'block', marginBottom: '0.2rem' }}>ถึงวันที่</label>
          <input
            type="date"
            className="input-field"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
          />
        </div>
      </div>

      <div style={{ background: '#fff', padding: '1.25rem', borderRadius: '8px', marginBottom: '1.5rem', maxHeight: '300px' }}>
        <h3 style={{ fontSize: '1rem', marginBottom: '0.5rem' }}>สรุปความหนาแน่นช่วงเวลาการลงเวลา</h3>
        <div style={{ height: '220px' }}>
          <Bar data={chartData} options={{ responsive: true, maintainAspectRatio: false }} />
        </div>
      </div>

      {isLoading && <p>กำลังโหลดประวัติลงเวลา...</p>}
      {error && <p style={{ color: 'var(--danger)' }}>เกิดข้อผิดพลาด: {error.message}</p>}

      <div className="table-container">
        <table className="data-table">
          <thead>
            <tr>
              <th>เวลาที่เกิดเหตุการณ์</th>
              <th>ชื่อพนักงาน</th>
              <th>อุปกรณ์ (Device)</th>
              <th>ทิศทาง</th>
              <th>ความแม่นยำ (Confidence)</th>
              <th>เวลาที่ซิงก์สำเร็จ</th>
            </tr>
          </thead>
          <tbody>
            {events?.map((e) => (
              <tr key={e.event_id}>
                <td>{new Date(e.created_at).toLocaleString('th-TH')}</td>
                <td>{e.user_name || e.user_id}</td>
                <td>{e.device_id}</td>
                <td>
                  <span
                    style={{
                      fontWeight: 'bold',
                      color: e.direction === 'IN' ? 'var(--success)' : 'var(--danger)',
                    }}
                  >
                    {e.direction === 'IN' ? '⬆ เข้างาน' : '⬇ ออกงาน'}
                  </span>
                </td>
                <td>{e.confidence ? `${(e.confidence * 100).toFixed(1)}%` : '-'}</td>
                <td>{e.synced_at ? new Date(e.synced_at).toLocaleTimeString('th-TH') : '-'}</td>
              </tr>
            ))}
            {events?.length === 0 && (
              <tr>
                <td colSpan="6" style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                  ไม่พบข้อมูลประวัติลงเวลา
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
