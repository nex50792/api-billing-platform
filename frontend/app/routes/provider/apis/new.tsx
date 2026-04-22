import React, { useState } from 'react';
import { useNavigate } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkInputClass = 'w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded';

export default function ProviderAPINew() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const errorId = 'provider-api-new-error';

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
      const res = await fetch(`${API_BASE}/provided-apis`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name, description, documentation }),
      });
      if (!res.ok) {
        const text = await res.text().catch(() => '');
        setError(text || '登録に失敗しました');
        return;
      }
      navigate('/provider/apis');
    } catch {
      setError('通信エラー');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="max-w-[600px] mx-auto my-10 p-4 sm:p-6 text-white">
      <h2 className="text-lg sm:text-2xl">新規APIサービス登録</h2>
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="provider-api-new-name">API名</label>
          <input
            id="provider-api-new-name"
            name="name"
            type="text"
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-api-new-description">説明</label>
          <textarea
            id="provider-api-new-description"
            name="description"
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-api-new-documentation">ドキュメント（任意）</label>
          <p className="text-xs text-gray-500 mt-1 mb-2">APIの使い方・エンドポイント・パラメータなど詳細情報を記載できます</p>
          <textarea
            id="provider-api-new-documentation"
            name="documentation"
            rows={10}
            placeholder={'## 概要\n\nAPIの説明...\n\n## エンドポイント\n\nPOST /v1/example\n\n## パラメータ\n\n- param1: 説明'}
            className="w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded font-mono text-[13px] resize-y"
          />
        </div>
        <button
          type="submit"
          disabled={submitting}
          aria-busy={submitting}
          className="p-2.5 bg-emerald-600 text-white border-none rounded"
        >
          {submitting ? '登録中...' : '登録'}
        </button>
      </form>
      {error && <p id={errorId} role="alert" className="text-red-600 mt-4">{error}</p>}
    </main>
  );
}
