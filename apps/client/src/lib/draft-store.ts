import AsyncStorage from '@react-native-async-storage/async-storage';
import { randomUUID } from 'expo-crypto';
import { Directory, File, Paths } from 'expo-file-system';
import type { DraftPhoto } from './photos';
import {
  checkJpeg, checkPhotoBytes, copyValue, DraftConflictError, draftInfo, invalidDraft, UUID,
  validateIdentity, validateMetadata, validateRevision,
} from './draft-store.types';
import type { DraftInfo, DraftMetadata, DraftValue, PhotoMetadata, StoredDraft } from './draft-store.types';

export { DraftConflictError } from './draft-store.types';
export type { DraftInfo, DraftValue, PendingDayDeletion, PendingMessage, StoredDraft } from './draft-store.types';

type NativeDraft = Omit<DraftMetadata, 'photos'> & { photos: (PhotoMetadata & { path: string })[] };
const PREFIX = 'fitty:draft:v1:';
const locks = new Map<string, Promise<unknown>>();
const keyFor = (userId: string, date: string) => `${PREFIX}${userId}:${date}`;
const folderFor = (userId: string, date: string) => `fitty-drafts/${userId}/${date}`;

function serial<T>(key: string, action: () => Promise<T>): Promise<T> {
  const running = (locks.get(key) ?? Promise.resolve()).catch(() => {}).then(action);
  locks.set(key, running);
  void running.then(() => { if (locks.get(key) === running) locks.delete(key); }, () => { if (locks.get(key) === running) locks.delete(key); });
  return running;
}

function validateFile(file: File): void {
  if (!file.exists) throw new Error('Ein gespeichertes Entwurfsfoto fehlt. Der Entwurf wurde nicht überschrieben.');
  checkPhotoBytes(file.size);
  const handle = file.open();
  try { checkJpeg(handle.readBytes(3)); } finally { handle.close(); }
}

function decode(raw: string, userId: string, date: string): NativeDraft {
  let value: unknown;
  try { value = JSON.parse(raw); } catch { invalidDraft(); }
  validateMetadata(value, userId, date);
  for (const photo of value.photos) {
    const path = (photo as PhotoMetadata & { path?: unknown }).path;
    const prefix = `${folderFor(userId, date)}/${photo.id}-`;
    if (typeof path !== 'string' || !path.startsWith(prefix) || !path.endsWith('.jpg') || !UUID.test(path.slice(prefix.length, -4))) invalidDraft();
    validateFile(new File(Paths.document, path));
  }
  return value as NativeDraft;
}

function materialize(value: NativeDraft): StoredDraft {
  return { text: value.text, pending: value.pending, deletion: value.deletion, revision: value.revision, updatedAt: value.updatedAt,
    photos: value.photos.map(photo => ({ id: photo.id, width: photo.width, height: photo.height, uri: new File(Paths.document, photo.path).uri })) };
}

export async function readDraft(userId: string, date: string): Promise<StoredDraft | null> {
  validateIdentity(userId, date);
  const key = keyFor(userId, date);
  return serial(key, async () => {
    const raw = await AsyncStorage.getItem(key);
    return raw === null ? null : materialize(decode(raw, userId, date));
  });
}

export async function listDrafts(userId: string): Promise<DraftInfo[]> {
  validateIdentity(userId);
  const prefix = `${PREFIX}${userId}:`;
  const keys = (await AsyncStorage.getAllKeys()).filter(key => key.startsWith(prefix));
  const result: DraftInfo[] = [];
  for (const key of keys) {
    const date = key.slice(prefix.length); validateIdentity(userId, date);
    const info = await serial(key, async () => {
      const raw = await AsyncStorage.getItem(key);
      return raw === null ? null : draftInfo(decode(raw, userId, date));
    });
    if (info) result.push(info);
  }
  return result.sort((a, b) => b.updatedAt - a.updatedAt || b.date.localeCompare(a.date));
}

export async function writeDraft(userId: string, date: string, expectedRevision: string | null, value: DraftValue): Promise<StoredDraft> {
  validateIdentity(userId, date); validateRevision(expectedRevision);
  const snapshot = copyValue(value);
  const key = keyFor(userId, date);
  return serial(key, async () => {
    const raw = await AsyncStorage.getItem(key);
    const current = raw === null ? null : decode(raw, userId, date);
    if ((current?.revision ?? null) !== expectedRevision) throw new DraftConflictError();
    const revision = randomUUID();
    const directory = new Directory(Paths.document, folderFor(userId, date));
    const photos: NativeDraft['photos'] = [];
    if (snapshot.photos.length) directory.create({ intermediates: true, idempotent: true });
    for (const photo of snapshot.photos) {
      const previous = current?.photos.find(item => item.id === photo.id && new File(Paths.document, item.path).uri === photo.uri);
      if (previous) { photos.push({ ...previous, width: photo.width, height: photo.height }); continue; }
      const source = new File(photo.uri);
      if (!source.uri.startsWith(`${Paths.cache.uri.replace(/\/$/, '')}/`) && !source.uri.startsWith(`${directory.uri.replace(/\/$/, '')}/`)) invalidDraft();
      validateFile(source);
      const path = `${folderFor(userId, date)}/${photo.id}-${revision}.jpg`;
      const destination = new File(Paths.document, path);
      await source.copy(destination);
      validateFile(destination);
      photos.push({ id: photo.id, width: photo.width, height: photo.height, path });
    }
    const next: NativeDraft = { schema: 1, userId, date, text: snapshot.text, pending: snapshot.pending, deletion: snapshot.deletion, photos, revision, updatedAt: Date.now() };
    // A rejected metadata commit can be ambiguous: retain both old and newly staged files.
    await AsyncStorage.setItem(key, JSON.stringify(next));
    cleanupCommittedFiles(directory, photos);
    return materialize(next);
  });
}

function cleanupCommittedFiles(directory: Directory, photos: NativeDraft['photos']): void {
  const kept = new Set(photos.map(photo => new File(Paths.document, photo.path).uri));
  try {
    if (!directory.exists) return;
    for (const file of directory.list()) {
      if (!(file instanceof File) || kept.has(file.uri)) continue;
      const name = file.name;
      if (name.length !== 77 || name[36] !== '-' || !name.endsWith('.jpg') || !UUID.test(name.slice(0, 36)) || !UUID.test(name.slice(37, -4))) continue;
      try { file.delete(); } catch { /* Retry stale-file cleanup after the next successful commit. */ }
    }
  } catch { /* Metadata is committed; cleanup must not report a failed save. */ }
}

export function releasePhotos(photos: DraftPhoto[]): void {
  const cache = `${Paths.cache.uri.replace(/\/$/, '')}/`;
  for (const photo of photos) {
    try { const file = new File(photo.uri); if (file.uri.startsWith(cache) && file.exists) file.delete(); }
    catch { /* Only temporary cache files are released; the OS also maintains this directory. */ }
  }
}
