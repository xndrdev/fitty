const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

function harness(platform) {
  const listeners = new Map(), hooks = [], cleanups = [], frames = new Map();
  let cursor = 0, nextFrame = 0, visible = false, dismissed = false;
  const focused = { top: 600, height: 48, measureInWindow(callback) { callback(0, this.top, 300, this.height); } };
  const react = {
    createContext: value => ({ Provider: 'Provider', value }), useContext: context => context.value,
    useState(initial) { const i = cursor++; if (!(i in hooks)) hooks[i] = typeof initial === 'function' ? initial() : initial; return [hooks[i], value => { hooks[i] = value; }]; },
    useRef(current) { const i = cursor++; return hooks[i] ??= { current }; },
    useEffect(effect) { const i = cursor++; if (!(i in hooks)) { hooks[i] = true; cleanups.push(effect()); } },
  };
  const Keyboard = {
    isVisible: () => visible, dismiss: () => { dismissed = true; },
    addListener(name, callback) { listeners.set(name, callback); return { remove: () => listeners.delete(name) }; },
  };
  const filename = path.join(__dirname, 'keyboard-layout.tsx');
  const source = ts.transpileModule(readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports = {};
  vm.runInNewContext(source, {
    exports,
    requestAnimationFrame(callback) { frames.set(++nextFrame, callback); return nextFrame; },
    cancelAnimationFrame(id) { frames.delete(id); },
    require(id) {
      if (id === 'react') return react;
      if (id === 'react/jsx-runtime') return { jsx: (type, props) => ({ type, props }), jsxs: (type, props) => ({ type, props }) };
      if (id === 'react-native') return { Keyboard, Platform: { OS: platform }, ScrollView: 'ScrollView', View: 'View', KeyboardAvoidingView: 'KeyboardAvoidingView', TextInput: { State: { currentlyFocusedInput: () => focused } } };
      if (id === 'react-native-safe-area-context') return { SafeAreaView: 'SafeAreaView' };
      if (id === '../styles/app') return { styles: {} };
      if (id === './ui') return { Button: 'Button' };
      throw new Error(`Unexpected dependency: ${id}`);
    },
  }, { filename });
  return {
    render(name, props) { cursor = 0; return exports[name](props); }, focused, listeners, frames,
    emit(name) { visible = name.endsWith('Show'); listeners.get(name)?.(); },
    flush() { for (const [id, callback] of frames) { frames.delete(id); callback(); } },
    unmount() { cleanups.forEach(cleanup => cleanup?.()); }, isDismissed: () => dismissed,
  };
}

for (const platform of ['ios', 'android']) test(`${platform}: numeric keyboards can be dismissed and subscriptions are removed`, () => {
  const app = harness(platform), props = { children: 'form' };
  app.render('KeyboardScreen', props);
  app.emit(platform === 'ios' ? 'keyboardWillShow' : 'keyboardDidShow');
  const open = app.render('KeyboardScreen', props);
  const safeArea = open.props.children.props.children;
  assert.equal(safeArea.props.edges.includes('bottom'), false);
  const toolbar = safeArea.props.children[1];
  toolbar.props.children.props.onPress();
  assert.equal(app.isDismissed(), true);
  app.emit(platform === 'ios' ? 'keyboardWillHide' : 'keyboardDidHide');
  const closed = app.render('KeyboardScreen', props).props.children.props.children;
  assert.equal(closed.props.edges.includes('bottom'), true);
  assert.equal(closed.props.children[1], false);
  app.unmount(); assert.equal(app.listeners.size, 0);
});

test('chat can hide the toolbar while forms retain numeric keyboard dismissal', () => {
  const app = harness('ios'), props = { children: 'chat' };
  const screen = app.render('KeyboardScreen', props);
  const setToolbar = screen.props.children.props.children.props.children[0].props.children.props.value;
  setToolbar(false);
  app.emit('keyboardWillShow');
  assert.equal(app.render('KeyboardScreen', props).props.children.props.children.props.children[1], false);
  setToolbar(true);
  assert.equal(app.render('KeyboardScreen', props).props.children.props.children.props.children[1].type, 'View');
  app.unmount();
});

test('native forms reveal the focused field above fixed actions without scrolling visible fields', () => {
  const app = harness('ios'), positions = [];
  const scrollRef = { current: { getNativeScrollRef: () => ({ measureInWindow: callback => callback(0, 100, 390, 300) }), scrollTo: ({ y }) => positions.push(y) } };
  const form = app.render('FormScrollView', { scrollRef });
  form.props.onScroll({ nativeEvent: { contentOffset: { y: 200 } } });
  app.emit('keyboardDidShow'); app.flush();
  assert.deepEqual(positions, []); // A keyboard opened in another form/modal.
  form.props.onFocus({});
  app.emit('keyboardDidShow'); app.flush();
  assert.deepEqual(positions, [460]); // Input bottom 648; available bottom 388.
  app.focused.top = 200;
  form.props.onFocus({}); app.flush();
  assert.deepEqual(positions, [460]);
  app.focused.top = 80;
  form.props.onFocus({}); app.flush();
  assert.deepEqual(positions, [460, 168]);
  form.props.onBlur({}); app.emit('keyboardDidShow'); app.flush();
  assert.deepEqual(positions, [460, 168]);
  form.props.onFocus({}); app.unmount();
  assert.equal(app.frames.size, 0); assert.equal(app.listeners.size, 0);
});

test('web leaves keyboard handling to the browser and never subscribes to native events', () => {
  const screen = harness('web');
  const root = screen.render('KeyboardScreen', { children: 'form' });
  assert.equal(root.props.children.props.enabled, false); assert.equal(screen.listeners.size, 0);
  const form = harness('web');
  let revealed = false;
  form.render('FormScrollView', {}).props.onFocus({ target: { scrollIntoView: () => { revealed = true; } } });
  assert.equal(revealed, true);
  assert.equal(form.frames.size, 0); assert.equal(form.listeners.size, 0);
});
