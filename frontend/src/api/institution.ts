import { api } from './client';
import type { ApiEnvelope, Institution } from '../types';

export interface RegisterInstitutionPayload {
  name: string;
  sector: string;
}

export interface InstitutionWithKey {
  data: Institution;
  apiKey: string;
}

export const institutionApi = {
  async get(id: string): Promise<Institution> {
    const res = await api.get<ApiEnvelope<Institution>>(`/institution/${id}`);
    return res.data.data;
  },

  async register(payload: RegisterInstitutionPayload): Promise<InstitutionWithKey> {
    const res = await api.post<InstitutionWithKey>('/institution/register', payload);
    return res.data;
  },

  async rotateKey(id: string): Promise<InstitutionWithKey> {
    const res = await api.post<InstitutionWithKey>(`/institution/${id}/rotate-key`);
    return res.data;
  },
};
