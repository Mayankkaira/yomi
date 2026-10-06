// Domain types for every entity Yomi exchanges with the backend. These mirror
// the Go structs in backend/internal/models.

export type Role = 'citizen' | 'institution' | 'admin';

export type ConsentStatus = 'pending' | 'approved' | 'rejected' | 'revoked' | 'expired';

export type ConsentEventType =
  | 'consent_requested'
  | 'consent_approved'
  | 'consent_rejected'
  | 'consent_revoked'
  | 'data_accessed';

export interface Citizen {
  id: string;
  aadhaarId: string;
  fullName: string;
  email: string;
  phone: string | null;
  createdAt: string;
}

export interface ConsentRequest {
  id: string;
  citizenId: string;
  institutionId: string;
  dataScope: string[];
  purpose: string;
  status: ConsentStatus;
  authToken: string | null;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
}

export interface Institution {
  id: string;
  name: string;
  sector: string;
  isActive: boolean;
  createdAt: string;
}

export interface AuditEntry {
  id: string;
  sequenceNo: number;
  eventType: ConsentEventType;
  citizenId: string | null;
  institutionId: string | null;
  requestId: string | null;
  metadata: Record<string, unknown> | null;
  prevHash: string;
  entryHash: string;
  createdAt: string;
}

export interface ChainLink {
  entry: AuditEntry;
  valid: boolean;
}

export interface ChainVerification {
  intact: boolean;
  entriesChecked: number;
  chain: ChainLink[];
}

export interface AuthResponse {
  accessToken: string;
  citizen: Citizen;
}

export interface ApiEnvelope<T> {
  data: T;
}

export interface ApiError {
  error: string;
  code: string;
}

// --- Frontend-only view models ----------------------------------------------

export interface ConsentRequestView extends ConsentRequest {
  institution?: Institution;
}

export interface DashboardStats {
  pending: number;
  approved: number;
  rejected: number;
  revoked: number;
  institutions: number;
  auditEvents: number;
}

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}
