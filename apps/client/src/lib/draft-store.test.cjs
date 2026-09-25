const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');
const vm = require('node:vm');
const { test } = require('node:test');
const ts = require('typescript');

const user = '11111111-1111-4111-8111-111111111111';
const otherUser = '22222222-2222-4222-8222-222222222222';
const date = '2032-01-10';
const empty = () => ({ text: '', photos: [], pending: null, deletion: null });

function nativeStore() {
  const values = new Map();
  const files = new Map();
  let documentUri = 'file:///sandbox-a/documents';
  const failures = { metadataBefore: false, metadataAfter: false, copy: false };
  const uriFor = parts => parts.map(part => typeof part === 'string' ? part : part.uri).map((part, i) => i === 0 ? part.replace(/\/$/, '') : part.replace(/^\/+|\/+$/g, '')).join('/');
  class File {
    constructor(...parts) { this.uri = uriFor(parts); }
    get exists() { return files.has(this.uri); }
    get size() { return files.get(this.uri)?.length ?? 0; }
    get name() { return this.uri.split('/').at(-1); }
    open() { return { readBytes: length => files.get(this.uri).slice(0, length), close() {} }; }
    async copy(destination) {
      if (failures.copy) throw new Error('copy failed');
      files.set(destination.uri, Buffer.from(files.get(this.uri)));
    }
    delete() { files.delete(this.uri); }
  }
  class Directory {
    constructor(...parts) { this.uri = uriFor(parts); }
    get exists() { return [...files.keys()].some(uri => uri.startsWith(`${this.uri}/`)); }
    create() {}
    list() { return [...files.keys()].filter(uri => uri.startsWith(`${this.uri}/`)).map(uri => new File(uri)); }
  }
  const storage = {
    async getItem(key) { return values.get(key) ?? null; },
    async setItem(key, value) {
      if (failures.metadataBefore) throw new Error('metadata failed');
      values.set(key, value);
      if (failures.metadataAfter) throw new Error('metadata failed after commit');
    },
    async getAllKeys() { return [...values.keys()]; },
  };
  const Paths = { get document() { return new Directory(documentUri); }, get cache() { return new Directory('file:///cache'); } };
  const modules = new Map();
  function load(name) {
    if (modules.has(name)) return modules.get(name);
    const filename = path.join(__dirname, `${name}.ts`);
    const source = ts.transpileModule(readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
    const exports = {};
    modules.set(name, exports);
    vm.runInNewContext(source, {
      exports,
      require(id) {
        if (id === 'expo-crypto') return { randomUUID };
        if (id === 'expo-file-system') return { File, Directory, Paths };
        if (id === '@react-native-async-storage/async-storage') return { __esModule: true, default: storage };
        if (id === './draft-store.types') return load('draft-store.types');
        throw new Error(`Unexpected dependency: ${id}`);
      },
    }, { filename });
    return exports;
  }
  return {
    store: load('draft-store'), values, files, failures,
    photo() {
      const photo = { id: randomUUID(), uri: `file:///cache/${randomUUID()}.jpg`, width: 640, height: 480 };
      files.set(photo.uri, Buffer.from([0xff, 0xd8, 0xff, 0xd9]));
      return photo;
    },
    relocate() {
      const previous = documentUri;
      documentUri = 'file:///sandbox-b/documents';
      for (const [uri, bytes] of [...files]) {
        if (uri.startsWith(`${previous}/`)) { files.delete(uri); files.set(uri.replace(previous, documentUri), bytes); }
      }
    },
  };
}

test('native CAS serializes concurrent writes and preserves an empty revision against delayed resurrection', async () => {
  const { store } = nativeStore();
  const outcomes = await Promise.allSettled([
    store.writeDraft(user, date, null, { ...empty(), text: 'first' }),
    store.writeDraft(user, date, null, { ...empty(), text: 'second' }),
  ]);
  assert.equal(outcomes.filter(result => result.status === 'fulfilled').length, 1);
  assert.equal(outcomes.find(result => result.status === 'rejected').reason.name, 'DraftConflictError');
  const first = await store.readDraft(user, date);
  const cleared = await store.writeDraft(user, date, first.revision, empty());
  assert.notEqual(first.revision, cleared.revision);
  assert.equal((await store.listDrafts(user)).length, 0);
  await assert.rejects(store.writeDraft(user, date, first.revision, { ...empty(), text: 'late' }), { name: 'DraftConflictError' });
  await assert.rejects(store.writeDraft(user, date, null, { ...empty(), text: 'late' }), { name: 'DraftConflictError' });
  assert.equal((await store.readDraft(user, date)).text, '');
});

test('native photos survive cache release and a changed iOS document location, isolated by account and date', async () => {
  const harness = nativeStore();
  const { store, files } = harness;
  const photo = harness.photo();
  const saved = await store.writeDraft(user, date, null, { ...empty(), text: 'lunch', photos: [photo] });
  assert.notEqual(saved.photos[0].uri, photo.uri);
  store.releasePhotos([photo, ...saved.photos]);
  assert.equal(files.has(photo.uri), false);
  assert.equal(files.has(saved.photos[0].uri), true);
  assert.equal(await store.readDraft(otherUser, date), null);
  assert.equal(await store.readDraft(user, '2032-01-11'), null);
  harness.relocate();
  const restored = await store.readDraft(user, date);
  assert.match(restored.photos[0].uri, /^file:\/\/\/sandbox-b\/documents\//);
  assert.equal(files.has(restored.photos[0].uri), true);
  const raw = [...harness.values.values()][0];
  assert.equal(raw.includes('sandbox-'), false);
  assert.equal((await store.listDrafts(user))[0].date, date);
});

test('native metadata failures retain old files and also tolerate a committed write with a rejected response', async () => {
  const { store, failures, files, photo } = nativeStore();
  const firstPhoto = photo();
  const first = await store.writeDraft(user, date, null, { ...empty(), photos: [firstPhoto] });
  const secondPhoto = photo();
  failures.metadataBefore = true;
  await assert.rejects(store.writeDraft(user, date, first.revision, { ...empty(), photos: [secondPhoto] }), /metadata failed/);
  assert.equal(files.has(first.photos[0].uri), true);
  assert.equal((await store.readDraft(user, date)).revision, first.revision);
  failures.metadataBefore = false;
  failures.metadataAfter = true;
  await assert.rejects(store.writeDraft(user, date, first.revision, { ...empty(), photos: [secondPhoto] }), /after commit/);
  const committed = await store.readDraft(user, date);
  assert.notEqual(committed.revision, first.revision);
  assert.equal(files.has(committed.photos[0].uri), true);
  assert.equal(files.has(first.photos[0].uri), true);
  failures.metadataAfter = false;
  await store.writeDraft(user, date, committed.revision, empty());
  assert.equal([...files.keys()].filter(uri => uri.includes('/documents/')).length, 0);
});

test('copy failure leaves the existing native draft intact', async () => {
  const { store, failures, files, photo } = nativeStore();
  const first = await store.writeDraft(user, date, null, { ...empty(), photos: [photo()] });
  failures.copy = true;
  await assert.rejects(store.writeDraft(user, date, first.revision, { ...empty(), photos: [photo()] }), /copy failed/);
  assert.equal((await store.readDraft(user, date)).revision, first.revision);
  assert.equal(files.has(first.photos[0].uri), true);
});

test('corrupt metadata and missing photo data are visible errors without overwriting the saved draft', async () => {
  const { store, values, files, photo } = nativeStore();
  const saved = await store.writeDraft(user, date, null, { ...empty(), photos: [photo()] });
  files.delete(saved.photos[0].uri);
  await assert.rejects(store.readDraft(user, date), /fehlt/);
  await assert.rejects(store.listDrafts(user), /fehlt/);
  const original = [...values.values()][0];
  await assert.rejects(store.writeDraft(user, date, saved.revision, empty()), /fehlt/);
  assert.equal([...values.values()][0], original);
  const key = [...values.keys()][0];
  values.set(key, '{broken');
  await assert.rejects(store.readDraft(user, date), /beschädigt/);
  await assert.rejects(store.writeDraft(user, date, saved.revision, empty()), /beschädigt/);
  assert.equal(values.get(key), '{broken');
});

test('pending request identities survive restore and invalid dates, paths and bounds are rejected', async () => {
  const { store, values, photo } = nativeStore();
  const value = { ...empty(), pending: { client_id: randomUUID(), content: 'retry', attachment_ids: [randomUUID()] },
    deletion: { request_id: randomUUID(), day_id: '123', version: 2 } };
  const saved = await store.writeDraft(user, date, null, value);
  const restored = await store.readDraft(user, date);
  assert.equal(JSON.stringify(restored.pending), JSON.stringify(value.pending));
  assert.equal(JSON.stringify(restored.deletion), JSON.stringify(value.deletion));
  assert.equal((await store.listDrafts(user))[0].hasPending, true);
  await assert.rejects(store.readDraft('../outside', date), /beschädigt/);
  await assert.rejects(store.readDraft(user, '2032-02-30'), /beschädigt/);
  await assert.rejects(store.writeDraft(user, date, saved.revision, { ...empty(), text: 'a'.repeat(8001) }), /beschädigt/);
  await assert.rejects(store.writeDraft(user, date, saved.revision, { ...empty(), photos: Array.from({ length: 5 }, photo) }), /beschädigt/);
  const photoSaved = await store.writeDraft(user, date, saved.revision, { ...empty(), photos: [photo()] });
  const key = [...values.keys()][0];
  const record = JSON.parse(values.get(key));
  record.photos[0].path = '../../other-account.jpg';
  values.set(key, JSON.stringify(record));
  await assert.rejects(store.readDraft(user, date), /beschädigt/);
  await assert.rejects(store.writeDraft(user, date, photoSaved.revision, empty()), /beschädigt/);
});
