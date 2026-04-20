import { Form, redirect, useActionData, useNavigation } from 'react-router';
import type { Route } from './+types/new';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkInputClass = 'w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded';

export async function action({ request }: Route.ActionArgs) {
  const formData = await request.formData();
  const name = String(formData.get('name') ?? '');
  const description = String(formData.get('description') ?? '');
  const documentationRaw = String(formData.get('documentation') ?? '');
  const documentation = documentationRaw || undefined;
  const res = await fetch(`${API_BASE}/provided-apis`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: request.headers.get('Cookie') ?? '',
    },
    body: JSON.stringify({ name, description, documentation }),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    return { error: text || '登録に失敗しました' };
  }
  return redirect('/provider/apis');
}

export default function ProviderAPINew() {
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const errorId = 'provider-api-new-error';

  return (
    <main className="max-w-[600px] mx-auto my-10 p-6 text-white">
      <h2>新規APIサービス登録</h2>
      <Form method="post">
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
      </Form>
      {error && <p id={errorId} role="alert" className="text-red-600 mt-4">{error}</p>}
    </main>
  );
}
