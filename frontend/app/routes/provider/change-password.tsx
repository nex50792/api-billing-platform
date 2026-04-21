import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkPwInputClass = 'w-full p-2 text-[15px] border border-slate-600 rounded bg-slate-900 text-white box-border';
const labelClass = 'block mb-1.5 text-sm text-slate-300';

export default function ProviderChangePassword() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const errorId = 'provider-change-password-error';

  useEffect(() => {
    if (success) {
      const t = setTimeout(() => navigate('/provider'), 2000);
      return () => clearTimeout(t);
    }
  }, [success, navigate]);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);
    const formData = new FormData(e.currentTarget);
    const currentPassword = String(formData.get('currentPassword') ?? '');
    const newPassword = String(formData.get('newPassword') ?? '');
    const confirmPassword = String(formData.get('confirmPassword') ?? '');
    if (newPassword !== confirmPassword) {
      setError('新しいパスワードが一致しません');
      return;
    }
    if (newPassword.length < 8) {
      setError('パスワードは8文字以上で設定してください');
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch(`${API_BASE}/provider/change-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      if (!res.ok) {
        const text = await res.text().catch(() => '');
        setError(text || 'パスワードの変更に失敗しました');
        return;
      }
      setSuccess('パスワードを変更しました');
    } catch {
      setError('通信エラー');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-slate-800 rounded-lg shadow-lg text-white">
      <h2 className="mb-6 font-bold">パスワード変更</h2>
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="provider-change-current" className={labelClass}>現在のパスワード</label>
          <input
            id="provider-change-current"
            name="currentPassword"
            type="password"
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkPwInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-change-new" className={labelClass}>新しいパスワード（8文字以上）</label>
          <input
            id="provider-change-new"
            name="newPassword"
            type="password"
            required
            minLength={8}
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkPwInputClass}
          />
        </div>
        <div className="mb-6">
          <label htmlFor="provider-change-confirm" className={labelClass}>新しいパスワード（確認）</label>
          <input
            id="provider-change-confirm"
            name="confirmPassword"
            type="password"
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className={darkPwInputClass}
          />
        </div>
        {error && <p id={errorId} role="alert" className="mb-4 text-red-400 text-sm">{error}</p>}
        {success && <p role="status" className="mb-4 text-green-400 text-sm">{success}</p>}
        <button
          type="submit"
          disabled={submitting}
          aria-busy={submitting}
          className={`w-full p-3 text-[15px] bg-indigo-500 text-white border-none rounded font-semibold ${submitting ? 'cursor-not-allowed' : 'cursor-pointer'}`}
        >
          {submitting ? '変更中...' : 'パスワードを変更する'}
        </button>
      </form>
      <div className="mt-4 text-center">
        <a href="/provider" className="text-slate-400 text-sm no-underline">ダッシュボードに戻る</a>
      </div>
    </main>
  );
}
