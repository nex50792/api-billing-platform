import React, { useState } from 'react';
import { useNavigate } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export default function ProviderLogin() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/provider/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
        credentials: 'include',
      });
      if (res.ok) {
        navigate('/provider');
      } else {
        setError('ログインに失敗しました');
      }
    } finally {
      setLoading(false);
    }
  };

  const errorId = 'provider-login-error';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-6 text-gray-900 font-bold">プロバイダーログイン</h2>
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="provider-login-email" className="block mb-2">メールアドレス</label>
          <input
            id="provider-login-email"
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className="w-full p-2 text-base border border-gray-300 rounded"
          />
        </div>
        <div className="mb-4">
          <label htmlFor="provider-login-password" className="block mb-2">パスワード</label>
          <input
            id="provider-login-password"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : undefined}
            className="w-full p-2 text-base border border-gray-300 rounded"
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
      {error && <p id={errorId} role="alert" className="mt-4 text-red-600 font-medium">{error}</p>}
      <div className="mt-4 text-center">
        <a href="/provider/forgot-password" className="text-gray-500 text-sm no-underline">パスワードをお忘れの方はこちら</a>
      </div>
    </main>
  );
}
