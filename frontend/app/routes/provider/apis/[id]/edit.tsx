import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkInputClass = 'w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded';

interface ProvidedAPIDetail {
  id: string;
  name: string;
  description: string;
  documentation?: string;
}

export default function ProviderAPIEdit() {
  const navigate = useNavigate();
  const params = useParams();
  const id = params.id ?? '';
  const [api, setApi] = useState<ProvidedAPIDetail | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const errorId = 'provider-api-edit-error';

  useEffect(() => {
    if (!id) {
      setLoadError('Not Found');
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(`${API_BASE}/provider/apis`, { credentials: 'include' });
        if (!res.ok) {
          if (!cancelled) setLoadError('Failed to load');
          return;
        }
        const list = (await res.json()) as ProvidedAPIDetail[];
        const found = list.find(a => a.id === id);
        if (!found) {
          if (!cancelled) setLoadError('Not Found');
          return;
        }
        if (!cancelled) setApi(found);
      } catch {
        if (!cancelled) setLoadError('通信エラー');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const formData = new FormData(e.currentTarget);
      const name = String(formData.get('name') ?? '');
      const description = String(formData.get('description') ?? '');
      const documentationRaw = String(formData.get('documentation') ?? '');
      const documentation = documentationRaw || undefined;
      const res = await fetch(`${API_BASE}/provided-apis/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name, description, documentation }),
      });
      if (!res.ok) {
        const text = await res.text().catch(() => '');
        setError(text || '更新に失敗しました');
        return;
      }
      navigate('/provider/apis');
    } catch {
      setError('通信エラー');
    } finally {
      setSubmitting(false);
    }
  };

  if (loadError) {
    return (
      <main className="max-w-[600px] mx-auto my-10 p-6 text-white">
        <p role="alert" className="text-red-600">{loadError}</p>
      </main>
    );
  }

  if (!api) {
    return (
      <main className="max-w-[600px] mx-auto my-10 p-6 text-white">
        <p>読み込み中...</p>
      </main>
    );
  }

  return (
    <main className="max-w-[600px] mx-auto my-10 p-6 text-white">
      <h2>APIサービス編集</h2>
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="provider-api-edit-name">API名</label>
          <input
            id="provider-api-edit-name"
            name="name"
            type="text"
            defaultValue={api.name}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-api-edit-description">説明</label>
          <textarea
            id="provider-api-edit-description"
            name="description"
            defaultValue={api.description}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-api-edit-documentation">ドキュメント（任意）</label>
          <p className="text-xs text-gray-500 mt-1 mb-2">APIの使い方・エンドポイント・パラメータなど詳細情報を記載できます</p>
          <textarea
            id="provider-api-edit-documentation"
            name="documentation"
            defaultValue={api.documentation ?? ''}
            rows={10}
            placeholder={'## 概要\n\nAPIの説明...\n\n## エンドポイント\n\nPOST /v1/example\n\n## パラメータ\n\n- param1: 説明'}
            className="w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded font-mono text-[13px] resize-y"
          />
        </div>
        <button
          type="submit"
          disabled={submitting}
          aria-busy={submitting}
          className="p-2.5 bg-blue-600 text-white border-none rounded"
        >
          {submitting ? '更新中...' : '更新'}
        </button>
      </form>
      {error && <p id={errorId} role="alert" className="text-red-600 mt-4">{error}</p>}
    </main>
  );
}
