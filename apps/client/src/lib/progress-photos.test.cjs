const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');
const ts = require('typescript');

const progress = {};
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, 'progress-photos.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText, { exports: progress });

test('capture dates require real calendar dates and cannot exceed the profile date', () => {
  for (const invalid of ['', '17.09.2026', '2026-9-17', '0000-01-01', '1899-12-31', '2026-02-29', '2026-04-31', '2026-13-01', '2026-09-18']) {
    assert.ok(progress.progressDateError(invalid, '2026-09-17'), invalid);
  }
  for (const valid of ['1900-01-01', '2024-02-29', '2026-09-17', '2026-01-01']) {
    assert.equal(progress.progressDateError(valid, '2026-09-17'), '', valid);
  }
  assert.equal(progress.progressDateError('2026-09-18', '2026-09-18'), '');
});

test('paginated history keeps selected photo objects and deduplicates an overlapping page', () => {
  const first = { id: 'a', date: '2026-09-17', view: 'front' };
  const second = { id: 'b', date: '2026-09-16', view: 'side' };
  const merged = progress.mergeProgressPhotos([first], [{ ...first }, second]);
  assert.equal(merged.length, 2);
  assert.equal(merged[0], first);
  assert.equal(merged[1], second);
  assert.equal(progress.progressViewLabel(second.view), 'Seitlich');
});
