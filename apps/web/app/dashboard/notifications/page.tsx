"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Notification } from "@/lib/types";

export default function NotificationsPage() {
  const qc = useQueryClient();
  const list = useQuery({
    queryKey: ["notifications"],
    queryFn: () =>
      api<{ notifications: Notification[] }>("/v1/notifications"),
    refetchInterval: 60_000,
  });

  const markRead = useMutation({
    mutationFn: (id: string) =>
      api(`/v1/notifications/${id}/read`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });

  return (
    <main className="container max-w-2xl py-8">
      <h1 className="mb-1 text-2xl font-bold">Notifications</h1>
      <p className="mb-6 text-sm text-muted">
        Trade proposals, waiver results, draft pings, and more.
      </p>

      <div className="space-y-2">
        {list.data?.notifications.length === 0 && (
          <div className="card text-sm text-muted">No notifications yet.</div>
        )}
        {list.data?.notifications.map((n) => (
          <div
            key={n.id}
            className={`card ${n.read_at ? "opacity-60" : "border-accent/40"}`}
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-xs uppercase text-muted">{n.type}</div>
                <div className="font-medium">{n.title}</div>
                {n.body && (
                  <p className="mt-1 text-sm text-muted">{n.body}</p>
                )}
                <div className="mt-1 text-xs text-muted">
                  {new Date(n.created_at).toLocaleString()}
                </div>
              </div>
              <div className="flex flex-col gap-1">
                {n.deep_link && (
                  <a
                    href={n.deep_link}
                    className="btn-outline text-xs"
                  >
                    Open
                  </a>
                )}
                {!n.read_at && (
                  <button
                    onClick={() => markRead.mutate(n.id)}
                    className="btn-ghost text-xs"
                  >
                    Mark read
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </main>
  );
}
