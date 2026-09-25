const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');
const ts = require('typescript');

const exportsForTargets = {};
const source = ts.transpileModule(readFileSync(path.join(__dirname, 'targets.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
vm.runInNewContext(source, { exports: exportsForTargets });
const { emptyTargets, targetDraft, validateTargets, targetProgress } = exportsForTargets;
const plain = value => JSON.parse(JSON.stringify(value));

test('optional targets stay absent and comma decimals round-trip without deriving other goals', () => {
  const blank = targetDraft(emptyTargets());
  assert.deepEqual(plain(validateTargets(blank)), { targets: { calories: null, protein_g: null, carbs_g: null, fat_g: null }, errors: {} });
  const result = validateTargets({ ...blank, calories: ' 2345,67 ', protein_g: '120.25' });
  assert.deepEqual(plain(result), { targets: { calories: 2345.67, protein_g: 120.25, carbs_g: null, fat_g: null }, errors: {} });
  assert.equal(targetDraft(result.targets).calories, '2345,67');
});

test('rejects zero, signs, exponents, grouping, excessive precision and each upper boundary', () => {
  const blank = targetDraft(emptyTargets());
  for (const value of ['0', '0.00', '-1', '+1', '1e3', '2 000', '2.000,00', '1,234', '.5', 'NaN', 'Infinity', '20000.01']) {
    assert.ok(validateTargets({ ...blank, calories: value }).errors.calories, value);
  }
  for (const key of ['protein_g', 'carbs_g', 'fat_g']) {
    assert.ok(validateTargets({ ...blank, [key]: '2000,01' }).errors[key]);
    assert.equal(validateTargets({ ...blank, [key]: '2000' }).targets[key], 2000);
  }
  assert.equal(validateTargets({ ...blank, calories: '20000' }).targets.calories, 20000);
  assert.equal(validateTargets({ ...blank, calories: '0,01' }).targets.calories, 0.01);
});

test('progress caps its bar, retains overshoot and avoids negative remaining amounts', () => {
  assert.deepEqual(plain(targetProgress(3000, 2000)), { percent: 100, remaining: 0, above: 1000 });
  assert.deepEqual(plain(targetProgress(500, 2000)), { percent: 25, remaining: 1500, above: 0 });
  assert.deepEqual(plain(targetProgress(0, 2000)), { percent: 0, remaining: 2000, above: 0 });
  assert.deepEqual(plain(targetProgress(0.1 + 0.2, 0.3)), { percent: 100, remaining: 0, above: 0 });
});
