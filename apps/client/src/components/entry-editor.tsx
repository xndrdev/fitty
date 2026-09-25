import { useEffect, useRef, useState } from 'react';
import { KeyboardAvoidingView, Modal, Platform, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { randomUUID } from 'expo-crypto';
import { api, APIError, dateLabel, Entry, EntryValues, errorMessage, Summary } from '../lib/chat-api';
import { styles as s } from '../styles/entry-editor';
import { Button, ErrorNotice } from './ui';

export type EntryEditorMode = 'edit' | 'delete';
type NumericKey = 'calories' | 'protein_g' | 'carbs_g' | 'fat_g' | 'duration_minutes' | 'distance_km';
type Draft = Record<NumericKey | 'label' | 'amount' | 'notes', string> & { source: Entry['source'] };
type PendingChange = { method: 'PUT' | 'DELETE'; body: { request_id: string; version: number; entry?: EntryValues } };
const sources = { estimate: 'Schätzung', user: 'Deine Angaben', device: 'Geräteangabe' };
const numericFields: { key: NumericKey; label: string; max: number }[] = [
  { key: 'calories', label: 'Kalorien (kcal)', max: 20000 },
  { key: 'protein_g', label: 'Protein (g)', max: 2000 },
  { key: 'carbs_g', label: 'Kohlenhydrate (g)', max: 2000 },
  { key: 'fat_g', label: 'Fett (g)', max: 2000 },
  { key: 'duration_minutes', label: 'Dauer (Minuten)', max: 1440 },
  { key: 'distance_km', label: 'Strecke (km)', max: 500 },
];

function draftFor(entry: Entry): Draft {
  const text = (value: number | null) => value === null ? '' : String(value).replace('.', ',');
  return {
    label: entry.label, amount: entry.amount, notes: entry.notes, source: entry.source,
    calories: text(entry.calories), protein_g: text(entry.protein_g), carbs_g: text(entry.carbs_g),
    fat_g: text(entry.fat_g), duration_minutes: text(entry.duration_minutes), distance_km: text(entry.distance_km),
  };
}

function validate(draft: Draft, kind: Entry['kind']) {
  const errors: Partial<Record<keyof Draft, string>> = {};
  const entry: EntryValues = { kind, label: draft.label.trim(), amount: draft.amount.trim(), notes: draft.notes.trim(), source: draft.source,
    calories: null, protein_g: null, carbs_g: null, fat_g: null, duration_minutes: null, distance_km: null };
  for (const [key, max] of [['label', 160], ['amount', 160], ['notes', 1000]] as const) {
    if (Array.from(entry[key]).length > max) errors[key] = `Bitte höchstens ${max} Zeichen eingeben.`;
    if (entry[key].includes('\0')) errors[key] = 'Die Angabe enthält ein ungültiges Zeichen. Bitte neu eingeben.';
  }
  if (!entry.label) errors.label = 'Bitte eine Bezeichnung eingeben.';
  for (const { key, max } of numericFields) {
    if (kind === 'food' ? ['duration_minutes', 'distance_km'].includes(key) : ['protein_g', 'carbs_g', 'fat_g'].includes(key)) continue;
    const value = draft[key].trim();
    if (!value) { if (kind === 'food') errors[key] = 'Bitte einen Wert eingeben; 0 ist möglich.'; continue; }
    if (!/^\d+(?:[.,]\d{1,2})?$/.test(value)) { errors[key] = 'Bitte eine positive Zahl oder 0 mit höchstens zwei Nachkommastellen eingeben, ohne Tausendertrennzeichen.'; continue; }
    const parsed = Number(value.replace(',', '.'));
    if (!Number.isFinite(parsed) || parsed > max) { errors[key] = `Bitte höchstens ${max.toLocaleString('de-DE')} eingeben.`; continue; }
    entry[key] = parsed;
  }
  if (kind === 'activity' && !draft.duration_minutes.trim() && !draft.distance_km.trim()) errors.duration_minutes = 'Bitte mindestens Dauer oder Strecke eingeben.';
  return { entry, errors };
}

function EditorField({ label, value, onChange, disabled, numeric, multiline, error }: {
  label: string; value: string; onChange: (value: string) => void; disabled: boolean; numeric?: boolean; multiline?: boolean; error?: string;
}) {
  const [focused, setFocused] = useState(false);
  return <View style={s.field}>
    <Text style={s.label}>{label}</Text>
    <TextInput accessibilityLabel={label} accessibilityHint={error} aria-invalid={!!error} value={value} onChangeText={onChange}
      editable={!disabled} multiline={multiline} keyboardType={numeric ? 'decimal-pad' : 'default'} autoCorrect={!numeric}
      onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
      style={[s.input, multiline && s.notes, !!error && s.inputInvalid, focused && s.focus]} />
    {!!error && <Text accessibilityRole="alert" style={s.error}>{error}</Text>}
  </View>;
}

export function EntryEditor({ date, entry: initialEntry, mode, analysisBusy, onClose, onSaved, onRefresh }: {
  date: string; entry: Entry; mode: EntryEditorMode; analysisBusy: boolean;
  onClose: () => void; onSaved: () => void; onRefresh: () => void;
}) {
  const [entry, setEntry] = useState(initialEntry);
  const [draft, setDraft] = useState(() => draftFor(initialEntry));
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof Draft, string>>>({});
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [gone, setGone] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [latest, setLatest] = useState<Entry | null>(null);
  const [deleteFocused, setDeleteFocused] = useState(false);
  const alive = useRef(true);
  const lock = useRef(false);
  const pending = useRef<PendingChange | null>(null);
  const content = useRef<ScrollView>(null);
  const deleting = mode === 'delete';
  const frozen = saving || !!pending.current || reloading || gone;
  const actionDisabled = saving || reloading || gone || (!pending.current && (analysisBusy || conflict));

  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => { if (latest) content.current?.scrollToEnd({ animated: false }); }, [latest]);

  function change(key: keyof Draft, value: string) {
    if (frozen) return;
    setDraft(previous => ({ ...previous, [key]: value }));
    setFieldErrors(previous => ({ ...previous, [key]: undefined }));
    if (!conflict) setError('');
    setNotice('');
  }
  function close() { if (!lock.current && !pending.current) onClose(); }

  async function submit() {
    if (lock.current || actionDisabled) return;
    if (!pending.current) {
      const { entry: values, errors } = validate(draft, entry.kind);
      if (!deleting && Object.keys(errors).length) {
        setFieldErrors(errors); setError('Bitte die markierten Angaben prüfen.'); content.current?.scrollTo({ y: 0, animated: false }); return;
      }
      pending.current = { method: deleting ? 'DELETE' : 'PUT', body: {
        request_id: randomUUID(), version: entry.version, ...(!deleting ? { entry: values } : {}),
      } };
    }
    lock.current = true; setSaving(true); setError(''); setNotice('');
    try {
      await api(`/v1/days/${date}/entries/${entry.id}`, { method: pending.current.method, body: pending.current.body });
      if (alive.current) { pending.current = null; onSaved(); }
    } catch (reason) {
      if (!alive.current) return;
      if (reason instanceof APIError && [400, 404, 409].includes(reason.status)) {
        pending.current = null;
        if (reason.status === 409) {
          setConflict(true); setLatest(null);
          setError('Der Eintrag wurde inzwischen geändert oder Fitty wertet den Tag gerade aus. Bitte den aktuellen Stand laden und prüfen. Dein Entwurf bleibt erhalten.');
        } else if (reason.status === 404) {
          setGone(true); setError('Dieser Eintrag ist nicht mehr vorhanden. Deine bisherigen Chatnachrichten bleiben erhalten.'); onRefresh();
        } else setError(errorMessage(reason));
      } else setError(errorMessage(reason));
    } finally { lock.current = false; if (alive.current) setSaving(false); }
  }

  async function reload() {
    if (lock.current || pending.current) return;
    lock.current = true; setReloading(true); setNotice('');
    try {
      const summary = await api<Summary>(`/v1/days/${date}/summary`);
      if (!alive.current) return;
      onRefresh();
      const current = summary.entries.find(item => item.id === entry.id);
      if (!current) { setGone(true); setLatest(null); setError('Dieser Eintrag ist nicht mehr vorhanden.'); return; }
      if (summary.pending_count > 0) { setLatest(null); setError('Fitty wertet den Tag noch aus. Bitte anschließend den aktuellen Stand erneut laden. Dein Entwurf bleibt erhalten.'); return; }
      if (current.version === entry.version) {
        setConflict(false); setLatest(null); setError('');
        setNotice(deleting ? 'Der Eintrag ist unverändert. Du kannst das Löschen jetzt bestätigen.' : 'Der Eintrag ist unverändert. Du kannst deinen Entwurf jetzt speichern.'); return;
      }
      setLatest(current); setError('');
    } catch (reason) { if (alive.current) setError(errorMessage(reason)); }
    finally { lock.current = false; if (alive.current) setReloading(false); }
  }

  function adoptLatest() {
    if (!latest || frozen || analysisBusy) return;
    setEntry(latest); setDraft(draftFor(latest)); setLatest(null); setConflict(false); setFieldErrors({}); setError('');
    setNotice(deleting ? 'Aktuellen Stand übernommen. Bitte das Löschen erneut bestätigen.' : 'Aktuelle Werte übernommen. Bitte vor dem Speichern prüfen.');
    content.current?.scrollTo({ y: 0, animated: false });
  }

  return <Modal visible transparent animationType="none" onRequestClose={close} accessibilityLabel={deleting ? 'Eintrag löschen' : 'Eintrag bearbeiten'}>
    <KeyboardAvoidingView style={s.overlay} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <View style={s.sheet} accessibilityViewIsModal>
        <View style={s.header}>
          <View style={s.headingGroup}><Text style={s.eyebrow}>{entry.kind === 'food' ? 'Essen' : 'Bewegung'} · {dateLabel(date)}</Text>
            <Text role="heading" aria-level={2} style={s.title}>{deleting ? 'Eintrag löschen?' : 'Eintrag bearbeiten'}</Text></View>
          <Button variant="ghost" icon="close" iconOnly label="Schließen" accessibilityLabel="Eintrag schließen" disabled={saving || reloading || !!pending.current} onPress={close} />
        </View>
        <ScrollView ref={content} style={s.scroll} contentContainerStyle={s.content} keyboardShouldPersistTaps="handled">
          {deleting ? <>
            <Text style={s.entryName}>{entry.label}</Text>
            {!!entry.amount && <Text style={s.text}>{entry.amount}</Text>}
            <Text style={s.text}>Dieser Eintrag wird aus deinen Tageswerten entfernt. Frühere Chatnachrichten und Fotos bleiben erhalten.</Text>
          </> : <>
            <EditorField label="Bezeichnung" value={draft.label} onChange={value => change('label', value)} disabled={frozen} error={fieldErrors.label} />
            <EditorField label="Menge / Beschreibung (optional)" value={draft.amount} onChange={value => change('amount', value)} disabled={frozen} error={fieldErrors.amount} />
            <View style={s.notice}><Text style={s.hint}>Alle Werte gelten für die gesamte angegebene Menge. Eine Änderung der Menge rechnet die Werte nicht automatisch um.</Text></View>
            <View style={s.numericFields}>
              {numericFields.filter(field => entry.kind === 'food' ? !['duration_minutes', 'distance_km'].includes(field.key) : !['protein_g', 'carbs_g', 'fat_g'].includes(field.key))
                .map(field => <View key={field.key} style={s.numericField}><EditorField label={field.label} value={draft[field.key]} numeric
                  onChange={value => change(field.key, value)} disabled={frozen} error={fieldErrors[field.key]} /></View>)}
            </View>
            {entry.kind === 'activity' && <Text style={s.hint}>Mindestens Dauer oder Strecke angeben. Leeres Feld bedeutet unbekannt; 0 ist eine konkrete Angabe.</Text>}
            <View style={s.field}><Text style={s.label}>Woher stammen die Werte?</Text><View style={s.actions}>
              {(Object.keys(sources) as Entry['source'][]).map(source => <Button key={source} secondary selected={draft.source === source}
                label={sources[source]} disabled={frozen} onPress={() => change('source', source)} />)}
            </View></View>
            <EditorField label="Notizen (optional)" value={draft.notes} onChange={value => change('notes', value)} disabled={frozen} multiline error={fieldErrors.notes} />
          </>}
          {latest && <View style={[s.notice, s.conflict]}>
            <Text style={s.label}>Inzwischen gespeichert</Text>
            <Text style={s.text}>{latest.label}{latest.amount ? ` · ${latest.amount}` : ''}</Text>
            <Text style={s.hint}>{numericFields.filter(field => latest[field.key] !== null).map(field => `${field.label}: ${latest[field.key]?.toLocaleString('de-DE')}`).join(' · ')}</Text>
            <Text style={s.hint}>{sources[latest.source]}{latest.notes ? ` · ${latest.notes}` : ''}</Text>
            <Text style={s.hint}>{deleting ? 'Übernimm den aktuellen Stand, bevor du das Löschen erneut bestätigst.' : '„Aktuelle Werte übernehmen“ ersetzt deinen Entwurf. Danach kannst du die neuen Werte bearbeiten.'}</Text>
            <Button secondary label="Aktuelle Werte übernehmen" disabled={frozen || analysisBusy} onPress={adoptLatest} />
          </View>}
        </ScrollView>
        <View style={s.footer}>
          <ErrorNotice text={error} />
          {!!notice && <Text accessibilityLiveRegion="polite" style={s.hint}>{notice}</Text>}
          {analysisBusy && !pending.current && <Text accessibilityLiveRegion="polite" style={s.hint}>Fitty wertet deinen Tag aus. Änderungen sind danach wieder möglich.</Text>}
          {!!pending.current && !saving && <Text style={s.hint}>Das Ergebnis ist noch offen. Bitte denselben Vorgang erneut versuchen. Deine Angaben bleiben bis zur Bestätigung gesperrt.</Text>}
          {conflict && !gone && <Button secondary label={reloading ? 'Stand wird geladen …' : 'Aktuellen Stand laden'} disabled={saving || reloading} onPress={() => void reload()} />}
          <View style={s.footerActions}>
            <Button secondary label={gone ? 'Schließen' : 'Abbrechen'} disabled={saving || reloading || !!pending.current} onPress={close} />
            {!gone && (deleting ? <Pressable accessibilityRole="button" accessibilityLabel={pending.current && !saving ? 'Löschen erneut versuchen' : 'Eintrag löschen'}
              accessibilityState={{ disabled: actionDisabled }} disabled={actionDisabled} onPress={() => void submit()}
              onFocus={() => setDeleteFocused(true)} onBlur={() => setDeleteFocused(false)}
              style={({ pressed }) => [s.deleteButton, actionDisabled && s.disabled, pressed && s.pressed, deleteFocused && s.focus]}>
              <Text style={s.deleteText}>{saving ? 'Löschen …' : pending.current ? 'Erneut versuchen' : 'Löschen'}</Text>
            </Pressable> : <Button label={saving ? 'Speichern …' : pending.current ? 'Erneut versuchen' : 'Änderungen speichern'} disabled={actionDisabled} onPress={() => void submit()} />)}
          </View>
        </View>
      </View>
    </KeyboardAvoidingView>
  </Modal>;
}
