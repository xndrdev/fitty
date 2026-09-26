import { useEffect, useRef, useState } from 'react';
import { Platform } from 'react-native';
import { listDrafts, readDraft, releasePhotos, writeDraft } from '../lib/draft-store';
import { DraftConflictError, type DraftInfo, type DraftValue, type PendingDayDeletion, type PendingMessage, type StoredDraft } from '../lib/draft-store.types';
import type { DraftPhoto } from '../lib/photos';

const empty = (): DraftValue => ({ text: '', photos: [], pending: null, deletion: null });
type DayDraft = {
  value: DraftValue; revision: string | null; loaded: boolean; error: string; conflict: boolean;
  edits: number; saved: number; task: Promise<void> | null; committing: Promise<StoredDraft> | null; loading: Promise<void> | null; writing: DraftPhoto[];
};
const newDay = (): DayDraft => ({ value: empty(), revision: null, loaded: false, error: '', conflict: false, edits: 0, saved: 0, task: null, committing: null, loading: null, writing: [] });

export function useDrafts(userId: string) {
  const days = useRef(new Map<string, DayDraft>());
  const resources = useRef(new Map<string, DraftPhoto>());
  const active = useRef(true);
  const [, render] = useState(0);
  const [infos, setInfos] = useState<DraftInfo[]>([]);
  const [listError, setListError] = useState('');
  const listGeneration = useRef(0);
  function emit() { if (active.current) render(n => n + 1); }
  function day(date: string) { let value = days.current.get(date); if (!value) { value = newDay(); days.current.set(date, value); } return value; }
  function remember(photos: DraftPhoto[]) { photos.forEach(photo => resources.current.set(photo.uri, photo)); }
  function collect() {
    const used = new Set([...days.current.values()].flatMap(item => [...item.value.photos, ...item.writing]).map(photo => photo.uri));
    for (const [uri, photo] of resources.current) if (!used.has(uri)) { releasePhotos([photo]); resources.current.delete(uri); }
  }
  async function refreshList() {
    const generation = ++listGeneration.current;
    try { const result = await listDrafts(userId); if (active.current && generation === listGeneration.current) { setInfos(result); setListError(''); } }
    catch { if (active.current && generation === listGeneration.current) setListError('Deine gespeicherten Entwürfe konnten nicht geladen werden. Bitte erneut versuchen.'); }
  }
  function report(item: DayDraft, error: unknown) {
    item.conflict = error instanceof DraftConflictError;
    item.error = item.conflict ? 'Der gespeicherte Entwurf wurde inzwischen geändert. Dein Text hier bleibt erhalten. Bitte wähle, mit welchem Entwurf du weiterarbeiten möchtest.'
      : 'Der Entwurf konnte auf diesem Gerät nicht gespeichert werden. Bitte die App geöffnet lassen und erneut versuchen.';
    emit();
  }
  async function load(date: string) {
    const item = day(date);
    if (item.loaded) return;
    if (item.loading) return item.loading;
    const task = (async () => {
      try {
        const stored = await readDraft(userId, date);
        item.value = stored ?? empty(); item.revision = stored?.revision ?? null; item.loaded = true; item.error = ''; remember(item.value.photos);
      } catch { item.error = 'Der Entwurf konnte nicht gelesen werden. Bitte erneut laden; gespeicherte Daten werden nicht überschrieben.'; }
      finally { item.loading = null; emit(); }
    })();
    item.loading = task; emit(); return task;
  }
  async function flush(date: string, insideCommit = false): Promise<void> {
    const item = day(date);
    if (item.committing && !insideCommit) await item.committing;
    await load(date);
    if (!item.loaded || item.conflict) throw new Error(item.error);
    if (item.task) return item.task;
    if (item.saved === item.edits && !item.error) return;
    if (item.saved === item.edits) item.edits++;
    item.error = '';
    const task = (async () => {
      try {
        while (item.saved !== item.edits) {
          const edits = item.edits, value = item.value;
          item.writing = value.photos;
          const stored = await writeDraft(userId, date, item.revision, value);
          item.revision = stored.revision; item.saved = edits; remember(stored.photos);
          // Native copies receive durable document URIs. Preserve text/photo
          // changes that were made while this older snapshot was being saved.
          item.value = { ...item.value, photos: item.value.photos.map(photo => stored.photos.find(saved => saved.id === photo.id && value.photos.some(old => old.id === photo.id && old.uri === photo.uri)) ?? photo) };
        }
        void refreshList();
      } catch (error) { report(item, error); throw error; }
      finally { item.task = null; item.writing = []; collect(); emit(); }
    })();
    item.task = task; emit(); return task;
  }
  function edit(date: string, patch: Partial<DraftValue>) {
    const item = day(date);
    if (!item.loaded || item.committing || item.value.pending || item.value.deletion) return;
    item.value = { ...item.value, ...patch }; item.edits++; remember(item.value.photos); emit();
    if (!item.conflict) void flush(date).catch(() => {});
  }
  // Durable transitions happen before uploads/requests, or before the UI is
  // cleared after acknowledgement. A storage failure retains the retry body.
  function commit(date: string, transform: (value: DraftValue) => DraftValue, discardDirty = false): Promise<StoredDraft> {
    const item = day(date), previous = item.committing;
    const task = (async () => {
      // Reserve the transition synchronously; two button events cannot both
      // pass the first await and write against the same local revision.
      if (previous) { try { await previous; } catch {} }
      if (discardDirty) {
        await load(date);
        if (item.task) { try { await item.task; } catch {} }
        if (!item.loaded || item.conflict) throw new Error(item.error);
      } else await flush(date, true);
      const next = transform(item.value); item.writing = next.photos;
      const stored = await writeDraft(userId, date, item.revision, next);
      item.value = stored; item.revision = stored.revision; item.saved = item.edits;
      item.error = ''; item.conflict = false; remember(stored.photos); void refreshList(); return stored;
    })().catch(error => { report(item, error); throw error; }).finally(() => {
      if (item.committing === task) item.committing = null;
      item.writing = []; collect(); emit();
    });
    item.committing = task; emit(); return task;
  }
  async function complete(date: string) {
    try { await commit(date, empty); }
    catch (error) {
      if (!(error instanceof DraftConflictError)) throw error;
      // Another tab may have acknowledged this same request and already
      // started a new draft. Keep that newer draft instead of clearing it.
      const stored = await readDraft(userId, date), item = day(date);
      item.value = stored ?? empty(); item.revision = stored?.revision ?? null;
      item.error = ''; item.conflict = false; item.saved = item.edits;
      remember(item.value.photos); collect(); emit(); void refreshList();
    }
  }

  async function reload(date: string) {
    const item = day(date);
    if (item.task) { try { await item.task; } catch {} }
    try {
      const stored = await readDraft(userId, date);
      item.value = stored ?? empty(); item.revision = stored?.revision ?? null; item.loaded = true; item.edits = 0; item.saved = 0; item.error = ''; item.conflict = false; remember(item.value.photos); collect(); emit(); void refreshList();
    } catch { item.error = 'Der gespeicherte Entwurf konnte nicht geladen werden. Dein aktueller Text bleibt erhalten.'; emit(); }
  }
  async function keepLocal(date: string) {
    const item = day(date);
    try {
      const stored = await readDraft(userId, date);
      try {
        if (stored?.pending || stored?.deletion) { item.error = 'Für diesen Tag ist noch ein Sende- oder Löschvorgang offen. Bitte den gespeicherten Entwurf übernehmen und den Vorgang abschließen.'; emit(); return; }
        item.revision = stored?.revision ?? null;
      } finally { if (stored) releasePhotos(stored.photos); }
      item.conflict = false; item.edits++; await flush(date);
    } catch (error) { report(item, error); }
  }
  useEffect(() => {
    active.current = true; void refreshList();
    const timer = setInterval(() => void refreshList(), 5000);
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if ([...days.current.values()].some(item => item.edits !== item.saved || item.task || item.committing)) { event.preventDefault(); event.returnValue = ''; }
    };
    if (Platform.OS === 'web' && typeof window !== 'undefined') window.addEventListener('beforeunload', beforeUnload);
    return () => {
      active.current = false; clearInterval(timer);
      if (Platform.OS === 'web' && typeof window !== 'undefined') window.removeEventListener('beforeunload', beforeUnload);
      void Promise.allSettled([...days.current.values()].flatMap(item => [item.task, item.committing, item.loading].filter(Boolean))).then(() => {
        if (!active.current) { releasePhotos([...resources.current.values()]); resources.current.clear(); }
      });
    };
  }, [userId]);
  return {
    flushAll: () => Promise.all([...days.current.entries()].filter(([, item]) => item.loaded && item.saved !== item.edits).map(([date]) => flush(date))),
    infos, listError, refreshList, load, get: (date: string) => day(date),
    setText: (date: string, text: string) => edit(date, { text }),
    setPhotos: (date: string, photos: DraftPhoto[]) => edit(date, { photos }),
    flush, reload, keepLocal,
    prepareSend: (date: string, pending: PendingMessage) => commit(date, value => ({ ...value, pending: value.pending ?? pending })),
    releaseSend: (date: string) => commit(date, value => ({ ...value, pending: null })),
    prepareDeletion: (date: string, deletion: PendingDayDeletion) => commit(date, value => ({ ...value, deletion })),
    releaseDeletion: (date: string) => commit(date, value => ({ ...value, deletion: null })),
    complete,
    clear: (date: string) => commit(date, empty, true),
  };
}
