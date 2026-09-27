import type { Entry, EntryValues } from './chat-api';

export type Favorite = { id: string; entry: EntryValues };
export type FavoriteMatch = { favorite: Favorite; start: number; end: number };
const normalize = (text: string) => text.normalize('NFD').replace(/\p{M}/gu, '').toLocaleLowerCase('de-DE').replace(/ß/g, 'ss');
const number = (value: number) => value.toLocaleString('de-DE', { maximumFractionDigits: 2 });

export function favoriteForEntry(favorites: Favorite[], entry: Entry) {
  return favorites.find(favorite => favorite.id === entry.id ||
    Object.keys(favorite.entry).every(key => favorite.entry[key as keyof EntryValues] === entry[key as keyof EntryValues]));
}

export function favoriteMatches(favorites: Favorite[], text: string, selection: { start: number; end: number }): FavoriteMatch[] {
  const end = selection.end;
  if (selection.start !== end || end < 0 || end > text.length || /[\p{L}\p{N}]/u.test(text[end] ?? '')) return [];
  // Only complete the phrase at the caret, preserving the surrounding draft.
  const prefix = text.slice(0, end).match(/[\p{L}\p{N}][\p{L}\p{N}\p{M}'’ -]*$/u)?.[0];
  if (!prefix) return [];
  const words = [...prefix.matchAll(/[\p{L}\p{N}][\p{L}\p{N}\p{M}'’-]*/gu)].slice(-6);
  const matches: FavoriteMatch[] = [];
  for (const favorite of favorites) {
    const label = normalize(favorite.entry.label);
    for (const word of words) {
      const query = normalize(prefix.slice(word.index).trim());
      if (query.length < 2) continue;
      if (label.startsWith(query) || label.includes(` ${query}`)) {
        matches.push({ favorite, start: end - prefix.length + word.index, end });
        break;
      }
    }
  }
  return matches.sort((a, b) => a.start - b.start).slice(0, 5);
}

export function favoriteText({ entry }: Favorite): string {
  const values = [
    entry.amount,
    entry.calories === null ? 'Kalorien unbekannt' : `${number(entry.calories)} kcal`,
    entry.protein_g === null ? 'Protein unbekannt' : `${number(entry.protein_g)} g Protein`,
    entry.carbs_g === null ? 'Kohlenhydrate unbekannt' : `${number(entry.carbs_g)} g Kohlenhydrate`,
    entry.fat_g === null ? 'Fett unbekannt' : `${number(entry.fat_g)} g Fett`,
    { estimate: 'gespeicherte Schätzung', user: 'eigene Angaben', device: 'Geräteangaben' }[entry.source],
    entry.notes,
  ].filter(Boolean);
  return `${entry.label} (${values.join('; ')})`;
}

export function insertFavorite(text: string, favorite: Favorite, match?: FavoriteMatch): string {
  const value = favoriteText(favorite);
  return match ? `${text.slice(0, match.start)}${value}${text.slice(match.end)}` : `${text}${text ? '\n' : ''}${value}`;
}
