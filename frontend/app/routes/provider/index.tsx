import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import type { ProvidedAPI } from '../../lib/api';
import { apiClient } from '../../lib/api';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

interface ProviderInfo {
  id: string;
  name: string;
  email: string;
}

interface UsageRow {
  api_id: string;
  api_name: string;
  customer_id: string;
  customer_name: string;
  customer_email: string;
  usage_count: number;
}

const thClass = 'border-b border-gray-500 text-left py-2 px-1';
const thRightClass = 'border-b border-gray-500 text-right py-2 px-1';

export default function ProviderDashboard() {
  const [provider, setProvider] = useState<ProviderInfo | null>(null);
  const [apis, setApis] = useState<ProvidedAPI[]>([]);
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState<string | null>(null);
  const [expandedDoc, setExpandedDoc] = useState<string | null>(null);
  const [usageData, setUsageData] = useState<UsageRow[]>([]);
  const [showUsage, setShowUsage] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    let mounted = true;
    fetch(`${API_BASE}/provider/me`, { credentials: 'include' })
      .then(async res => {
        if (!mounted) return;
        if (!res.ok) {
          navigate('/provider/login');
          return;
        }
        setProvider((await res.json()) as ProviderInfo);
        const apisRes = await fetch(`${API_BASE}/provider/apis`, { credentials: 'include' });
        if (mounted && apisRes.ok) {
          setApis((await apisRes.json()) as ProvidedAPI[]);
        }
      })
      .catch(() => { if (mounted) navigate('/provider/login'); });
    return () => { mounted = false; };
  }, [navigate]);

  if (!provider) return null;

  const handleDelete = async (id: string) => {
    if (!window.confirm('本当に削除しますか？')) return;
    setDeleting(id);
    setError('');
    try {
      await apiClient.deleteProvidedAPI(id);
      setApis(prev => prev.filter(api => api.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : '削除に失敗しました');
    } finally {
      setDeleting(null);
    }
  };

  const handleToggleUsage = () => {
    if (!showUsage && usageData.length === 0) {
      fetch(`${API_BASE}/provider/usage`, { credentials: 'include' })
        .then(r => r.json() as Promise<UsageRow[]>)
        .then(data => setUsageData(data ?? []))
        .catch(() => setUsageData([]));
    }
    setShowUsage(prev => !prev);
  };

  const handleLogout = async () => {
    await fetch(`${API_BASE}/provider/logout`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'include' });
    navigate('/provider/login');
  };

  const totalRequests = apis.reduce((sum, a) => sum + (a.total_usage ?? 0), 0);

  return (
    <main className="max-w-[800px] mx-auto p-4 sm:p-6 text-white">
      <header className="flex flex-wrap gap-2 justify-between items-center mb-6">
        <div>
          <h1 className="m-0 text-lg sm:text-[22px] font-bold">プロバイダー管理画面</h1>
          <div className="mt-1 text-gray-300 text-sm">
            {provider.name}（{provider.email}）
          </div>
        </div>
        <div className="flex flex-wrap gap-2 items-center">
          <a href="/provider/change-password" className="py-2 px-3 text-[13px] text-slate-400 no-underline border border-slate-600 rounded">
            パスワード変更
          </a>
          <button
            onClick={handleLogout}
            className="py-2 px-4 bg-red-600 text-white border-none rounded font-semibold cursor-pointer"
          >
            ログアウト
          </button>
        </div>
      </header>

      {apis.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-5">
          <div className="bg-slate-800 rounded-lg py-4 px-5">
            <div className="text-xs text-slate-400 mb-1">合計APIリクエスト数</div>
            <div className="text-[22px] font-bold">{totalRequests.toLocaleString()}</div>
          </div>
          <div className="bg-slate-800 rounded-lg py-4 px-5">
            <div className="text-xs text-slate-400 mb-1">提供APIサービス数</div>
            <div className="text-[22px] font-bold">{apis.length}</div>
          </div>
        </div>
      )}

      <div className="flex flex-wrap gap-2 justify-between items-center mb-3">
        <h2 className="m-0 text-lg sm:text-2xl">APIサービス一覧</h2>
        <a href="/provider/apis/new" className="text-blue-600 font-semibold">+ 新規登録</a>
      </div>

      {error && <div role="alert" className="text-red-600 mb-2">{error}</div>}

      <div className="overflow-x-auto">
      <table className="w-full min-w-[640px] border-collapse">
        <thead>
          <tr>
            <th className={thClass}>API名</th>
            <th className={thClass}>説明</th>
            <th className={thRightClass}>発行キー数</th>
            <th className={thRightClass}>利用回数</th>
            <th className="border-b border-gray-500 py-2 px-1">操作</th>
          </tr>
        </thead>
        <tbody>
          {apis.map(api => {
            const docExpanded = !!api.documentation && expandedDoc === api.id;
            const rowBorder = docExpanded ? 'border-b-0' : 'border-b border-gray-600';
            return (
              <React.Fragment key={api.id}>
                <tr>
                  <td className={`${rowBorder} py-2 px-1`}>
                    <div>{api.name}</div>
                    {api.documentation && (
                      <button
                        type="button"
                        onClick={() => setExpandedDoc(expandedDoc === api.id ? null : api.id)}
                        className="text-[11px] text-sky-300 bg-transparent border-none cursor-pointer p-0 mt-0.5 flex items-center gap-[3px]"
                      >
                        <span className="text-[9px]">{expandedDoc === api.id ? '▼' : '▶'}</span>
                        ドキュメント
                      </button>
                    )}
                  </td>
                  <td className={`${rowBorder} py-2 px-1 text-gray-400 text-[13px]`}>{api.description}</td>
                  <td className={`${rowBorder} text-right py-2 px-1`}>{api.key_count ?? 0}</td>
                  <td className={`${rowBorder} text-right py-2 px-1`}>{(api.total_usage ?? 0).toLocaleString()}</td>
                  <td className={`${rowBorder} text-center py-2 px-1`}>
                    <a href={`/provider/apis/${api.id}/edit`} className="mr-2 text-blue-600">編集</a>
                    <button
                      className={`text-red-600 border-none bg-transparent ${deleting === api.id ? 'cursor-wait' : 'cursor-pointer'}`}
                      onClick={() => handleDelete(api.id)}
                      disabled={!!deleting}
                    >
                      {deleting === api.id ? '削除中...' : '削除'}
                    </button>
                  </td>
                </tr>
                {docExpanded && (
                  <tr>
                    <td colSpan={5} className="border-b border-gray-600 pt-0 pb-3 px-1">
                      <pre className="m-0 p-2.5 bg-slate-900 border border-[#334] rounded text-xs text-gray-300 whitespace-pre-wrap break-words max-h-[300px] overflow-y-auto">
                        {api.documentation}
                      </pre>
                    </td>
                  </tr>
                )}
              </React.Fragment>
            );
          })}
          {apis.length === 0 && (
            <tr><td colSpan={5} className="text-center text-gray-400 p-4">APIサービスがありません</td></tr>
          )}
        </tbody>
      </table>
      </div>

      <div className="mt-8">
        <div className="flex flex-wrap gap-2 justify-between items-center mb-3">
          <h2 className="m-0 text-lg sm:text-2xl">顧客別利用統計</h2>
          <button
            onClick={handleToggleUsage}
            className="py-1.5 px-3.5 bg-slate-800 text-sky-300 border border-slate-700 rounded cursor-pointer text-[13px]"
          >
            {showUsage ? '▲ 非表示' : '▼ 表示'}
          </button>
        </div>
        {showUsage && (
          <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] border-collapse text-[13px]">
            <thead>
              <tr>
                <th className={thClass}>APIサービス名</th>
                <th className={thClass}>顧客名</th>
                <th className={thClass}>メール</th>
                <th className={thRightClass}>利用回数</th>
              </tr>
            </thead>
            <tbody>
              {usageData.map((u, i) => (
                <tr key={i}>
                  <td className="py-1.5 px-1 text-sky-300">{u.api_name}</td>
                  <td className="py-1.5 px-1">{u.customer_name}</td>
                  <td className="py-1.5 px-1 text-slate-400">{u.customer_email}</td>
                  <td className="py-1.5 px-1 text-right font-semibold">{u.usage_count.toLocaleString()}</td>
                </tr>
              ))}
              {usageData.length === 0 && (
                <tr><td colSpan={4} className="p-3 text-gray-400 text-center">利用データがありません</td></tr>
              )}
            </tbody>
          </table>
          </div>
        )}
      </div>
    </main>
  );
}
