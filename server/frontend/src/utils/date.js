/**
 * Thai date and time formatting utilities
 */

export function formatThaiDateTime(dateStr, fallback = '-') {
  if (!dateStr || dateStr === '-') return fallback;
  try {
    const utcStr = dateStr.endsWith('Z') || dateStr.includes('+') ? dateStr : `${dateStr}Z`;
    return new Date(utcStr).toLocaleString('th-TH');
  } catch {
    return fallback;
  }
}

export function formatThaiDate(dateStr, fallback = '-') {
  if (!dateStr || dateStr === '-') return fallback;
  try {
    const d = new Date(dateStr);
    return d.toLocaleDateString('th-TH', { year: 'numeric', month: 'short', day: 'numeric' });
  } catch {
    return dateStr || fallback;
  }
}

export function formatThaiTime(isoStr, fallback = '-') {
  if (!isoStr || isoStr === '-') return fallback;
  try {
    const utcStr = isoStr.endsWith('Z') || isoStr.includes('+') ? isoStr : `${isoStr}Z`;
    return new Date(utcStr).toLocaleTimeString('th-TH', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' น.';
  } catch {
    return isoStr || fallback;
  }
}
