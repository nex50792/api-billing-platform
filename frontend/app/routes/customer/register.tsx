import React, { useState } from 'react';
import { useNavigate } from 'react-router';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const lightInputClass = 'w-full p-2 text-base border border-gray-300 rounded';

export default function CustomerRegister() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [showVerify, setShowVerify] = useState(false);
  const [verifyCode, setVerifyCode] = useState('');
  const [verifyMsg, setVerifyMsg] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [verifying, setVerifying] = useState(false);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);
    setVerifyMsg(null);
    setSubmitting(true);
    try {
      const res = await fetch(`${API_BASE}/customer/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, name }),
        credentials: 'include',
      });
      if (res.ok) {
        setSuccess('登録が完了しました。メールの認証コードを入力してください。');
        setShowVerify(true);
      } else {
        setError('登録に失敗しました');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleVerify = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setVerifyMsg(null);
    setVerifying(true);
    try {
      const res = await fetch(`${API_BASE}/customer/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code: verifyCode }),
        credentials: 'include',
      });
      if (res.ok) {
        setVerifyMsg('認証が完了しました。ログイン後ページへ移動します。');
        const loginRes = await fetch(`${API_BASE}/customer/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password }),
          credentials: 'include',
        });
        if (loginRes.ok) {
          setTimeout(() => navigate('/'), 1200);
        } else {
          setVerifyMsg('認証は成功しましたが、ログインに失敗しました。');
        }
      } else {
        setVerifyMsg('認証に失敗しました。コードを確認してください。');
      }
    } finally {
      setVerifying(false);
    }
  };

  const registerErrorId = 'register-error';
  const verifyErrorId = 'verify-msg';

  return (
    <main className="max-w-sm mx-auto my-10 p-6 bg-white rounded-lg shadow text-gray-900">
      <h2 className="mb-6 text-gray-900 font-bold">顧客登録</h2>
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="register-name" className="block mb-2">名前</label>
          <input
            id="register-name"
            type="text"
            value={name}
            onChange={e => setName(e.target.value)}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? registerErrorId : undefined}
            className={lightInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="register-email" className="block mb-2">メールアドレス</label>
          <input
            id="register-email"
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? registerErrorId : undefined}
            className={lightInputClass}
          />
        </div>
        <div className="mb-4">
          <label htmlFor="register-password" className="block mb-2">パスワード</label>
          <input
            id="register-password"
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
            aria-required="true"
            aria-invalid={!!error}
            aria-describedby={error ? registerErrorId : undefined}
            className={lightInputClass}
          />
        </div>
        <button
          type="submit"
          disabled={submitting}
          aria-busy={submitting}
          className={`w-full p-3 text-base bg-emerald-600 text-white border-none rounded font-semibold ${submitting ? 'cursor-not-allowed' : 'cursor-pointer'}`}
        >
          {submitting ? '登録中...' : '登録'}
        </button>
      </form>
      {error && <p id={registerErrorId} role="alert" className="mt-4 text-red-600 font-medium">{error}</p>}
      {success && <p className="mt-4 text-green-600 font-medium">{success}</p>}

      {showVerify && (
        <form onSubmit={handleVerify} className="mt-8">
          <div className="mb-4">
            <label htmlFor="verify-code" className="block mb-2">認証コード（メールで受信した6桁）</label>
            <input
              id="verify-code"
              type="text"
              value={verifyCode}
              onChange={e => setVerifyCode(e.target.value)}
              className={lightInputClass}
              maxLength={6}
              required
              aria-required="true"
              aria-describedby={verifyMsg ? verifyErrorId : undefined}
            />
          </div>
          <button
            type="submit"
            disabled={verifying}
            aria-busy={verifying}
            className={`w-full p-3 text-base bg-blue-600 text-white border-none rounded font-semibold ${verifying ? 'cursor-not-allowed' : 'cursor-pointer'}`}
          >
            {verifying ? '認証中...' : '認証する'}
          </button>
          {verifyMsg && (
            <p
              id={verifyErrorId}
              role="status"
              className={`mt-4 font-medium ${verifyMsg.includes('完了') ? 'text-green-600' : 'text-red-600'}`}
            >
              {verifyMsg}
            </p>
          )}
        </form>
      )}
    </main>
  );
}
