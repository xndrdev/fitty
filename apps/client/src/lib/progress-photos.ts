export const progressViews = [
  { value: 'front', label: 'Vorne' },
  { value: 'side', label: 'Seitlich' },
  { value: 'back', label: 'Hinten' },
  { value: 'other', label: 'Sonstige' },
] as const;

export type ProgressView = typeof progressViews[number]['value'];
export type ProgressPhoto = {
  id: string; date: string; view: ProgressView; mime_type: 'image/jpeg'; byte_size: number; width: number; height: number;
};
export type ProgressPhotoPage = { photos: ProgressPhoto[]; next_before: string | null; photos_enabled: boolean; pending_deletions: number };

export function progressViewLabel(value: ProgressView) {
  return progressViews.find(view => view.value === value)!.label;
}

export function progressDateLabel(date: string) {
  return new Intl.DateTimeFormat('de-DE', { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(`${date}T12:00:00Z`));
}

export function progressDateError(date: string, today: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return 'Bitte das Aufnahmedatum als JJJJ-MM-TT eingeben.';
  const parsed = new Date(`${date}T12:00:00Z`);
  if (Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== date) return 'Bitte ein gültiges Aufnahmedatum eingeben.';
  if (date < '1900-01-01') return 'Bitte ein Aufnahmedatum ab 1900 eingeben.';
  if (date > today) return 'Das Aufnahmedatum darf nicht in der Zukunft liegen.';
  return '';
}

export function mergeProgressPhotos(current: ProgressPhoto[], next: ProgressPhoto[]) {
  const ids = new Set(current.map(photo => photo.id));
  return [...current, ...next.filter(photo => !ids.has(photo.id))];
}
