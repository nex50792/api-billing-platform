import { useEffect } from 'react';
import { useNavigate } from 'react-router';

export default function ProviderAPIsRedirect() {
  const navigate = useNavigate();
  useEffect(() => { navigate('/provider', { replace: true }); }, [navigate]);
  return null;
}
