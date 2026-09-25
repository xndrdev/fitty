import type { DraftPhoto } from './photos';

export type PendingMessage = { client_id: string; content: string; attachment_ids: string[] };
export type PendingDayDeletion = { request_id: string; day_id: string; version: number };
export type DraftValue = { text: string; photos: DraftPhoto[]; pending: PendingMessage | null; deletion: PendingDayDeletion | null };
export type StoredDraft = DraftValue & { revision: string; updatedAt: number };
export type DraftInfo = { date: string; updatedAt: number; hasPending: boolean };
export type PhotoMetadata = Omit<DraftPhoto, 'uri'>;
export type DraftMetadata = Omit<StoredDraft, 'photos'> & { schema: 1; userId: string; date: string; photos: PhotoMetadata[] };

export class DraftConflictError extends Error {
  constructor() {
    super('Dieser Entwurf wurde inzwischen in einem anderen Fenster geändert. Bitte lade den gespeicherten Stand.');
    this.name = 'DraftConflictError';
  }
}

export const MAX_PHOTO_BYTES = 8 * 1024 * 1024;
export const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
export function invalidDraft(): never { throw new Error('Der lokale Entwurf ist unvollständig oder beschädigt. Er wurde nicht überschrieben.'); }
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value); }
function uuid(value: unknown): value is string { return typeof value === 'string' && UUID.test(value); }
function text(value: unknown): value is string { return typeof value === 'string' && value.length <= 8000; }

export function validateIdentity(userId: string, date?: string): void {
  if (!uuid(userId)) invalidDraft();
  if (date !== undefined && (!/^\d{4}-\d{2}-\d{2}$/.test(date) || date < '1900-01-01' || date > '9999-12-31'
    || !Number.isFinite(Date.parse(date)) || new Date(date).toISOString().slice(0, 10) !== date)) invalidDraft();
}

export function validateRevision(revision: unknown): asserts revision is string | null {
  if (revision !== null && !uuid(revision)) invalidDraft();
}

export function validateValue(value: unknown): asserts value is DraftValue {
  if (!record(value) || !text(value.text) || !Array.isArray(value.photos) || value.photos.length > 4) invalidDraft();
  const ids = new Set<string>();
  for (const photo of value.photos) {
    if (!record(photo) || !uuid(photo.id) || ids.has(photo.id) || typeof photo.uri !== 'string' || !photo.uri
      || !Number.isInteger(photo.width) || !Number.isInteger(photo.height)
      || Number(photo.width) < 1 || Number(photo.height) < 1 || Number(photo.width) > 4096 || Number(photo.height) > 4096) invalidDraft();
    ids.add(photo.id);
  }
  const pending = value.pending;
  if (pending !== null && (!record(pending) || !uuid(pending.client_id) || !text(pending.content)
    || !Array.isArray(pending.attachment_ids) || pending.attachment_ids.length > 4
    || !pending.attachment_ids.every(uuid) || new Set(pending.attachment_ids).size !== pending.attachment_ids.length)) invalidDraft();
  const deletion = value.deletion;
  if (deletion !== null && (!record(deletion) || !uuid(deletion.request_id) || typeof deletion.day_id !== 'string'
    || !/^[1-9]\d{0,18}$/.test(deletion.day_id) || !Number.isSafeInteger(deletion.version) || Number(deletion.version) < 1)) invalidDraft();
}

export function validateMetadata(value: unknown, userId: string, date: string): asserts value is DraftMetadata {
  if (!record(value) || value.schema !== 1 || value.userId !== userId || value.date !== date || !uuid(value.revision)
    || !Number.isSafeInteger(value.updatedAt) || Number(value.updatedAt) < 1 || !Array.isArray(value.photos)) invalidDraft();
  validateIdentity(userId, date);
  validateValue({ ...value, photos: value.photos.map(photo => record(photo) ? { ...photo, uri: 'stored:' } : photo) });
}

export function copyValue(value: DraftValue): DraftValue {
  validateValue(value);
  return { text: value.text, photos: value.photos.map(photo => ({ id: photo.id, uri: photo.uri, width: photo.width, height: photo.height })),
    pending: value.pending ? { client_id: value.pending.client_id, content: value.pending.content, attachment_ids: [...value.pending.attachment_ids] } : null,
    deletion: value.deletion ? { request_id: value.deletion.request_id, day_id: value.deletion.day_id, version: value.deletion.version } : null };
}

export function draftInfo(value: DraftMetadata): DraftInfo | null {
  if (!value.text.trim() && !value.photos.length && !value.pending && !value.deletion) return null;
  return { date: value.date, updatedAt: value.updatedAt, hasPending: Boolean(value.pending || value.deletion) };
}

export function checkPhotoBytes(size: number): void {
  if (!Number.isSafeInteger(size) || size < 3 || size > MAX_PHOTO_BYTES) {
    throw new Error('Ein Entwurfsfoto fehlt oder ist größer als 8 MiB. Der gespeicherte Entwurf wurde nicht ersetzt.');
  }
}

export function checkJpeg(bytes: Uint8Array): void {
  if (bytes[0] !== 0xff || bytes[1] !== 0xd8 || bytes[2] !== 0xff) {
    throw new Error('Ein Entwurfsfoto konnte nicht als JPEG gelesen werden. Der gespeicherte Entwurf wurde nicht ersetzt.');
  }
}
