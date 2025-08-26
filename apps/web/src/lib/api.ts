// API設定
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081';
const USE_MOCK = process.env.NEXT_PUBLIC_USE_MOCK === 'true' || !process.env.NEXT_PUBLIC_API_URL;

// APIクライアントクラス
class ApiClient {
  private baseUrl: string;
  private useMock: boolean;

  constructor() {
    this.baseUrl = API_BASE_URL;
    this.useMock = USE_MOCK;
  }

  private async request(endpoint: string, options: RequestInit = {}) {
    const url = this.useMock 
      ? `/api/mock${endpoint}` 
      : `${this.baseUrl}${endpoint}`;

    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      throw new Error(`API request failed: ${response.statusText}`);
    }

    return response.json();
  }

  // 戦略関連API
  async getStrategies() {
    return this.request('/strategies');
  }

  async createStrategy(data: any) {
    return this.request('/strategies', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getStrategy(id: string) {
    return this.request(`/strategies/${id}`);
  }

  async updateStrategy(id: string, data: any) {
    return this.request(`/strategies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteStrategy(id: string) {
    return this.request(`/strategies/${id}`, {
      method: 'DELETE',
    });
  }

  // 注文関連API
  async getOrders(params?: { limit?: number }) {
    const query = params?.limit ? `?limit=${params.limit}` : '';
    return this.request(`/orders${query}`);
  }

  async createOrder(data: any) {
    return this.request('/orders', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getOrder(id: string) {
    return this.request(`/orders/${id}`);
  }

  async updateOrder(id: string, data: any) {
    return this.request(`/orders/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async cancelOrder(id: string) {
    return this.request(`/orders/${id}`, {
      method: 'DELETE',
    });
  }

  // バックテスト関連API
  async getBacktests() {
    return this.request('/backtests');
  }

  async createBacktest(data: any) {
    return this.request('/backtests', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getBacktest(id: string) {
    return this.request(`/backtests/${id}`);
  }

  async deleteBacktest(id: string) {
    return this.request(`/backtests/${id}`, {
      method: 'DELETE',
    });
  }

  // ユニバース関連API
  async getUniverse() {
    return this.request('/universe');
  }

  async addSymbol(data: any) {
    return this.request('/universe', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async removeSymbol(symbol: string) {
    return this.request(`/universe/${symbol}`, {
      method: 'DELETE',
    });
  }

  // ヘルスチェック
  async healthCheck() {
    return this.request('/healthz');
  }
}

// シングルトンインスタンス
export const apiClient = new ApiClient();

// 使用例:
// import { apiClient } from '@/lib/api';
// const strategies = await apiClient.getStrategies();