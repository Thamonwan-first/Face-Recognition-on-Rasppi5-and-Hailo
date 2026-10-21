import React from 'react';

export default function StatusBadge({ status }) {
  const getBadgeClass = (s) => {
    switch (s?.toLowerCase()) {
      case 'online':
      case 'active':
      case 'done':
        return 'badge-online';
      case 'offline':
      case 'inactive':
      case 'failed':
        return 'badge-offline';
      case 'pending':
      case 'processing':
      case 'pending_approval':
      case 'open':
        return 'badge-warning';
      default:
        return '';
    }
  };

  return <span className={`badge ${getBadgeClass(status)}`}>{status || 'N/A'}</span>;
}
