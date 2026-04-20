import { useState, useEffect } from 'react';
import { useLoaderData } from 'react-router';
import type { Route } from './+types/home';
import { API_BASE_URL, apiClient, type APIKey, type BillingCheckResponse, type ProvidedAPI, type User } from '../lib/api';

export function meta(_args: Route.MetaArgs) {
  return [
    { title: 'API Billing Platform - Dashboard' },
    { name: 'description', content: 'SaaS API billing platform dashboard' },
  ];
}

export async function loader({ request }: Route.LoaderArgs) {
  const cookie = request.headers.get('Cookie') ?? '';
  const fetchJSON = async <T,>(path: string): Promise<T | null> => {
    const res = await fetch(`${API_BASE_URL}${path}`, { headers: { Cookie: cookie } });
    if (!res.ok) return null;
    return res.json() as Promise<T>;
  };
  const [me, providedAPIs, apiKeys] = await Promise.all([
    fetchJSON<User>('/customer/me'),
    fetchJSON<ProvidedAPI[]>('/provided-apis'),
    fetchJSON<APIKey[]>('/customer/apikeys'),
  ]);
  return {
    me,
    providedAPIs: providedAPIs ?? [],
    apiKeys: apiKeys ?? [],
  };
}

const loginCardClass = 'max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900';
const lightInputClass = 'w-full p-2 text-base border border-gray-300 rounded';

export default function Home() {
  const initial = useLoaderData<typeof loader>();
  const [currentUser, setCurrentUser] = useState<User | null>(initial.me);
  const [apiKeys, setApiKeys] = useState<APIKey[]>(initial.apiKeys);
  const [providedAPIs, setProvidedAPIs] = useState<ProvidedAPI[]>(initial.providedAPIs);
  // plainKeys: セッション内で生成したキーの平文 (id -> plain_key)。Billing確認用のみ。
  const [plainKeys, setPlainKeys] = useState<Record<string, string>>({});
  const [issuedKeyModal, setIssuedKeyModal] = useState<{ plainKey: string; apiName: string } | null>(null);
  const [billingResults, setBillingResults] = useState<Record<string, BillingCheckResponse>>({});
  const [loading, setLoading] = useState(false);
  const [upgrading, setUpgrading] = useState(false);
  const [error, setError] = useState<string>('');
  const [success, setSuccess] = useState<string>('');
  const [issuingFor, setIssuingFor] = useState<string | null>(null);
  const [expandedDocAPI, setExpandedDocAPI] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('checkout') === 'success') {
      setSuccess('プランのアップグレードが完了しました！');
      window.history.replaceState({}, '', '/');
      setTimeout(() => setSuccess(''), 5000);
    } else if (params.get('checkout') === 'cancel') {
      setError('チェックアウトがキャンセルされました');
      window.history.replaceState({}, '', '/');
      setTimeout(() => setError(''), 5000);
    }
  }, []);

  const handleDeleteApiKey = async (id: string) => {
    if (!window.confirm('このAPIキーを削除しますか？')) return;
    setError('');
    try {
      await apiClient.deleteAPIKey(id);
      setApiKeys(prev => prev.filter(k => k.id !== id));
      setPlainKeys(prev => { const next = { ...prev }; delete next[id]; return next; });
      setBillingResults(prev => { const next = { ...prev }; delete next[id]; return next; });
      setSuccess('APIキーを削除しました');
      setTimeout(() => setSuccess(''), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : '削除に失敗しました');
    }
  };

  const handleIssueApiKey = async (apiId: string) => {
    setIssuingFor(apiId);
    setError('');
    try {
      const result = await apiClient.issueAPIKey(apiId);
      const newKey: APIKey = { id: result.id, customer_id: currentUser?.id ?? '', usage_count: 0, usage_reset_at: '', created_at: new Date().toISOString(), provided_api_id: apiId };
      setApiKeys([newKey, ...apiKeys]);
      setPlainKeys(prev => ({ ...prev, [result.id]: result.plain_key }));
      setIssuedKeyModal({ plainKey: result.plain_key, apiName: providedAPIs.find(a => a.id === apiId)?.name ?? '' });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate API key');
    } finally {
      setIssuingFor(null);
    }
  };

  const handleLogout = async () => {
    try {
      await apiClient.logout();
    } finally {
      setCurrentUser(null);
      setApiKeys([]);
      setPlainKeys({});
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    const form = e.target as HTMLFormElement;
    const formData = new FormData(form);
    const email = (formData.get('email') as string) || '';
    const password = (formData.get('password') as string) || '';
    setLoading(true);
    setError('');
    try {
      await apiClient.login(email, password);
      const [me, fetchedAPIs, fetchedKeys] = await Promise.all([
        apiClient.me(),
        apiClient.getProvidedAPIs(),
        apiClient.getCustomerAPIKeys(),
      ]);
      setCurrentUser(me as User | null);
      setProvidedAPIs(fetchedAPIs ?? []);
      setApiKeys(fetchedKeys ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleUpgrade = async () => {
    setUpgrading(true);
    setError('');
    try {
      const res = await apiClient.createCheckoutSession();
      window.location.href = res.checkout_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start checkout');
    } finally {
      setUpgrading(false);
    }
  };

  const handleManageSubscription = async () => {
    setError('');
    try {
      const res = await apiClient.createBillingPortalSession();
      window.location.href = res.portal_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to open billing portal');
    }
  };

  const handleCheckBilling = async (keyId: string, plainKey: string) => {
    setLoading(true);
    setError('');
    try {
      const res = await apiClient.checkBilling(plainKey);
      setBillingResults(prev => ({ ...prev, [keyId]: res }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to check billing');
    } finally {
      setLoading(false);
    }
  };

  if (!currentUser) {
    return (
      <main className={loginCardClass}>
        <h2 className="mb-6 text-gray-900 font-bold">ログイン</h2>
        <form onSubmit={handleLogin}>
          <div className="mb-4">
            <label htmlFor="home-login-email" className="block mb-2">メールアドレス</label>
            <input
              id="home-login-email"
              name="email"
              type="email"
              placeholder="email@example.com"
              className={lightInputClass}
              required
              aria-required="true"
              aria-invalid={!!error}
              aria-describedby={error ? 'home-login-error' : undefined}
            />
          </div>
          <div className="mb-4">
            <label htmlFor="home-login-password" className="block mb-2">パスワード</label>
            <input
              id="home-login-password"
              name="password"
              type="password"
              placeholder="パスワード"
              className={lightInputClass}
              required
              aria-required="true"
              aria-invalid={!!error}
              aria-describedby={error ? 'home-login-error' : undefined}
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            aria-busy={loading}
            className={`w-full p-3 text-base bg-blue-600 text-white border-none rounded font-semibold ${loading ? 'cursor-not-allowed' : 'cursor-pointer'}`}
          >
            {loading ? 'ログイン中...' : 'ログイン'}
          </button>
        </form>
        {error && <p id="home-login-error" role="alert" className="mt-4 text-red-600 font-medium">{error}</p>}
        <div className="mt-6 text-center flex gap-3 justify-center">
          <button
            type="button"
            className="py-2 px-6 text-[15px] bg-emerald-600 text-white border-none rounded font-semibold cursor-pointer whitespace-nowrap"
            onClick={() => window.location.href = '/register'}
          >
            新規登録はこちら
          </button>
          <button
            type="button"
            className="py-2 px-6 text-[15px] bg-indigo-500 text-white border-none rounded font-semibold cursor-pointer whitespace-nowrap"
            onClick={() => window.location.href = '/provider/login'}
          >
            プロバイダー向け
          </button>
        </div>
        <div className="mt-3">
          <a href="/forgot-password" className="text-gray-500 text-sm no-underline">パスワードをお忘れの方はこちら</a>
        </div>
      </main>
    );
  }

  const isPro = currentUser.plan === 'pro';
  const planLimit = isPro ? 10000 : 100;
  const totalUsage = apiKeys.reduce((sum, k) => sum + (k.usage_count ?? 0), 0);
  const usagePct = Math.min(100, Math.round((totalUsage / planLimit) * 100));
  const usageTextClass = usagePct >= 90 ? 'text-red-400' : usagePct >= 70 ? 'text-amber-400' : 'text-green-400';
  const usageBarClass = usagePct >= 90 ? 'bg-red-500' : usagePct >= 70 ? 'bg-amber-500' : 'bg-green-500';

  return (
    <div className="max-w-[1200px] mx-auto p-5">
      {issuedKeyModal && (
        <div className="fixed inset-0 bg-black/75 flex items-center justify-center z-[1000]">
          <div className="bg-slate-800 rounded-lg p-7 max-w-[520px] w-[90%] border border-slate-600">
            <h2 className="mt-0 mb-3 text-white text-lg">APIキーが発行されました</h2>
            <div className="bg-[#431407] border border-red-600 rounded-md py-2.5 px-3.5 mb-4 text-red-300 text-[13px] leading-relaxed">
              ⚠️ このキーは<strong>この画面でのみ</strong>表示されます。今すぐコピーして安全な場所に保管してください。閉じると二度と確認できません。
            </div>
            {issuedKeyModal.apiName && (
              <div className="mb-2.5 text-[13px] text-slate-400">APIサービス: <strong className="text-sky-300">{issuedKeyModal.apiName}</strong></div>
            )}
            <div className="flex items-stretch gap-2 mb-5">
              <code className="flex-1 bg-slate-900 border border-slate-700 rounded-md py-2.5 px-3 text-green-400 font-mono text-[13px] break-all leading-relaxed">
                {issuedKeyModal.plainKey}
              </code>
              <button
                onClick={() => { navigator.clipboard.writeText(issuedKeyModal.plainKey); setSuccess('コピーしました'); setTimeout(() => setSuccess(''), 2000); }}
                className="px-4 bg-blue-600 text-white border-none rounded-md cursor-pointer font-bold text-sm whitespace-nowrap"
              >コピー</button>
            </div>
            <button
              onClick={() => setIssuedKeyModal(null)}
              className="w-full py-2.5 bg-gray-700 text-gray-300 border-none rounded-md cursor-pointer font-semibold text-sm"
            >
              保存しました・閉じる
            </button>
          </div>
        </div>
      )}
      <div className="flex justify-between items-center mb-2">
        <h1 className="m-0">API Billing Platform Dashboard</h1>
        <button
          onClick={handleLogout}
          className="py-2 px-4 bg-red-600 text-white border-none rounded cursor-pointer"
        >
          ログアウト
        </button>
      </div>

      {error && (
        <div className="p-2.5 bg-[#5a1a1a] border border-[#a33] rounded mb-2.5 text-[#ffcccc]">
          {error}
        </div>
      )}
      {success && (
        <div className="p-2.5 bg-[#1a3a2a] border border-[#3a7a4a] rounded mb-2.5 text-[#aaffcc]">
          {success}
        </div>
      )}

      <div className="grid grid-cols-[1fr_2fr] gap-5">
        <div className="border border-gray-600 p-4 rounded-lg">
          <h2>アカウント</h2>
          <div className="mb-3">
            <div className="text-gray-400 text-[13px]">メールアドレス</div>
            <div className="text-white">{currentUser.email}</div>
          </div>
          <div className="mb-3">
            <a href="/change-password" className="text-[13px] text-blue-400 no-underline">パスワードを変更する</a>
          </div>
          <div className="mb-4">
            <div className="text-gray-400 text-[13px]">現在のプラン</div>
            <div className="flex items-center gap-2 mt-1">
              <span className={`py-0.5 px-2.5 text-[13px] rounded text-white font-semibold ${isPro ? 'bg-emerald-600' : 'bg-gray-500'}`}>
                {isPro ? 'Pro' : 'Free'}
              </span>
              {currentUser.subscription_status === 'canceling' && (
                <span className="text-xs text-red-400">（期間終了後にキャンセル予定）</span>
              )}
              {currentUser.subscription_status && currentUser.subscription_status !== 'canceling' && currentUser.subscription_status !== 'active' && (
                <span className="text-xs text-gray-400">({currentUser.subscription_status})</span>
              )}
            </div>
          </div>

          <div className="mb-4">
            <div className="text-gray-400 text-[13px] mb-1">今月の合計リクエスト数</div>
            <div className={`text-[15px] font-semibold ${usageTextClass}`}>
              {totalUsage.toLocaleString()} <span className="text-xs text-gray-500 font-normal">/ {planLimit.toLocaleString()} 回</span>
            </div>
            <div className="mt-1.5 h-1.5 bg-gray-700 rounded-sm overflow-hidden">
              <div
                className={`h-full rounded-sm transition-[width] duration-300 ${usageBarClass}`}
                style={{ width: `${usagePct}%` }}
              />
            </div>
          </div>

          {!isPro && (
            <button
              onClick={handleUpgrade}
              disabled={upgrading}
              className={`w-full p-2.5 bg-violet-600 text-white border-none rounded font-semibold mb-4 ${upgrading ? 'cursor-wait' : 'cursor-pointer'}`}
            >
              {upgrading ? '...' : 'Proにアップグレード'}
            </button>
          )}
          {isPro && (
            <button
              onClick={handleManageSubscription}
              className="w-full p-2.5 bg-transparent text-gray-400 border border-gray-500 rounded font-semibold cursor-pointer mb-4"
            >
              サブスクを管理・キャンセル
            </button>
          )}

          <div className="border-t border-gray-600 pt-4">
            <h3 className="mb-2.5 text-sm">プラン比較</h3>
            <table className="w-full border-collapse text-xs">
              <thead>
                <tr>
                  <th className="text-left py-1 px-1.5 text-gray-400"></th>
                  <th className="text-center py-1 px-1.5 text-gray-400">Free</th>
                  <th className="text-center py-1 px-1.5 text-emerald-600">Pro</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td className="py-1 px-1.5 text-gray-300">月額料金</td>
                  <td className="text-center py-1 px-1.5">¥0</td>
                  <td className="text-center py-1 px-1.5 text-emerald-600 font-semibold">¥1,000</td>
                </tr>
                <tr>
                  <td className="py-1 px-1.5 text-gray-300">月間リクエスト数</td>
                  <td className="text-center py-1 px-1.5">100回</td>
                  <td className="text-center py-1 px-1.5 text-emerald-600 font-semibold">10,000回</td>
                </tr>
                <tr>
                  <td className="py-1 px-1.5 text-gray-300">上限超過時</td>
                  <td className="text-center py-1 px-1.5 text-red-400">429エラー</td>
                  <td className="text-center py-1 px-1.5 text-red-400">429エラー</td>
                </tr>
                <tr>
                  <td className="py-1 px-1.5 text-gray-300">リセット周期</td>
                  <td className="text-center py-1 px-1.5">毎月1日</td>
                  <td className="text-center py-1 px-1.5">毎月1日</td>
                </tr>
              </tbody>
            </table>
            <p className="mt-2 text-[11px] text-gray-500">
              ※ リクエスト数はアカウント内の全APIキー合計
            </p>
          </div>
        </div>

        <div className="border border-gray-600 p-4 rounded-lg">
          <h2>API Keys</h2>
          {apiKeys.length === 0 && (
            <div className="text-gray-500 text-[13px] mb-2">下の「利用可能なAPIサービス」からキーを発行できます</div>
          )}

          {apiKeys.length > 0 && (
            <div className="mt-4">
              <h3>Generated Keys</h3>
              {apiKeys.map((key) => {
                const api = providedAPIs.find(a => a.id === key.provided_api_id);
                const plainKey = plainKeys[key.id];
                const result = billingResults[key.id];
                return (
                  <div
                    key={key.id}
                    className="py-2 px-2.5 bg-neutral-800 border border-gray-600 rounded mb-1.5 text-xs text-white"
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        {api && <div className="text-xs text-sky-300 font-semibold">{api.name}</div>}
                        <div className="text-gray-400 mt-0.5">
                          利用回数: {key.usage_count}
                          {key.usage_reset_at && <span className="ml-2 text-[11px] text-slate-500">リセット: {new Date(key.usage_reset_at).toLocaleDateString('ja-JP')}</span>}
                        </div>
                      </div>
                      <div className="flex gap-1.5 shrink-0">
                        {plainKey && (
                          <button
                            onClick={() => handleCheckBilling(key.id, plainKey)}
                            disabled={loading}
                            className={`text-[11px] py-0.5 px-2 bg-cyan-700 border-none text-white rounded-sm ${loading ? 'cursor-not-allowed' : 'cursor-pointer'}`}
                          >利用確認</button>
                        )}
                        <button
                          onClick={() => handleDeleteApiKey(key.id)}
                          className="text-[11px] py-0.5 px-2 bg-transparent border border-red-600 text-red-600 rounded-sm cursor-pointer"
                        >削除</button>
                      </div>
                    </div>
                    {result && (
                      <div className="mt-1.5 py-1.5 px-2 bg-[#1e2a1e] rounded text-[11px] text-gray-300 border border-[#3a6a3a]">
                        <span className="mr-3"><strong>Plan:</strong> {result.customer?.plan}</span>
                        <span className="mr-3"><strong>Usage:</strong> {result.api_key?.usage_count}</span>
                        {result.provided_api && <span><strong>Service:</strong> {result.provided_api.name}</span>}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {providedAPIs.length > 0 && (
        <div className="mt-6 border border-gray-600 rounded-lg p-4">
          <h2 className="mt-0 mb-3">利用可能なAPIサービス</h2>
          <div className="flex flex-col gap-2">
            {providedAPIs.map(api => {
              const isExpanded = expandedDocAPI === api.id;
              const issuedKeys = apiKeys.filter(k => k.provided_api_id === api.id);
              const hasKey = issuedKeys.length > 0;
              return (
                <div key={api.id} className={`rounded-md overflow-hidden border ${hasKey ? 'border-[#1d4e2a]' : 'border-neutral-700'}`}>
                  <div className={`flex items-center justify-between py-2.5 px-3.5 gap-2 ${hasKey ? 'bg-[#0f2318]' : 'bg-neutral-900'}`}>
                    <div className="flex-1 min-w-0">
                      <span className="font-semibold">{api.name}</span>
                      <span className="ml-2.5 text-[13px] text-gray-400">{api.description}</span>
                      {hasKey && (
                        <span className="ml-2.5 text-[11px] text-green-400 bg-[#052e16] border border-green-800 rounded-sm py-px px-1.5">
                          キー発行済み {issuedKeys.length > 1 ? `(${issuedKeys.length}件)` : ''}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      {api.documentation && (
                        <button
                          type="button"
                          onClick={() => setExpandedDocAPI(isExpanded ? null : api.id)}
                          className="text-xs text-sky-300 bg-transparent border border-[#334] rounded cursor-pointer py-[3px] px-2.5 flex items-center gap-1"
                        >
                          <span className="text-[10px]">{isExpanded ? '▼' : '▶'}</span>
                          ドキュメント
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={() => handleIssueApiKey(api.id)}
                        disabled={issuingFor !== null}
                        className={`text-xs text-white rounded py-1 px-3 font-semibold ${issuingFor === api.id ? 'bg-[#1a5c30]' : hasKey ? 'bg-gray-700' : 'bg-green-600'} ${hasKey ? 'border border-gray-600' : 'border-none'} ${issuingFor !== null ? 'cursor-not-allowed' : 'cursor-pointer'}`}
                      >
                        {issuingFor === api.id ? '発行中...' : hasKey ? '追加発行' : 'キーを発行'}
                      </button>
                    </div>
                  </div>
                  {api.documentation && isExpanded && (
                    <pre className="m-0 py-3 px-3.5 bg-slate-900 text-[13px] text-gray-300 whitespace-pre-wrap break-words max-h-[400px] overflow-y-auto border-t border-[#334]">
                      {api.documentation}
                    </pre>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
