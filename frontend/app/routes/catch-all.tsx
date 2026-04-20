import { Outlet, useLocation } from 'react-router';
import React, { useEffect, useState } from 'react';
import type { Route } from './+types/home';

export function meta(_args: Route.MetaArgs) {
  return [
    { title: 'API Billing Platform - Dashboard' },
    { name: 'description', content: 'SaaS API billing platform dashboard' },
  ];
}

export default function CatchAll() {
  const [ClientComp, setClientComp] = useState<React.ComponentType | null>(null);
  const location = useLocation();

  useEffect(() => {
    if (typeof window === 'undefined') return;
    let active = true;
    const p = location.pathname.replace(/\/+$/, '');
    const load = async () => {
      try {
        if (p === '/provider/set-password') {
          const m = await import('./provider/set-password');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p === '/provider/apis' || p === '/provider' || p === '/provider/') {
          const m = await import('./provider/apis');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p === '/provider/apis/new') {
          const m = await import('./provider/apis/new');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (/^\/provider\/apis\/[^/]+\/edit$/.test(p)) {
          const m = await import('./provider/apis/[id]/edit');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p.startsWith('/provider')) {
          const m = await import('./provider/apis');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p.startsWith('/admin') && p !== '/admin/login') {
          (async () => {
            try {
              const res = await import('../lib/adminApi');
              const me = await res.adminApiClient.me();
              if (!me) {
                window.location.href = '/admin/login';
                return;
              }
            } catch {
              window.location.href = '/admin/login';
              return;
            }
          })();
        }
        if (p === '/admin/login') {
          const m = await import('./admin/login');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p === '/admin' || p === '/admin/' || p === '/admin/dashboard') {
          const m = await import('./admin/dashboard');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (p.startsWith('/admin')) {
          const m = await import('./admin/dashboard');
          if (active) setClientComp(() => (m && m.default) || null);
          return;
        }
        if (active) setClientComp(null);
      } catch (e) {
        if (active) setClientComp(null);
      }
    };
    load();
    return () => { active = false };
  }, [location.pathname]);

  if (ClientComp) return <ClientComp />;

  return <Outlet />;
}
