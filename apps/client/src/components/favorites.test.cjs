const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

function menu(platform) {
  const hooks = [], effects = [], actions = [];
  const favorite = { id: '1', entry: { label: 'Haferflocken' } };
  let cursor = 0, closed = false;
  const filename = path.join(__dirname, 'favorites.tsx');
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
      if (id === './icon') return { Icon: 'Icon' };
      if (id === './ui') return { Button: 'Button' };
      if (id === '../styles/composer-actions' || id === '../styles/favorites') return { styles: {} };
      throw new Error(`Unexpected dependency: ${id}`);
    },
  }, { filename });
  function render(visible) {
    cursor = 0;
    const tree = exports.FavoritesSheet({ visible, model: {favorites: [favorite], ready: true, busy: '', error: ''}, onClose: () => { closed = true; }, onChoose: action => actions.push(action.id) });
    effects.splice(0).forEach(effect => effect());
    return tree;
  }
  function find(node, label) {
    if (node?.props?.favorite?.id === label) return node;
    return [node?.props?.children].flat(Infinity).filter(Boolean).map(child => find(child, label)).find(Boolean);
  }
  return { render, find, actions, closed: () => closed };
}

for (const platform of ['ios', 'android', 'web']) test(`${platform} inserts a favorite only after dismissal, exactly once`, () => {
  const app = menu(platform), open = app.render(true);
  app.find(open, '1').props.onPress();
  assert.deepEqual(app.actions, []);
  const hidden = app.render(false);
  if (platform !== 'android') assert.deepEqual(app.actions, []);
  hidden.props.onDismiss(); hidden.props.onDismiss();
  assert.deepEqual(app.actions, ['1']);
});

test('closing the favorites sheet without selection does not change the draft', () => {
  const app = menu('ios'), open = app.render(true);
  open.props.onRequestClose();
  app.render(false).props.onDismiss();
  assert.equal(app.closed(), true); assert.deepEqual(app.actions, []);
});
