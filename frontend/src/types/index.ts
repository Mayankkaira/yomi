export type Role = 'citizen' | 'institution' | 'admin';

export type InstitutionRole = 'viewer' | 'analyst' | 'admin';

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
  institutionRole?: InstitutionRole;
  dataScope: string[];
  purpose: string;
  status: ConsentStatus;
  accessTokenHash?: string;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
}

export interface Institution {
  id: string;
  name: string;
  sector: string;
  role: InstitutionRole;
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

export interface Receipt {
  receiptId: string;
  version: string;
  sequenceNo: number;
  eventType: ConsentEventType;
  citizenId?: string;
  institutionId?: string;
  requestId?: string;
  metadata?: Record<string, unknown>;
  recordedAt: string;
  prevHash: string;
  entryHash: string;
  content: string;
  signature: string;
}

export interface ReceiptVerification {
  valid: boolean;
  reason?: 'NOT_ISSUED' | 'SIGNATURE_MISMATCH';
  receipt?: Receipt;
  chainAnchored?: boolean;
}

export interface AccessTokenCheck {
  active: boolean;
  reason?: 'INVALID' | 'EXPIRED' | 'REVOKED';
  requestId?: string;
  institutionId?: string;
  scope?: string[];
  expiresAt?: string;
}

export interface ScopeSummary {
  key: string;
  label: string;
  detail: string;
}

export interface ScopeCatalog {
  role: InstitutionRole;
  scopes: ScopeSummary[];
}

export interface ApproveResult {
  request: ConsentRequest;
  accessToken: string;
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