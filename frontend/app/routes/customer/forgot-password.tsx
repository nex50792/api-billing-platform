import { Form, useActionData, useNavigation } from 'react-router';
import type { Route } from './+types/forgot-password';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const lightInputClass = 'w-full p-2 text-base border border-gray-300 rounded';

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const email = String(formData.get('email') ?? '');
  const res = await fetch(`${API_BASE}/customer/forgot-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: request.headers.get('Cookie') ?? '',
    },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    return { error: 'リクエストに失敗しました。もう一度お試しください。' };
  }
  const data = (await res.json().catch(() => ({}))) as { message?: string };
  return { message: data.message ?? '送信しました' };
}

export default function ForgotPassword() {
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const message = actionData && 'message' in actionData ? actionData.message : null;
  const errorId = 'forgot-password-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-6 text-gray-900 font-bold">パスワードをお忘れの方</h2>
      {message ? (
        <p role="status" className="text-emerald-600 font-medium">{message}</p>
      ) : (
        <Form method="post">
          <div className="mb-4">
            <label htmlFor="forgot-password-email" className="block mb-2">登録済みのメールアドレス</label>
            <input
              id="forgot-password-email"
              name="email"
              type="email"
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
            {submitting ? '送信中...' : 'リセットリンクを送信'}
          </button>
        </Form>
      )}
      {error && <p id={errorId} role="alert" className="mt-4 text-red-600 font-medium">{error}</p>}
      <div className="mt-6 text-center">
        <a href="/login" className="text-blue-600 text-sm no-underline">ログインに戻る</a>
      </div>
    </main>
  );
}
