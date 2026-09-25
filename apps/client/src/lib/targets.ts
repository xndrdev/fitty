import type { DailyTargets } from './chat-api';

export type TargetKey = keyof DailyTargets;
export type TargetDraft = Record<TargetKey, string>;
export type TargetErrors = Partial<Record<TargetKey, string>>;
export const targetFields: { key: TargetKey; label: string; unit: string; max: number }[] = [
  { key: 'calories', label: 'Kalorien', unit: 'kcal', max: 20000 },
  { key: 'protein_g', label: 'Protein', unit: 'g', max: 2000 },
  { key: 'carbs_g', label: 'Kohlenhydrate', unit: 'g', max: 2000 },
  { key: 'fat_g', label: 'Fett', unit: 'g', max: 2000 },
];
export const emptyTargets = (): DailyTargets => ({ calories: null, protein_g: null, carbs_g: null, fat_g: null });

export function targetDraft(targets: DailyTargets): TargetDraft {
  const draft: TargetDraft = { calories: '', protein_g: '', carbs_g: '', fat_g: '' };
  for (const { key } of targetFields) draft[key] = targets[key] === null ? '' : String(targets[key]).replace('.', ',');
  return draft;
}

export function validateTargets(draft: TargetDraft): { targets: DailyTargets; errors: TargetErrors } {
  const targets = emptyTargets();
  const errors: TargetErrors = {};
  for (const { key, max } of targetFields) {
    const text = draft[key].trim();
    if (!text) continue;
    if (!/^\d+(?:[.,]\d{1,2})?$/.test(text)) {
      errors[key] = 'Bitte eine positive Zahl mit höchstens zwei Nachkommastellen eingeben, ohne Tausendertrennzeichen.';
      continue;
    }
    const value = Number(text.replace(',', '.'));
    if (!Number.isFinite(value) || value <= 0 || value > max) {
      errors[key] = `Bitte einen Wert größer als 0 und höchstens ${max.toLocaleString('de-DE')} eingeben. Ohne Ziel das Feld leer lassen.`;
      continue;
    }
    targets[key] = value;
  }
  return { targets, errors };
}

export function targetProgress(actual: number, target: number) {
  const difference = Math.round((target - actual) * 100) / 100;
  return { percent: Math.max(0, Math.min(100, actual / target * 100)), remaining: Math.max(0, difference), above: Math.max(0, -difference) };
}
