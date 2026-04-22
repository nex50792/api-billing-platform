import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

interface Provider {
  id: string;
  name: string;
  email: string;
  is_active: boolean;
}

interface Customer {
  id: string;
  name: string;
  email: string;
  plan: string;
  subscription_status: string;
  is_active: boolean;
  created_at: string;
}

interface EmailLog {
  id: string;
  to_email: string;
  subject: string;
  status: string;
  error_message: string;
  sent_at: string;
}

const darkInputClass = 'p-1.5 bg-neutral-700 text-white border border-gray-500 rounded';
const thClass = 'text-left border-b border-gray-500 py-2 px-1';

export default function AdminDashboard() {
  const navigate = useNavigate();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [providerLoading, setProviderLoading] = useState(true);
  const [providerName, setProviderName] = useState('');
  const [providerEmail, setProviderEmail] = useState('');
  const [providerError, setProviderError] = useState('');
  const [providerRegistering, setProviderRegistering] = useState(false);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [customerLoading, setCustomerLoading] = useState(true);
  const [emailLogs, setEmailLogs] = useState<EmailLog[]>([]);
  const [emailLogLoading, setEmailLogLoading] = useState(true);
  const [pwCurrent, setPwCurrent] = useState('');
  const [pwNew, setPwNew] = useState('');
  const [pwConfirm, setPwConfirm] = useState('');
  const [pwError, setPwError] = useState('');
  const [pwSuccess, setPwSuccess] = useState('');
  const [pwLoading, setPwLoading] = useState(false);
  const [pwOpen, setPwOpen] = useState(false);
  const [authChecked, setAuthChecked] = useState(false);

  const fetchCustomers = (mounted = true) => {
    setCustomerLoading(true);
    fetch(`${API_BASE}/admin/customers`, { credentials: 'include' })
      .then(r => r.json() as Promise<Customer[]>)
      .then(data => { if (mounted) setCustomers(data || []); })
      .catch(() => { if (mounted) setCustomers([]); })
      .finally(() => { if (mounted) setCustomerLoading(false); });
  };

  const fetchEmailLogs = (mounted = true) => {
    setEmailLogLoading(true);
    fetch(`${API_BASE}/admin/email-logs`, { credentials: 'include' })
      .then(r => r.json() as Promise<EmailLog[]>)
      .then(data => { if (mounted) setEmailLogs(data || []); })
      .catch(() => { if (mounted) setEmailLogs([]); })
      .finally(() => { if (mounted) setEmailLogLoading(false); });
  };

  const fetchProviders = (mounted = true) => {
    setProviderLoading(true);
    fetch(`${API_BASE}/providers`, { credentials: 'include' })
      .then(r => r.json() as Promise<Provider[]>)
      .then(data => { if (mounted) setProviders(data || []); })
      .catch(() => { if (mounted) setProviders([]); })
      .finally(() => { if (mounted) setProviderLoading(false); });
  };

  useEffect(() => {
    let mounted = true;
    fetch(`${API_BASE}/admin/me`, { credentials: 'include' })
      .then(res => {
        if (!mounted) return;
        if (!res.ok) {
          navigate('/admin/login');
          return;
        }
        setAuthChecked(true);
        fetchProviders(mounted);
        fetchCustomers(mounted);
        fetchEmailLogs(mounted);
      })
      .catch(() => { if (mounted) navigate('/admin/login'); });
    return () => { mounted = false; };
  }, [navigate]);

  if (!authChecked) return null;

  const handleRegisterProvider = async () => {
    setProviderError('');
    if (!providerName || !providerEmail) {
      setProviderError('全て入力してください');
      return;
    }
    setProviderRegistering(true);
    try {
      const res = await fetch(`${API_BASE}/providers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name: providerName, email: providerEmail }),
      });
      if (!res.ok) throw new Error('登録に失敗しました');
      setProviderName('');
      setProviderEmail('');
      fetchProviders();
    } catch {
      setProviderError('登録に失敗しました');
    } finally {
      setProviderRegistering(false);
    }
  };

  const handleDeleteProvider = async (id: string) => {
    if (!window.confirm('このProviderを削除しますか？')) return;
    try {
      const res = await fetch(`${API_BASE}/providers/${id}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (!res.ok) throw new Error('削除に失敗しました');
      setProviders(prev => prev.filter(p => p.id !== id));
    } catch {
      alert('削除に失敗しました');
    }
  };

  const handleLogout = async () => {
    await fetch(`${API_BASE}/admin/logout`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'include' });
    navigate('/admin/login');
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwError('');
    if (pwNew !== pwConfirm) { setPwError('新しいパスワードが一致しません'); return; }
    if (pwNew.length < 8) { setPwError('パスワードは8文字以上で設定してください'); return; }
    setPwLoading(true);
    try {
      const res = await fetch(`${API_BASE}/admin/change-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ current_password: pwCurrent, new_password: pwNew }),
      });
      if (!res.ok) {
        const text = await res.text().catch(() => '');
        throw new Error(text || '変更に失敗しました');
      }
      setPwSuccess('パスワードを変更しました');
      setPwCurrent(''); setPwNew(''); setPwConfirm('');
      setTimeout(() => setPwSuccess(''), 4000);
    } catch (err) {
      setPwError(err instanceof Error ? err.message : '変更に失敗しました');
    } finally {
      setPwLoading(false);
    }
  };

  const pwErrorId = 'admin-pw-error';
  const providerErrorId = 'admin-provider-error';

  return (
    <main className="p-3 sm:p-6 relative">
      <button
        className="absolute top-3 right-3 sm:top-6 sm:right-6 py-2 px-4 bg-red-500 text-white border-none rounded cursor-pointer"
        onClick={handleLogout}
      >管理者ログアウト</button>
      <h1 className="text-lg sm:text-2xl">Admin Dashboard</h1>

      <section className="mb-8 max-w-sm">
        <button
          type="button"
          onClick={() => setPwOpen(o => !o)}
          className={`flex items-center gap-2 bg-transparent border-none text-white cursor-pointer p-0 ${pwOpen ? 'mb-3' : 'mb-0'}`}
          aria-expanded={pwOpen}
          aria-controls="admin-pw-form"
        >
          <h2 className="m-0">パスワード変更</h2>
          <span className="text-xs text-gray-500">{pwOpen ? '▲ 閉じる' : '▶ 開く'}</span>
        </button>
        {pwOpen && (
          <form id="admin-pw-form" onSubmit={handleChangePassword}>
            <div className="mb-2">
              <label htmlFor="admin-pw-current" className="sr-only">現在のパスワード</label>
              <input
                id="admin-pw-current"
                type="password"
                placeholder="現在のパスワード"
                value={pwCurrent}
                onChange={e => setPwCurrent(e.target.value)}
                required
                aria-required="true"
                aria-invalid={!!pwError}
                aria-describedby={pwError ? pwErrorId : undefined}
                className="w-full p-1.5 bg-neutral-700 text-white border border-gray-500 rounded box-border"
              />
            </div>
            <div className="mb-2">
              <label htmlFor="admin-pw-new" className="sr-only">新しいパスワード</label>
              <input
                id="admin-pw-new"
                type="password"
                placeholder="新しいパスワード（8文字以上）"
                value={pwNew}
                onChange={e => setPwNew(e.target.value)}
                required
                minLength={8}
                aria-required="true"
                aria-invalid={!!pwError}
                aria-describedby={pwError ? pwErrorId : undefined}
                className="w-full p-1.5 bg-neutral-700 text-white border border-gray-500 rounded box-border"
              />
            </div>
            <div className="mb-2">
              <label htmlFor="admin-pw-confirm" className="sr-only">新しいパスワード（確認）</label>
              <input
                id="admin-pw-confirm"
                type="password"
                placeholder="新しいパスワード（確認）"
                value={pwConfirm}
                onChange={e => setPwConfirm(e.target.value)}
                required
                aria-required="true"
                aria-invalid={!!pwError}
                aria-describedby={pwError ? pwErrorId : undefined}
                className="w-full p-1.5 bg-neutral-700 text-white border border-gray-500 rounded box-border"
              />
            </div>
            {pwError && <p id={pwErrorId} role="alert" className="text-red-400 text-[13px] mb-1.5">{pwError}</p>}
            {pwSuccess && <p role="status" className="text-green-400 text-[13px] mb-1.5">{pwSuccess}</p>}
            <button
              type="submit"
              disabled={pwLoading}
              aria-busy={pwLoading}
              className={`py-1.5 px-5 bg-blue-600 text-white border-none rounded ${pwLoading ? 'cursor-not-allowed' : 'cursor-pointer'}`}
            >
              {pwLoading ? '変更中...' : 'パスワードを変更する'}
            </button>
          </form>
        )}
      </section>

      <section className="mb-8">
        <h2>Provider 新規登録</h2>
        <div className="flex flex-wrap gap-2 items-center mb-2">
          <label htmlFor="admin-provider-name" className="sr-only">Provider名</label>
          <input
            id="admin-provider-name"
            type="text"
            placeholder="Provider名"
            value={providerName}
            onChange={e => setProviderName(e.target.value)}
            aria-required="true"
            aria-invalid={!!providerError}
            aria-describedby={providerError ? providerErrorId : undefined}
            className={darkInputClass}
          />
          <label htmlFor="admin-provider-email" className="sr-only">メールアドレス</label>
          <input
            id="admin-provider-email"
            type="email"
            placeholder="メールアドレス"
            value={providerEmail}
            onChange={e => setProviderEmail(e.target.value)}
            aria-required="true"
            aria-invalid={!!providerError}
            aria-describedby={providerError ? providerErrorId : undefined}
            className={darkInputClass}
          />
          <button
            type="button"
            onClick={handleRegisterProvider}
            disabled={providerRegistering}
            aria-busy={providerRegistering}
            className={`py-1.5 px-4 bg-blue-600 text-white border-none rounded ${providerRegistering ? 'cursor-not-allowed opacity-70' : 'cursor-pointer'}`}
          >{providerRegistering ? '登録中...' : '登録'}</button>
        </div>
        {providerError && <p id={providerErrorId} role="alert" className="text-red-600">{providerError}</p>}
      </section>

      <section className="mb-8">
        <h2>Customer 一覧</h2>
        {customerLoading ? <div>Loading…</div> : (
          <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse">
            <thead>
              <tr>
                <th className={thClass}>Name</th>
                <th className={thClass}>Email</th>
                <th className={thClass}>プラン</th>
                <th className={thClass}>サブスク状態</th>
                <th className={thClass}>登録日</th>
              </tr>
            </thead>
            <tbody>
              {customers.map(c => (
                <tr key={c.id}>
                  <td className="py-2 px-1">{c.name}</td>
                  <td className="py-2 px-1">{c.email}</td>
                  <td className="py-2 px-1">
                    <span className={`py-0.5 px-2 rounded-xl text-xs font-semibold ${c.plan === 'pro' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-700'}`}>
                      {c.plan === 'pro' ? 'Pro' : 'Free'}
                    </span>
                  </td>
                  <td className="py-2 px-1 text-xs text-gray-400">{c.subscription_status || '-'}</td>
                  <td className="py-2 px-1 text-xs text-gray-400">{c.created_at ? new Date(c.created_at).toLocaleDateString('ja-JP') : '-'}</td>
                </tr>
              ))}
              {customers.length === 0 && <tr><td colSpan={5} className="p-3 text-gray-400">Customerが見つかりません</td></tr>}
            </tbody>
          </table>
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2>メール送信ログ</h2>
        {emailLogLoading ? <div>Loading…</div> : (
          <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse text-[13px]">
            <thead>
              <tr>
                <th className={thClass}>宛先</th>
                <th className={thClass}>件名</th>
                <th className={thClass}>ステータス</th>
                <th className={thClass}>送信日時</th>
              </tr>
            </thead>
            <tbody>
              {emailLogs.map(l => (
                <tr key={l.id}>
                  <td className="py-1.5 px-1 text-gray-300">{l.to_email}</td>
                  <td className="py-1.5 px-1 text-gray-300">{l.subject}</td>
                  <td className="py-1.5 px-1">
                    <span className={`py-0.5 px-2 rounded-xl text-[11px] font-semibold ${l.status === 'success' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}`}>
                      {l.status}
                    </span>
                    {l.error_message && <span className="ml-1.5 text-red-400 text-[11px]">{l.error_message}</span>}
                  </td>
                  <td className="py-1.5 px-1 text-gray-400 whitespace-nowrap">{l.sent_at ? new Date(l.sent_at).toLocaleString('ja-JP') : '-'}</td>
                </tr>
              ))}
              {emailLogs.length === 0 && <tr><td colSpan={4} className="p-3 text-gray-400">送信ログがありません</td></tr>}
            </tbody>
          </table>
          </div>
        )}
      </section>

      <section>
        <h2>Provider 一覧</h2>
        {providerLoading ? <div>Loading…</div> : (
          <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] border-collapse">
            <thead>
              <tr>
                <th className={thClass}>Name</th>
                <th className={thClass}>Email</th>
                <th className={thClass}>ステータス</th>
                <th className="border-b border-gray-500 py-2 px-1">操作</th>
              </tr>
            </thead>
            <tbody>
              {providers.map(p => (
                <tr key={p.id}>
                  <td className="py-2 px-1">{p.name}</td>
                  <td className="py-2 px-1">{p.email}</td>
                  <td className="py-2 px-1">
                    <span className={`py-0.5 px-2 rounded-xl text-xs ${p.is_active ? 'bg-green-100 text-green-800' : 'bg-amber-100 text-amber-800'}`}>
                      {p.is_active ? '本登録済み' : '仮登録（未設定）'}
                    </span>
                  </td>
                  <td className="py-2 px-1 text-center">
                    <button
                      onClick={() => handleDeleteProvider(p.id)}
                      className="text-red-600 border-none bg-transparent cursor-pointer text-[13px]"
                    >削除</button>
                  </td>
                </tr>
              ))}
              {providers.length === 0 && <tr><td colSpan={4} className="p-3 text-gray-400">Providerが見つかりません</td></tr>}
            </tbody>
          </table>
          </div>
        )}
      </section>
    </main>
  );
}
