class APIClient {
    async logout(): Promise<void> {
      await this.request('/customer/logout', { method: 'POST' });
    }
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
    await this.request('/customer/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async me(): Promise<User | null> {
    return this.request('/customer/me', { method: 'GET' });
  }

  async issueAPIKey(providedApiId?: string): Promise<{ plain_key: string; id: string }> {
    // backend returns: { api_key: "<plain_key>", id: "<uuid>" }
    const res = await this.request('/apikeys', {
      method: 'POST',
      body: JSON.stringify({ provided_api_id: providedApiId }),
    });
    return { plain_key: res.api_key, id: res.id };
  }

  async getCustomerAPIKeys(): Promise<APIKey[]> {
    return this.request('/customer/apikeys', { method: 'GET' });
  }

  async deleteAPIKey(id: string): Promise<void> {
    return this.request(`/apikeys/${id}`, { method: 'DELETE' });
  }

  async checkBilling(apiKey: string): Promise<BillingCheckResponse> {
    return this.request('/billing/check', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}` },
    });
  }

  async getProvidedAPIs(): Promise<ProvidedAPI[]> {
    return this.request('/provided-apis', { method: 'GET' });
  }

  async getProviderAPIs(): Promise<ProvidedAPI[]> {
    return this.request('/provider/apis', { method: 'GET' });
  }

  async createProvidedAPI(input: { name: string; description: string; documentation?: string }): Promise<void> {
    return this.request('/provided-apis', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  async updateProvidedAPI(id: string, input: { name: string; description: string; documentation?: string }): Promise<void> {
    return this.request(`/provided-apis/${id}`, {
      method: 'PUT',
      body: JSON.stringify(input),
    });
  }

  async deleteProvidedAPI(id: string): Promise<void> {
    return this.request(`/provided-apis/${id}`, {
      method: 'DELETE',
    });
  }

  async createCheckoutSession(): Promise<{ checkout_url: string; session_id: string }> {
    return this.request('/customer/checkout', {
      method: 'POST',
      body: JSON.stringify({}),
    });
  }

  async getSubscription(): Promise<{ plan: string; subscription_status?: string; stripe_customer_id?: string }> {
    return this.request('/customer/subscription', { method: 'GET' });
  }

  async createBillingPortalSession(): Promise<{ portal_url: string }> {
    return this.request('/customer/billing-portal', { method: 'POST' });
  }

  async changePassword(currentPassword: string, newPassword: string): Promise<void> {
    return this.request('/customer/change-password', {
      method: 'POST',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    });
  }

  async changeProviderPassword(currentPassword: string, newPassword: string): Promise<void> {
    return this.request('/provider/change-password', {
      method: 'POST',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    });
  }
}

export const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface APIKey {
  id: string;
  customer_id: string;
  provided_api_id?: string;
  usage_count: number;
  usage_reset_at: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  plan: string;
  subscription_status?: string;
  subscription_cancel_at?: string;
}

export interface ProvidedAPI {
  id: string;
  name: string;
  description: string;
  documentation?: string;
  created_at: string;
  key_count?: number;
  total_usage?: number;
}

export interface BillingCheckResponse {
  customer: { id: string; plan: string; subscription_status?: string };
  api_key: { id: string; usage_count: number; usage_reset_at: string };
  provided_api?: { id: string; name: string };
}



export const apiClient = new APIClient();
