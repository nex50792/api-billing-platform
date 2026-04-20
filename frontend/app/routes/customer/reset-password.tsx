import { useEffect } from 'react';
import { Form, useActionData, useNavigate, useNavigation, useSearchParams } from 'react-router';
import type { Route } from './+types/reset-password';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const lightInputClass = 'w-full p-2 text-base border border-gray-300 rounded';

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const token = String(formData.get('token') ?? '');
  const password = String(formData.get('password') ?? '');
  const confirm = String(formData.get('confirm') ?? '');
  if (password !== confirm) {
    return { error: 'パスワードが一致しません。' };
  }
  if (password.length < 8) {
    return { error: 'パスワードは8文字以上で入力してください。' };
  }
  const res = await fetch(`${API_BASE}/customer/reset-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: request.headers.get('Cookie') ?? '',
    },
    body: JSON.stringify({ token, password }),
  });
  if (res.ok) {
    return { ok: true as const };
  }
  const text = await res.text().catch(() => '');
  if (text.includes('expired')) {
    return { error: 'リンクの有効期限が切れています。再度リセットを申請してください。' };
  }
  if (text.includes('already used')) {
    return { error: 'このリンクはすでに使用済みです。' };
  }
  return { error: 'リセットに失敗しました。リンクが無効か期限切れです。' };
}

export default function ResetPassword() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const token = searchParams.get('token') ?? '';
  const submitting = navigation.state === 'submitting';

  useEffect(() => {
    if (actionData && 'ok' in actionData && actionData.ok) {
      alert('パスワードをリセットしました。ログインしてください。');
      navigate('/login');
    }
  }, [actionData, navigate]);

  const error = (actionData && 'error' in actionData ? actionData.error : null) ?? (!token ? '無効なリンクです。' : null);
  const errorId = 'reset-password-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-6 text-gray-900 font-bold">パスワードリセット</h2>
      {!token ? (
        <p className="text-red-600">無効なリンクです。</p>
      ) : (
        <Form method="post">
          <input type="hidden" name="token" value={token} />
          <div className="mb-4">
            <label htmlFor="reset-password-new" className="block mb-2">新しいパスワード（8文字以上）</label>
            <input
              id="reset-password-new"
              name="password"
              type="password"
              required
              minLength={8}
              aria-required="true"
              aria-invalid={!!error}
              aria-describedby={error ? errorId : undefined}
              className={lightInputClass}
            />
          </div>
          <div className="mb-4">
            <label htmlFor="reset-password-confirm" className="block mb-2">パスワード（確認）</label>
            <input
              id="reset-password-confirm"
              name="confirm"
              type="password"
              required
              aria-required="true"
              aria-invalid={!!error}
              aria-describedby={error ? errorId : undefined}
              className={lightInputClass}
            />
          </div>
          <button
            type="submit"
            disabled={submitting}
            aria-busy={submitting}
            className={`w-full p-3 text-base bg-blue-600 text-white border-none rounded font-semibold ${submitting ? 'cursor-not-allowed' : 'cursor-pointer'}`}
          >
            {submitting ? '処理中...' : 'パスワードを変更する'}
          </button>
        </Form>
      )}
      {error && <p id={errorId} role="alert" className="mt-4 text-red-600 font-medium">{error}</p>}
    </main>
  );
}
