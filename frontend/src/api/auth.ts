import { api } from './client';
import type { ApiEnvelope, AuthResponse, Citizen } from '../types';

export interface RegisterPayload {
  fullName: string;
  aadhaarId: string;
  email: string;
  phone?: string;
  password: string;
}

export interface LoginPayload {
  aadhaarId: string;
  password: string;
}

export const authApi = {
  async register(payload: RegisterPayload): Promise<Citizen> {
    const res = await api.post<ApiEnvelope<Citizen>>('/auth/register', payload);
    return res.data.data;
  },

  async login(payload: LoginPayload): Promise<AuthResponse> {
    const res = await api.post<ApiEnvelope<AuthResponse>>('/auth/login', payload);
    return res.data.data;
  },

  /** Explicitly refresh; the httpOnly cookie supplies the refresh token. */
  async refresh(): Promise<AuthResponse> {
    const res = await api.post<ApiEnvelope<AuthResponse>>('/auth/refresh', {});
    return res.data.data;
  },

  async logout(): Promise<void> {
    await api.post('/auth/logout', {});
  },
};
