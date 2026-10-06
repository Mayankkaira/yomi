import { api } from './client';
import type { ApiEnvelope, ConsentRequest } from '../types';

export const consentApi = {
  async pending(citizenId: string): Promise<ConsentRequest[]> {
    const res = await api.get<ApiEnvelope<ConsentRequest[]>>(`/consent/pending/${citizenId}`);
    return res.data.data;
  },

  async history(citizenId: string): Promise<ConsentRequest[]> {
    const res = await api.get<ApiEnvelope<ConsentRequest[]>>(`/consent/history/${citizenId}`);
    return res.data.data;
  },

  async approve(requestId: string): Promise<ConsentRequest> {
    const res = await api.post<ApiEnvelope<ConsentRequest>>(`/consent/approve/${requestId}`);
    return res.data.data;
  },

  async reject(requestId: string): Promise<ConsentRequest> {
    const res = await api.post<ApiEnvelope<ConsentRequest>>(`/consent/reject/${requestId}`);
    return res.data.data;
  },

  async revoke(requestId: string): Promise<ConsentRequest> {
    const res = await api.delete<ApiEnvelope<ConsentRequest>>(`/consent/revoke/${requestId}`);
    return res.data.data;
  },
};
