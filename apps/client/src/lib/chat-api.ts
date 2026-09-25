import { supabase } from './supabase';

export type DailyTargets = { calories: number | null; protein_g: number | null; carbs_g: number | null; fat_g: number | null };
export type TargetSettings = { version: number; today: string; effective_from: string | null; targets: DailyTargets };

export type Profile = { display_name: string; time_zone: string; goals: string; preferences: string };
export type AnalysisStatus = 'saved' | 'queued' | 'processing' | 'completed' | 'failed' | 'disabled' | 'skipped';
export type Attachment = { id: string; mime_type: string; byte_size: number; width: number; height: number };
export type Message = { id: string; client_id: string; role: 'user' | 'assistant'; content: string; created_at: string; analysis_status: AnalysisStatus; analysis_error: string; analysis_attempts: number; attachments: Attachment[] };
export type Entry = {
  id: string; version: number; kind: 'food' | 'activity'; label: string; amount: string;
  calories: number | null; protein_g: number | null; carbs_g: number | null; fat_g: number | null;
  duration_minutes: number | null; distance_km: number | null; source: 'estimate' | 'user' | 'device'; notes: string;
};
export type EntryValues = Omit<Entry, 'id' | 'version'>;
export type DaySnapshot = { id: string; version: number; message_count: number; entry_count: number; photo_count: number };
export type DayDeletionPreview = { day: DaySnapshot | null; pending_photo_deletions: number };
export type DayDeletionResult = { status: 'deleted'; pending_photo_deletions: number };
export type Summary = DayDeletionPreview & {
  targets: DailyTargets; targets_effective_from: string | null;
  entries: Entry[]; ai_enabled: boolean; photos_enabled: boolean; pending_count: number;
  totals: { calories: number; protein_g: number; carbs_g: number; fat_g: number; activity_calories: number; duration_minutes: number; distance_km: number; estimated_entries: number; unknown_activity_calories: number };
  analysis: { message_id: string; status: AnalysisStatus; error: string; attempts: number }[];
};
export type Day = { date: string; message_count: number; preview: string };
export type MessagePage = { date: string; day_id: string | null; messages: Message[]; next_before: string | null };
export type DayPage = { days: Day[]; next_before: string | null };

export class APIError extends Error {
  constructor(message: string, public readonly status: number) { super(message); }
}

export async function api<T>(path: string, options: { body?: unknown; rawBody?: ArrayBuffer; method?: string; signal?: AbortSignal; timeout?: number; userId?: string } = {}): Promise<T> {
  const { data } = await supabase!.auth.getSession();
  if (!data.session || (options.userId && data.session.user.id !== options.userId)) throw new Error('Bitte erneut mit dem zugehörigen Konto anmelden.');
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener('abort', abort, { once: true });
  if (options.signal?.aborted) controller.abort();
  const timeout = setTimeout(abort, options.timeout ?? 12000);
  try {
    const response = await fetch(`${process.env.EXPO_PUBLIC_API_URL ?? 'http://127.0.0.1:8787'}${path}`, {
      method: options.method ?? 'GET',
      headers: { Authorization: `Bearer ${data.session.access_token}`, 'Content-Type': options.rawBody ? 'image/jpeg' : 'application/json' },
      ...(options.rawBody ? { body: options.rawBody } : options.body !== undefined ? { body: JSON.stringify(options.body) } : {}),
      signal: controller.signal,
    });
    const value = await response.json();
    if (!response.ok) throw new APIError(typeof value.error === 'string' ? value.error : 'Die Anfrage ist fehlgeschlagen.', response.status);
    return value as T;
  } catch (error) {
    if (error instanceof TypeError || (error instanceof Error && error.name === 'AbortError')) throw new Error('Keine Verbindung zu Fitty. Bitte erneut versuchen.');
    throw error;
  } finally {
    clearTimeout(timeout);
    options.signal?.removeEventListener('abort', abort);
  }
}

export function todayIn(timeZone: string) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date());
  const part = (type: string) => parts.find(p => p.type === type)?.value;
  return `${part('year')}-${part('month')}-${part('day')}`;
}
export function dateLabel(date: string, full = false) {
  return new Intl.DateTimeFormat('de-DE', { timeZone: 'UTC', day: 'numeric', month: full ? 'long' : 'short', ...(full ? { weekday: 'long', year: 'numeric' } as const : {}) }).format(new Date(`${date}T12:00:00Z`));
}
export function moveDate(date: string, offset: number) {
  const value = new Date(`${date}T12:00:00Z`);
  value.setUTCDate(value.getUTCDate() + offset);
  return value.toISOString().slice(0, 10);
}
export const errorMessage = (error: unknown) => error instanceof Error ? error.message : 'Etwas ist schiefgelaufen. Bitte erneut versuchen.';
