import { randomUUID } from 'expo-crypto';
import type { DraftPhoto } from './photos';
import {
  checkJpeg, checkPhotoBytes, copyValue, DraftConflictError, draftInfo, invalidDraft,
  validateIdentity, validateMetadata, validateRevision,
} from './draft-store.types';
import type { DraftInfo, DraftMetadata, DraftValue, PhotoMetadata, StoredDraft } from './draft-store.types';

export { DraftConflictError } from './draft-store.types';
export type { DraftInfo, DraftValue, PendingDayDeletion, PendingMessage, StoredDraft } from './draft-store.types';

type WebDraft = Omit<DraftMetadata, 'photos'> & { photos: (PhotoMetadata & { blob: Blob })[] };
let connection: Promise<IDBDatabase> | undefined;
const photoBlobs = new Map<string, Blob>();

function database(): Promise<IDBDatabase> {
  if (!connection) {
    connection = new Promise<IDBDatabase>((resolve, reject) => {
      if (typeof indexedDB === 'undefined') { reject(new Error('Der Browser stellt keinen lokalen Entwurfsspeicher bereit.')); return; }
      const request = indexedDB.open('fitty-drafts', 1);
      let blocked = false;
      request.onupgradeneeded = () => {
        const store = request.result.createObjectStore('drafts', { keyPath: ['userId', 'date'] });
        store.createIndex('userId', 'userId');
      };
      request.onerror = () => reject(request.error ?? new Error('Der lokale Entwurfsspeicher konnte nicht geöffnet werden.'));
      request.onblocked = () => { blocked = true; reject(new Error('Bitte schließe andere Fitty-Fenster, damit der Entwurfsspeicher geöffnet werden kann.')); };
      request.onsuccess = () => {
        const db = request.result;
        if (blocked) { db.close(); return; }
        db.onversionchange = () => { db.close(); connection = undefined; };
        db.onclose = () => { connection = undefined; };
        resolve(db);
      };
    }).catch(error => { connection = undefined; throw error; });
  }
  return connection;
}

async function transaction<T>(mode: IDBTransactionMode, action: (store: IDBObjectStore, done: (value: T) => void, fail: (error: unknown) => void) => void): Promise<T> {
  const db = await database();
  return new Promise<T>((resolve, reject) => {
    const tx = db.transaction('drafts', mode, { durability: 'strict' });
    let result: T;
    let failure: unknown;
    tx.oncomplete = () => resolve(result);
    tx.onabort = () => reject(failure ?? tx.error ?? new Error('Der Entwurf konnte nicht lokal gespeichert werden.'));
    const fail = (error: unknown) => { failure = error; tx.abort(); };
    try { action(tx.objectStore('drafts'), value => { result = value; }, fail); } catch (error) { fail(error); }
  });
}

function validateWeb(value: unknown, userId: string, date: string): asserts value is WebDraft {
  validateMetadata(value, userId, date);
  for (const photo of value.photos) {
    const blob = (photo as PhotoMetadata & { blob?: unknown }).blob;
    if (!(blob instanceof Blob) || blob.type !== 'image/jpeg') invalidDraft();
    checkPhotoBytes(blob.size);
  }
}

export async function readDraft(userId: string, date: string): Promise<StoredDraft | null> {
  validateIdentity(userId, date);
  const record = await transaction<unknown>('readonly', (store, done) => {
    const request = store.get([userId, date]); request.onsuccess = () => done(request.result);
  });
  if (record === undefined) return null;
  validateWeb(record, userId, date);
  const photos: DraftPhoto[] = [];
  try {
    for (const photo of record.photos) {
      checkJpeg(new Uint8Array(await photo.blob.slice(0, 3).arrayBuffer()));
      const uri = URL.createObjectURL(photo.blob); photoBlobs.set(uri, photo.blob);
      photos.push({ id: photo.id, width: photo.width, height: photo.height, uri });
    }
    return { text: record.text, pending: record.pending, deletion: record.deletion, photos, revision: record.revision, updatedAt: record.updatedAt };
  } catch (error) { releasePhotos(photos); throw error; }
}

export async function listDrafts(userId: string): Promise<DraftInfo[]> {
  validateIdentity(userId);
  const rows = await transaction<unknown[]>('readonly', (store, done) => {
    const request = store.index('userId').getAll(userId); request.onsuccess = () => done(request.result);
  });
  const result: DraftInfo[] = [];
  for (const value of rows) {
    const date = (value as { date?: unknown })?.date;
    if (typeof date !== 'string') invalidDraft();
    validateWeb(value, userId, date);
    const info = draftInfo(value); if (info) result.push(info);
  }
  return result.sort((a, b) => b.updatedAt - a.updatedAt || b.date.localeCompare(a.date));
}

export async function writeDraft(userId: string, date: string, expectedRevision: string | null, value: DraftValue): Promise<StoredDraft> {
  validateIdentity(userId, date); validateRevision(expectedRevision);
  const snapshot = copyValue(value);
  const photos: WebDraft['photos'] = [];
  for (const photo of snapshot.photos) {
    if (!photo.uri.startsWith('blob:') && !photo.uri.startsWith('data:image/jpeg;')) invalidDraft();
    let blob = photoBlobs.get(photo.uri);
    if (!blob) {
      const response = await fetch(photo.uri);
      if (!response.ok) throw new Error('Ein Entwurfsfoto konnte nicht lokal gelesen werden.');
      blob = await response.blob();
      checkPhotoBytes(blob.size); checkJpeg(new Uint8Array(await blob.slice(0, 3).arrayBuffer()));
      photoBlobs.set(photo.uri, blob);
    }
    photos.push({ id: photo.id, width: photo.width, height: photo.height, blob: blob.type === 'image/jpeg' ? blob : new Blob([blob], { type: 'image/jpeg' }) });
  }
  const revision = randomUUID();
  const updatedAt = Date.now();
  await transaction<void>('readwrite', (store, done, fail) => {
    const request = store.get([userId, date]);
    request.onsuccess = () => {
      try {
        const current: unknown = request.result;
        if (current !== undefined) validateWeb(current, userId, date);
        if ((current === undefined ? null : (current as WebDraft).revision) !== expectedRevision) throw new DraftConflictError();
        // Replacing the row replaces all blobs in the same transaction, including an empty tombstone.
        store.put({ schema: 1, userId, date, text: snapshot.text, pending: snapshot.pending, deletion: snapshot.deletion, photos, revision, updatedAt } satisfies WebDraft);
        done();
      } catch (error) { fail(error); }
    };
  });
  return { ...snapshot, revision, updatedAt };
}

export function releasePhotos(photos: DraftPhoto[]): void {
  for (const photo of photos) { photoBlobs.delete(photo.uri); if (photo.uri.startsWith('blob:')) URL.revokeObjectURL(photo.uri); }
}
