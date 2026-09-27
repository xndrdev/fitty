const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

function menu(platform, props = {}) {
  const hooks = [], effects = [], actions = [];
  let cursor = 0, closed = false;
  const filename = path.join(__dirname, 'workspace-menu.tsx');
  const source = ts.transpileModule(readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports = {};
  vm.runInNewContext(source, {
    exports,
    require(id) {
      if (id === 'react') return {
        useRef(value) { return hooks[cursor++] ??= { current: value }; },
        useEffect(effect) { effects.push(effect); },
      };
      if (id === 'react/jsx-runtime') return { jsx: (type, props) => ({ type, props }), jsxs: (type, props) => ({ type, props }) };
      if (id === 'react-native') return { Platform: { OS: platform }, Modal: 'Modal', View: 'View', ScrollView: 'ScrollView', Pressable: 'Pressable', Text: 'Text' };
      if (id === 'react-native-safe-area-context') return { SafeAreaView: 'SafeAreaView' };
      if (id === '../lib/chat-api') return { dateLabel: date => date };
      if (id === './icon') return { Icon: 'Icon' };
      if (id === './ui') return { Brand: 'Brand', Button: 'Button' };
      if (id === '../styles/workspace-menu') return { styles: {} };
      throw new Error(`Unexpected dependency: ${id}`);
    },
  }, { filename });
  function render(visible) {
    cursor = 0;
    const tree = exports.WorkspaceMenu({ visible, pane: 'chat', date: '2026-09-27', today: '2026-09-27', disabled: false,
      onClose: () => { closed = true; }, onAction: action => actions.push(action), ...props });
    effects.splice(0).forEach(effect => effect());
    return tree;
  }
  function find(node, label) {
    if (node?.props?.label === label) return node;
    return [node?.props?.children].flat().filter(Boolean).map(child => find(child, label)).find(Boolean);
  }
  return { render, find, actions, closed: () => closed };
}

for (const [label, action] of [['Datum wählen', 'date'], ['Tageschat löschen', 'delete'], ['Ziele & Vorlieben', 'profile']]) {
  test(`iOS completes dismissal before ${action} and dispatches once`, () => {
    const app = menu('ios'), open = app.render(true);
    app.find(open, label).props.onPress();
    assert.equal(app.closed(), true); assert.deepEqual(app.actions, []);
    const hidden = app.render(false);
    assert.deepEqual(app.actions, []);
    hidden.props.onDismiss(); hidden.props.onDismiss();
    assert.deepEqual(app.actions, [action]);
  });
}

for (const [label, action] of [['Datum wählen', 'date'], ['Tageschat löschen', 'delete'], ['Ziele & Vorlieben', 'profile']]) {
  test(`Web completes dismissal before ${action} and dispatches once`, () => {
    const app = menu('web'), open = app.render(true);
    app.find(open, label).props.onPress();
    assert.equal(app.closed(), true); assert.deepEqual(app.actions, []);
    const hidden = app.render(false);
    assert.deepEqual(app.actions, []);
    hidden.props.onDismiss(); hidden.props.onDismiss();
    assert.deepEqual(app.actions, [action]);
  });
}

for (const platform of ['android']) test(`${platform} navigates after closing without an onDismiss event`, () => {
  const app = menu(platform), open = app.render(true);
  app.find(open, 'Verlauf').props.onPress();
  assert.deepEqual(app.actions, []);
  app.render(false).props.onDismiss();
  assert.deepEqual(app.actions, ['history']);
});

test('dismissing the menu does not navigate or log out', () => {
  const app = menu('ios'), open = app.render(true);
  open.props.onRequestClose();
  app.render(false).props.onDismiss();
  assert.equal(app.closed(), true); assert.deepEqual(app.actions, []);
});

test('a second tap during dismissal cannot replace the chosen action', () => {
  const app = menu('ios'), open = app.render(true);
  app.find(open, 'Tageschat').props.onPress();
  app.find(open, 'Tageschat löschen').props.onPress();
  app.render(false).props.onDismiss();
  assert.deepEqual(app.actions, ['chat']);
});

test('busy work prevents stale action callbacks from navigating or logging out', () => {
  const app = menu('web', { disabled: true }), open = app.render(true);
  app.find(open, 'Abmelden').props.onPress();
  app.find(open, 'Vorheriger Tag').props.onPress();
  app.render(false);
  assert.equal(app.closed(), false); assert.deepEqual(app.actions, []);
});
