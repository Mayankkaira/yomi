import { api } from './client';
import type { ApiEnvelope, AuditEntry, ChainVerification } from '../types';

export const auditApi = {
  async forCitizen(citizenId: string): Promise<AuditEntry[]> {
    const res = await api.get<ApiEnvelope<AuditEntry[]>>(`/audit/citizen/${citizenId}`);
    return res.data.data;
  },

  async forInstitution(institutionId: string): Promise<AuditEntry[]> {
    const res = await api.get<ApiEnvelope<AuditEntry[]>>(`/audit/institution/${institutionId}`);
    return res.data.data;
  },

  async byEvent(eventId: string): Promise<AuditEntry> {
    const res = await api.get<ApiEnvelope<AuditEntry>>(`/audit/event/${eventId}`);
    return res.data.data;
  },

  async verify(): Promise<ChainVerification> {
    const res = await api.get<ApiEnvelope<ChainVerification>>('/audit/verify');
    return res.data.data;
  },
};
