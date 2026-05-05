"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { useState, useEffect, useRef } from "react";
import { api, wsURL } from "@/lib/api";
import type { ChatMessage } from "@/lib/types";

export default function ChatPage() {
  const { leagueId } = useParams<{ leagueId: string }>();
  const qc = useQueryClient();
  const [body, setBody] = useState("");
  const listRef = useRef<HTMLDivElement>(null);

  const messages = useQuery({
    queryKey: ["messages", leagueId],
    queryFn: () =>
      api<{ messages: ChatMessage[] }>(
        `/v1/leagues/${leagueId}/messages?channel=main&limit=100`,
      ),
  });

  useEffect(() => {
    const ws = new WebSocket(wsURL(`/ws/league/${leagueId}`));
    ws.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        if (e.type === "chat") {
          qc.invalidateQueries({ queryKey: ["messages", leagueId] });
        }
      } catch {
        // ignore
      }
    };
    return () => ws.close();
  }, [leagueId, qc]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [messages.data]);

  const post = useMutation({
    mutationFn: (text: string) =>
      api(`/v1/leagues/${leagueId}/messages`, {
        method: "POST",
        body: JSON.stringify({ channel: "main", body: text }),
      }),
    onSuccess: () => {
      setBody("");
      qc.invalidateQueries({ queryKey: ["messages", leagueId] });
    },
  });

  return (
    <main className="container flex flex-col py-8" style={{ height: "calc(100vh - 6rem)" }}>
      <h1 className="mb-4 text-2xl font-bold">League chat</h1>
      <div
        ref={listRef}
        className="card flex-1 space-y-3 overflow-y-auto p-4"
      >
        {messages.data?.messages
          .slice()
          .reverse()
          .map((m) => (
            <div key={m.id} className="text-sm">
              <span className="mr-2 text-xs font-semibold text-accent">
                {m.display_name ?? "Anon"}
              </span>
              <span className="text-muted text-xs">
                {new Date(m.created_at).toLocaleString()}
              </span>
              <div>{m.body}</div>
            </div>
          ))}
        {messages.data?.messages.length === 0 && (
          <div className="text-center text-sm text-muted">
            Quiet in here. Start something.
          </div>
        )}
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (body.trim()) post.mutate(body);
        }}
        className="mt-3 flex gap-2"
      >
        <input
          className="input flex-1"
          placeholder="Trash talk goes here..."
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <button className="btn-primary" disabled={!body.trim() || post.isPending}>
          Send
        </button>
      </form>
    </main>
  );
}
