import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';

/**
 * useRealtime listens to Server-Sent Events (SSE) from the backend
 * and automatically invalidates React Query caches for instant dashboard updates
 * without periodic polling.
 */
export function useRealtime() {
  const queryClient = useQueryClient();

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) return;

    let eventSource = null;
    let reconnectTimeout = null;

    const connect = () => {
      try {
        const streamUrl = `/api/realtime/stream?token=${encodeURIComponent(token)}`;
        eventSource = new EventSource(streamUrl);

        eventSource.addEventListener('attendance', () => {
          queryClient.invalidateQueries({ queryKey: ['attendance_pairs'] });
          queryClient.invalidateQueries({ queryKey: ['attendance_summary'] });
          queryClient.invalidateQueries({ queryKey: ['attendance_events'] });
        });

        eventSource.addEventListener('device', () => {
          queryClient.invalidateQueries({ queryKey: ['devices'] });
          queryClient.invalidateQueries({ queryKey: ['devices_status'] });
        });

        eventSource.addEventListener('enrollment', () => {
          queryClient.invalidateQueries({ queryKey: ['enrollment_jobs'] });
          queryClient.invalidateQueries({ queryKey: ['pending_enrollments'] });
          queryClient.invalidateQueries({ queryKey: ['users'] });
        });

        eventSource.onerror = (err) => {
          // EventSource automatically attempts to reconnect on network drop.
          // If connection is closed (e.g., token expired or server restarted), re-initiate.
          if (eventSource.readyState === EventSource.CLOSED) {
            reconnectTimeout = setTimeout(connect, 3000);
          }
        };
      } catch (e) {
        console.error('Failed to initialize SSE connection:', e);
      }
    };

    connect();

    return () => {
      if (eventSource) {
        eventSource.close();
      }
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
      }
    };
  }, [queryClient]);
}
