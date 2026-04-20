import { Outlet } from 'react-router';

export default function ProviderRoot() {
  return <Outlet />;
}

export function meta() {
  return [{ title: 'Provider - API管理' }];
}
