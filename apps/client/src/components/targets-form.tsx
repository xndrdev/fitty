import { useEffect, useRef, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import { randomUUID } from 'expo-crypto';
import { api, APIError, dateLabel, errorMessage, TargetSettings } from '../lib/chat-api';
import { emptyTargets, targetDraft, TargetDraft, TargetErrors, targetFields, TargetKey, validateTargets } from '../lib/targets';
import { styles as s } from '../styles/targets';
import { Button, ErrorNotice } from './ui';

type PendingTargets = { request_id: string; version: number; effective_from: string; targets: TargetSettings['targets'] };

export function TargetsForm({ userId, timeZone, disabled = false, onBusy }: {
  userId: string; timeZone: string; disabled?: boolean; onBusy: (busy: boolean) => void;
}) {
  const [settings, setSettings] = useState<TargetSettings | null>(null);
  const [draft, setDraft] = useState<TargetDraft>(() => targetDraft(emptyTargets()));
  const [errors, setErrors] = useState<TargetErrors>({});
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [latest, setLatest] = useState<TargetSettings | null>(null);
  const [focused, setFocused] = useState<TargetKey | null>(null);
  const alive = useRef(true);
  const lock = useRef(false);
  const pending = useRef<PendingTargets | null>(null);
  const current = useRef<TargetSettings | null>(null);
  const dirty = useRef(false);
  const generation = useRef(0);
  const busyCallback = useRef(onBusy);
  busyCallback.current = onBusy;
  const busy = loading || saving || !!pending.current;
  const frozen = disabled || busy || !settings;

  useEffect(() => { busyCallback.current(busy); }, [busy]);
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; generation.current++; busyCallback.current(false); };
  }, []);
  useEffect(() => {
    if (pending.current) return;
    const controller = new AbortController();
    void readSettings(false, controller.signal);
    return () => controller.abort();
  }, [userId, timeZone]);

  function adopt(value: TargetSettings, keepDraft = false) {
    current.current = value; setSettings(value); setLatest(null); setConflict(false); setErrors({}); setError('');
    if (!keepDraft) { setDraft(targetDraft(value.targets)); dirty.current = false; }
  }

  async function readSettings(review: boolean, signal?: AbortSignal) {
    if (pending.current || (lock.current && !signal)) return;
    const requestGeneration = ++generation.current;
    lock.current = true; setLoading(true); setError(''); setNotice('');
    try {
      const value = await api<TargetSettings>('/v1/targets', { userId, signal });
      if (!alive.current || signal?.aborted || requestGeneration !== generation.current) return;
      if (!current.current || (!review && !dirty.current)) { adopt(value); return; }
      if (value.version === current.current.version && value.today === current.current.today) {
        current.current = value; setSettings(value); setConflict(false); setLatest(null);
        setNotice('Der gespeicherte Stand ist unverändert. Du kannst deine Angaben jetzt speichern.');
      } else { setLatest(value); setConflict(true); }
    } catch (reason) {
      if (alive.current && !signal?.aborted && requestGeneration === generation.current) {
        setError(errorMessage(reason));
        if (current.current) setConflict(true);
      }
    } finally {
      if (alive.current && requestGeneration === generation.current) { lock.current = false; setLoading(false); }
    }
  }

  function change(key: TargetKey, text: string) {
    if (frozen) return;
    setDraft(previous => ({ ...previous, [key]: text })); dirty.current = true;
    setErrors(previous => ({ ...previous, [key]: undefined })); setNotice('');
    if (!conflict) setError('');
  }

  async function save() {
    if (disabled || lock.current || !settings || (conflict && !pending.current)) return;
    if (!pending.current) {
      const validated = validateTargets(draft);
      if (Object.keys(validated.errors).length) { setErrors(validated.errors); setError('Bitte die markierten Tagesziele prüfen.'); return; }
      pending.current = { request_id: randomUUID(), version: settings.version, effective_from: settings.today, targets: validated.targets };
    }
    lock.current = true; setSaving(true); setError(''); setNotice('');
    try {
      const value = await api<TargetSettings>('/v1/targets', { userId, method: 'PUT', body: pending.current });
      if (!alive.current) return;
      pending.current = null; adopt(value); setNotice('Tagesziele gespeichert.');
    } catch (reason) {
      if (!alive.current) return;
      if (reason instanceof APIError && [400, 401, 403, 404, 409].includes(reason.status)) {
        pending.current = null;
        if (reason.status === 409) {
          setConflict(true); setLatest(null);
          setError('Die Tagesziele oder das heutige Datum haben sich inzwischen geändert. Bitte den aktuellen Stand laden und prüfen. Deine Angaben bleiben erhalten.');
        } else setError(errorMessage(reason));
      } else setError(errorMessage(reason));
    } finally { lock.current = false; if (alive.current) setSaving(false); }
  }

  function confirmLatest(keepDraft: boolean) {
    if (!latest || frozen) return;
    adopt(latest, keepDraft);
    setNotice(keepDraft ? 'Deine Angaben bleiben erhalten. Beim Speichern ersetzen sie die aktuellen Tagesziele ab heute. Bitte vorher prüfen.' : 'Aktuelle Tagesziele übernommen. Du kannst sie jetzt bearbeiten.');
  }

  return <View style={s.section} testID="targets-form">
    <Text role="heading" aria-level={2} style={s.heading}>Deine Tagesziele</Text>
    <Text style={s.hint}>Lege nur die Ziele fest, die für dich passen. Alle Angaben sind optional. Ein leeres Feld entfernt das jeweilige Ziel.</Text>
    <Text style={s.hint}>Änderungen gelten ab heute in deiner Profilzeitzone. Frühere Tage behalten ihre damaligen Ziele.</Text>
    {settings && <Text style={s.hint}>Heute: {dateLabel(settings.today, true)} · {timeZone}</Text>}
    {loading && <Text accessibilityLiveRegion="polite" style={s.hint}>Tagesziele werden geladen …</Text>}
    {settings && <View style={s.fields}>
      {targetFields.map(field => <View style={s.field} key={field.key}>
        <Text style={s.label}>{field.label} ({field.unit})</Text>
        <TextInput accessibilityLabel={`Tagesziel ${field.label} (${field.unit})`} accessibilityHint={errors[field.key] || 'Optional. Ohne Ziel leer lassen.'}
          aria-invalid={!!errors[field.key]} value={draft[field.key]} onChangeText={text => change(field.key, text)}
          editable={!frozen} keyboardType="decimal-pad" inputMode="decimal" autoCorrect={false} maxLength={32}
          placeholder="Kein Ziel" placeholderTextColor="#687061" onFocus={() => setFocused(field.key)} onBlur={() => setFocused(null)}
          style={[s.input, !!errors[field.key] && s.invalid, focused === field.key && s.focus]} />
        {!!errors[field.key] && <Text accessibilityRole="alert" style={s.error}>{errors[field.key]}</Text>}
      </View>)}
    </View>}
    {latest && <View style={s.notice}>
      <Text style={s.label}>Aktuell gespeicherte Tagesziele</Text>
      <Text style={s.hint}>Heute: {dateLabel(latest.today, true)}</Text>
      {targetFields.map(field => <Text key={field.key} style={s.hint}>{field.label}: {latest.targets[field.key] === null ? 'Kein Ziel' : `${latest.targets[field.key]?.toLocaleString('de-DE', { maximumFractionDigits: 2 })} ${field.unit}`}</Text>)}
      <Text style={s.hint}>„Aktuelle Ziele übernehmen“ ersetzt deine Eingaben. Mit „Meine Angaben beibehalten“ kannst du deine Eingaben prüfen und anschließend für heute speichern.</Text>
      <View style={s.actions}>
        <Button secondary label="Aktuelle Ziele übernehmen" disabled={frozen} onPress={() => confirmLatest(false)} />
        <Button secondary label="Meine Angaben beibehalten" disabled={frozen} onPress={() => confirmLatest(true)} />
      </View>
    </View>}
    <ErrorNotice text={error} />
    {!!notice && <Text accessibilityLiveRegion="polite" style={s.success}>{notice}</Text>}
    {!!pending.current && !saving && <Text accessibilityLiveRegion="polite" style={s.hint}>Das Ergebnis ist noch offen. Bitte denselben Speichervorgang erneut versuchen. Deine Angaben bleiben bis zur Bestätigung gesperrt.</Text>}
    {(!settings || conflict) && !pending.current && <Button secondary label={loading ? 'Stand wird geladen …' : 'Aktuelle Tagesziele laden'} disabled={disabled || loading || saving} onPress={() => void readSettings(true)} />}
    {settings && <Button label={saving ? 'Tagesziele werden gespeichert …' : pending.current ? 'Tagesziele erneut speichern' : 'Tagesziele speichern'}
      disabled={disabled || loading || saving || (conflict && !pending.current)} onPress={() => void save()} />}
  </View>;
}
