import { useEffect } from 'react';
import { Form, useActionData, useNavigate, useNavigation } from 'react-router';
import type { Route } from './+types/change-password';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkPwInputClass = 'w-full p-2 text-[15px] border border-slate-600 rounded bg-slate-900 text-white box-border';
const labelClass = 'block mb-1.5 text-sm text-slate-300';

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const currentPassword = String(formData.get('currentPassword') ?? '');
  const newPassword = String(formData.get('newPassword') ?? '');
  const confirmPassword = String(formData.get('confirmPassword') ?? '');
  if (newPassword !== confirmPassword) {
    return { error: '新しいパスワードが一致しません' };
  }
  if (newPassword.length < 8) {
    return { error: 'パスワードは8文字以上で設定してください' };
  }
  const res = await fetch(`${API_BASE}/provider/change-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: request.headers.get('Cookie') ?? '',
    },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    return { error: text || 'パスワードの変更に失敗しました' };
  }
  return { ok: true as const };
}

export default function ProviderChangePassword() {
  const navigate = useNavigate();
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  useEffect(() => {
    if (actionData && 'ok' in actionData && actionData.ok) {
      const t = setTimeout(() => navigate('/provider'), 2000);
      return () => clearTimeout(t);
    }
  }, [actionData, navigate]);

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const success = actionData && 'ok' in actionData && actionData.ok ? 'パスワードを変更しました' : null;
  const errorId = 'provider-change-password-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-slate-800 rounded-lg shadow-lg text-white">
      <h2 className="mb-6 font-bold">パスワード変更</h2>
      <Form method="post">
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
      </Form>
      <div className="mt-4 text-center">
        <a href="/provider" className="text-slate-400 text-sm no-underline">ダッシュボードに戻る</a>
      </div>
    </main>
  );
}
