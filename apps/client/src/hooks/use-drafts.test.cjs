const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

function loadHook(platform, window, writeDraft = async () => {}) {
  const effects = [];
  const timers = new Set();
  const filename = path.join(__dirname, 'use-drafts.ts');
  const source = ts.transpileModule(readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports = {};
  vm.runInNewContext(source, {
    exports, window,
    setInterval(callback) { timers.add(callback); return callback; },
    clearInterval(callback) { timers.delete(callback); },
    require(id) {
      if (id === 'react') return {
        useRef: current => ({ current }),
        useState: initial => [initial, () => {}],
        useEffect: effect => effects.push(effect),
      };
      if (id === 'react-native') return { Platform: { OS: platform } };
      if (id === '../lib/draft-store') return {
        listDrafts: async () => [], readDraft: async () => null,
        releasePhotos() {}, writeDraft,
      };
      if (id === '../lib/draft-store.types') return { DraftConflictError: class extends Error {} };
      throw new Error(`Unexpected dependency: ${id}`);
    },
  }, { filename });
  const drafts = exports.useDrafts('11111111-1111-4111-8111-111111111111');
  return { drafts, timers, mount: () => effects[0]() };
}

test('iOS draft lifecycle works when window exists without browser event methods', async () => {
  const { drafts, mount, timers } = loadHook('ios', {});
  const unmount = mount();
  assert.equal(timers.size, 1);
  await drafts.load('2032-01-10');
  assert.equal(drafts.get('2032-01-10').loaded, true);
  assert.doesNotThrow(unmount);
  assert.equal(timers.size, 0);
});

test('web warns during unsaved draft writes and removes the listener on unmount', async () => {
  const listeners = new Map();
  const window = {
    addEventListener(type, listener) { listeners.set(type, listener); },
    removeEventListener(type, listener) {
      assert.equal(listeners.get(type), listener);
      listeners.delete(type);
    },
  };
  let finishWrite;
  const pendingWrite = new Promise(resolve => { finishWrite = resolve; });
  const { drafts, mount, timers } = loadHook('web', window, async (_user, _date, _revision, value) => {
    await pendingWrite;
    return { ...value, revision: 'saved' };
  });
  const unmount = mount();
  const beforeUnload = listeners.get('beforeunload');
  assert.equal(typeof beforeUnload, 'function');
  const event = () => ({ prevented: false, returnValue: undefined, preventDefault() { this.prevented = true; } });
  const clean = event();
  beforeUnload(clean);
  assert.equal(clean.prevented, false);

  const date = '2032-01-10';
  await drafts.load(date);
  drafts.setText(date, 'Unsent meal');
  const dirty = event();
  beforeUnload(dirty);
  assert.equal(dirty.prevented, true);
  assert.equal(dirty.returnValue, '');

  finishWrite();
  await drafts.flush(date);
  const saved = event();
  beforeUnload(saved);
  assert.equal(saved.prevented, false);
  unmount();
  assert.equal(listeners.size, 0);
  assert.equal(timers.size, 0);
});
