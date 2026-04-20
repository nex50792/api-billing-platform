import { Form, useActionData, useNavigation } from 'react-router';
import type { Route } from './+types/forgot-password';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const email = String(formData.get('email') ?? '');
  try {
    const res = await fetch(`${API_BASE_URL}/provider/forgot-password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      return { error: text || 'エラーが発生しました' };
    }
    return { success: 'リセットリンクをメールで送信しました（登録済みの場合）' };
  } catch {
    return { error: '通信エラーが発生しました' };
  }
}

export default function ProviderForgotPassword() {
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const success = actionData && 'success' in actionData ? actionData.success : null;
  const errorId = 'provider-forgot-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-2 text-gray-900 font-bold">パスワードをお忘れの方</h2>
      <p className="text-sm text-gray-600 mb-6">
        登録済みのメールアドレスを入力してください。パスワード再設定用のリンクをお送りします。
      </p>
      {success ? (
        <div role="status" className="p-4 bg-green-50 border border-green-300 rounded-md text-green-800 text-sm">
          {success}
        </div>
      ) : (
        <Form method="post">
          <div className="mb-4">
            <label htmlFor="provider-forgot-email" className="block mb-2">メールアドレス</label>
            <input
              id="provider-forgot-email"
              name="email"
              type="email"
              required
              aria-required="true"
              aria-invalid={!!error}
              aria-describedby={error ? errorId : undefined}
              className="w-full p-2 text-base border border-gray-300 rounded box-border"
            />
          </div>
          {error && <p id={errorId} role="alert" className="mb-3 text-red-600 text-sm">{error}</p>}
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
      <div className="mt-5 text-center">
        <a href="/provider/login" className="text-gray-500 text-sm no-underline">ログインに戻る</a>
      </div>
    </main>
  );
}
