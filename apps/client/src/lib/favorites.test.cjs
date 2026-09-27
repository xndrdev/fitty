const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');
const ts = require('typescript');
const exportsForFavorites = {};
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, 'favorites.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText, { exports: exportsForFavorites });
const { favoriteMatches, favoriteText, insertFavorite, favoriteForEntry } = exportsForFavorites;
const favorite = { id: '4', entry: { kind: 'food', label: 'Haferflocken mit Obst', amount: '1 Schüssel', calories: 450,
  protein_g: 20, carbs_g: 60, fat_g: 14, duration_minutes: null, distance_km: null, source: 'estimate', notes: 'Mit Apfel' } };
const find = (text, end = text.length) => favoriteMatches([favorite], text, {start: end, end});

test('completes the longest matching phrase without replacing earlier or later text', () => {
  const text = 'Frühstück: Haferfl mit Kaffee';
  const match = find(text, 'Frühstück: Haferfl'.length)[0];
  assert.equal(insertFavorite(text, favorite, match), `Frühstück: ${favoriteText(favorite)} mit Kaffee`);
  const sentence = 'Ich habe Haferflocken mit';
  assert.equal(find(sentence)[0].start, 'Ich habe '.length);
});
test('matches case and accents, including later words of meal names', () => {
  assert.equal(find('Heute OB')[0].favorite.id, '4');
  const meal = {...favorite, entry: {...favorite.entry, label: 'Crème brûlée'}};
  assert.equal(favoriteMatches([meal], 'creme br', {start: 8, end: 8})[0].start, 0);
});
test('does not offer favorites for an empty, short, selected, or unfinished middle word', () => {
  for (const text of ['', 'h', 'Heute eine Banane.', 'Haferflocken\n']) assert.equal(find(text).length, 0);
  assert.equal(find('Haferflocken', 3).length, 0);
  assert.equal(favoriteMatches([favorite], 'Hafer', {start: 0, end: 5}).length, 0);
});
test('suggestions are bounded and favor more specific matching phrases', () => {
  const many = Array.from({length: 10}, (_, i) => ({...favorite, id: String(i)}));
  assert.equal(favoriteMatches(many, 'haf', {start: 3, end: 3}).length, 5);
  const specific = {...favorite, id: 'specific', entry: {...favorite.entry, label: 'Heute Haferflocken'}};
  assert.equal(favoriteMatches([favorite, specific], 'Heute haf', {start: 9, end: 9})[0].favorite.id, 'specific');
});
test('inserted portion preserves values, unknown versus zero, notes and provenance', () => {
  const value = favoriteText({...favorite, entry: {...favorite.entry, calories: 0, protein_g: null, carbs_g: 12.5}});
  for (const text of ['1 Schüssel', '0 kcal', 'Protein unbekannt', '12,5 g Kohlenhydrate', 'gespeicherte Schätzung', 'Mit Apfel']) assert(value.includes(text), text);
  assert.equal(insertFavorite('Schon vorhandene Notiz', favorite), `Schon vorhandene Notiz\n${favoriteText(favorite)}`);
});
test('the source and identical portions share a bookmark; different portions remain distinct', () => {
  const entry = {...favorite.entry, id: '99', version: 1};
  assert.equal(favoriteForEntry([favorite], entry).id, '4');
  assert.equal(favoriteForEntry([favorite], {...entry, amount: '2 Schüsseln'}), undefined);
  assert.equal(favoriteForEntry([favorite], {...entry, id: '4', amount: '2 Schüsseln'}).id, '4');
});
