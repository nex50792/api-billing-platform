import React, { useEffect, useState } from "react";
import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
} from "react-router";

import type { Route } from "./+types/root";
import "./app.css";

declare global {
  interface Window {
    __appErrors?: Array<{ type: string; message: string; error?: unknown; reason?: unknown }>;
  }
}

export const links: Route.LinksFunction = () => [
  { rel: "preconnect", href: "https://fonts.googleapis.com" },
  {
    rel: "preconnect",
    href: "https://fonts.gstatic.com",
    crossOrigin: "anonymous",
  },
  {
    rel: "stylesheet",
    href: "https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap",
  },
];

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ErrorLogger />
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

function ErrorLogger() {
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // expose array for quick inspection in console
    window.__appErrors = window.__appErrors || []

    const onError = (ev: ErrorEvent) => {
      const msg = `${ev.message} at ${ev.filename}:${ev.lineno}:${ev.colno}`
      console.error("[App Error]", msg, ev.error)
      window.__appErrors!.push({ type: "error", message: msg, error: ev.error })
      setError(msg)
    }

    const onRejection = (ev: PromiseRejectionEvent) => {
      const reason = ev.reason instanceof Error ? ev.reason.message : String(ev.reason)
      const msg = `UnhandledRejection: ${reason}`
      console.error("[App Rejection]", ev.reason)
      window.__appErrors!.push({ type: "rejection", message: msg, reason: ev.reason })
      setError(msg)
    }

    window.addEventListener("error", onError)
    window.addEventListener("unhandledrejection", onRejection)

    return () => {
      window.removeEventListener("error", onError)
      window.removeEventListener("unhandledrejection", onRejection)
    }
  }, [])

  if (!error) return null

  return (
    <div style={{
      position: "fixed",
      right: 12,
      bottom: 12,
      zIndex: 9999,
      background: "rgba(220,53,69,0.95)",
      color: "white",
      padding: "8px 12px",
      borderRadius: 6,
      fontSize: 13,
      boxShadow: "0 6px 18px rgba(0,0,0,0.2)",
    }}>
      <div style={{ fontWeight: 700, marginBottom: 6 }}>Client error detected</div>
      <div style={{ maxWidth: 420, whiteSpace: "pre-wrap" }}>{error}</div>
      <div style={{ marginTop: 8, display: "flex", gap: 8 }}>
        <button onClick={() => { console.log(window.__appErrors); alert('Errors logged to console') }}>Open console</button>
        <button onClick={() => setError(null)}>Dismiss</button>
      </div>
    </div>
  )
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = "エラーが発生しました";
  let details = "予期しないエラーが発生しました。時間をおいて再度お試しください。";
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404 ページが見つかりません" : `エラー ${error.status}`;
    details = error.status === 404
      ? "お探しのページは存在しないか、移動した可能性があります。"
      : error.statusText || details;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main style={{ maxWidth: 600, margin: '80px auto', padding: '0 24px', color: '#fff', fontFamily: 'system-ui, sans-serif' }}>
      <h1 style={{ fontSize: 24, fontWeight: 700, marginBottom: 12 }}>{message}</h1>
      <p style={{ color: '#aaa', marginBottom: 24 }}>{details}</p>
      <a href="/" style={{ display: 'inline-block', padding: '8px 20px', background: '#2563eb', color: '#fff', borderRadius: 4, textDecoration: 'none', fontWeight: 600 }}>
        ホームに戻る
      </a>
      {stack && (
        <pre style={{ marginTop: 24, padding: 16, background: '#1e293b', borderRadius: 6, fontSize: 12, overflowX: 'auto', color: '#f87171' }}>
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}
