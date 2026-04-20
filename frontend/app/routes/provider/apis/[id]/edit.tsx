import { Form, redirect, useActionData, useLoaderData, useNavigation } from 'react-router';
import type { Route } from './+types/edit';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const darkInputClass = 'w-full p-2 bg-neutral-700 text-white border border-gray-500 rounded';

interface ProvidedAPIDetail {
  id: string;
  name: string;
  description: string;
  documentation?: string;
}

export async function loader({ params, request }: Route.LoaderArgs) {
  const id = params.id ?? '';
  if (!id) {
    throw new Response('Not Found', { status: 404 });
  }
  const res = await fetch(`${API_BASE}/provider/apis`, {
    headers: { Cookie: request.headers.get('Cookie') ?? '' },
  });
  if (!res.ok) {
    throw new Response('Failed to load', { status: 500 });
  }
  const list = (await res.json()) as ProvidedAPIDetail[];
  const api = list.find(a => a.id === id);
  if (!api) {
    throw new Response('Not Found', { status: 404 });
  }
  return { api };
}

export async function action({ params, request }: Route.ActionArgs) {
  const id = params.id ?? '';
  const formData = await request.formData();
  const name = String(formData.get('name') ?? '');
  const description = String(formData.get('description') ?? '');
  const documentationRaw = String(formData.get('documentation') ?? '');
  const documentation = documentationRaw || undefined;
  const res = await fetch(`${API_BASE}/provided-apis/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Cookie: request.headers.get('Cookie') ?? '',
    },
    body: JSON.stringify({ name, description, documentation }),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    return { error: text || '更新に失敗しました' };
  }
  return redirect('/provider/apis');
}

export default function ProviderAPIEdit() {
  const { api } = useLoaderData<typeof loader>();
  const actionData = useActionData<typeof action>();
  const navigation = useNavigation();
  const submitting = navigation.state === 'submitting';

  const error = actionData && 'error' in actionData ? actionData.error : null;
  const errorId = 'provider-api-edit-error';

  return (
    <main className="max-w-[600px] mx-auto my-10 p-6 text-white">
      <h2>APIサービス編集</h2>
      <Form method="post">
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
      </Form>
      {error && <p id={errorId} role="alert" className="text-red-600 mt-4">{error}</p>}
    </main>
  );
}
