import { useEffect } from 'react';
import { Form, useActionData, useLoaderData, useNavigation } from 'react-router';
import type { Route } from './+types/set-password';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const token = url.searchParams.get('token');
  if (!token) {
    return { valid: false as const, token: '' };
  }
  try {
    const res = await fetch(`${API_BASE}/provider/validate-token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    return { valid: res.ok, token };
  } catch {
    return { valid: false as const, token };
  }
}

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const token = String(formData.get('token') ?? '');
  const password = String(formData.get('password') ?? '');
  const res = await fetch(`${API_BASE}/provider/set-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, password }),
  });
  if (res.ok) {
    return { ok: true as const };
  }
  return { error: 'パスワード設定に失敗しました。' };
}

export default function ProviderSetPassword() {
  const { valid, token } = useLoaderData<typeof loader>();
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  useEffect(() => {
    if (actionData && 'ok' in actionData && actionData.ok) {
      const t = setTimeout(() => {
        window.location.href = '/provider/login';
      }, 1200);
      return () => clearTimeout(t);
    }
  }, [actionData]);

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const success = actionData && 'ok' in actionData && actionData.ok;
  const errorId = 'set-password-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-6 text-gray-900 font-bold">パスワード設定</h2>
      {success ? (
        <p role="status" className="font-medium text-green-600">パスワードが設定されました。ログイン画面へ移動します。</p>
      ) : valid ? (
        <>
          <Form method="post">
            <input type="hidden" name="token" value={token} />
            <div className="mb-4">
              <label htmlFor="set-password-new" className="block mb-2 text-gray-900 font-medium">新しいパスワード</label>
              <input
                id="set-password-new"
                type="password"
                name="password"
                required
                aria-required="true"
                aria-invalid={!!error}
                aria-describedby={error ? errorId : undefined}
                className="w-full p-2 text-base border border-gray-300 rounded text-gray-900 bg-gray-50"
              />
            </div>
            <button
              type="submit"
              disabled={submitting}
              aria-busy={submitting}
              className={`w-full p-3 text-base bg-blue-600 text-white border-none rounded font-semibold ${submitting ? 'cursor-not-allowed' : 'cursor-pointer'}`}
            >
              {submitting ? '設定中...' : '設定する'}
            </button>
          </Form>
          {error && <p id={errorId} role="alert" className="mt-4 font-medium text-red-600">{error}</p>}
        </>
      ) : (
        <p className="text-red-600 font-medium">トークンが不正または期限切れです。</p>
      )}
    </main>
  );
}
