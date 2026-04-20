class AdminAPIClient {
  private async request(path: string, init?: RequestInit) {
    const url = `${API_BASE_URL}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(init && init.headers ? (init.headers as Record<string, string>) : {}),
    };
    const res = await fetch(url, { ...init, headers, credentials: 'include' });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`Request failed: ${res.status} ${res.statusText} ${text}`);
    }
    const txt = await res.text();
    if (!txt) return undefined;
    try {
      return JSON.parse(txt);
    } catch {
      return txt;
    }
  }

  async login(email: string, password: string): Promise<void> {
    await this.request('/admin/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async logout(): Promise<void> {
    await this.request('/admin/logout', { method: 'POST' });
  }

  async me(): Promise<{ email: string } | null> {
    return this.request('/admin/me', { method: 'GET' });
  }
}

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';
export const adminApiClient = new AdminAPIClient();
