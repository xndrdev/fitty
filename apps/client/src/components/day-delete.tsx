import { useEffect, useRef, useState } from 'react';
import { Modal, Pressable, ScrollView, Text, View } from 'react-native';
import { randomUUID } from 'expo-crypto';
import { api, APIError, dateLabel, DayDeletionPreview, DayDeletionResult, errorMessage } from '../lib/chat-api';
import { styles as s } from '../styles/day-delete';
import { Button, ErrorNotice } from './ui';

import type { PendingDayDeletion } from '../lib/draft-store.types';

export function DayDelete({ userId, date, hasDraft, hasPhotos, initialRequest, prepareRequest, releaseRequest, onClose, onDeleted, onRefresh }: {
  userId: string; initialRequest: PendingDayDeletion | null; prepareRequest: (request: PendingDayDeletion) => Promise<unknown>; releaseRequest: () => Promise<unknown>;
  date: string; hasDraft: boolean; hasPhotos: boolean; onClose: () => void;
  onDeleted: (result: DayDeletionResult) => Promise<void>; onRefresh: () => void;
}) {
  const [preview, setPreview] = useState<DayDeletionPreview | null>(null);
  const [loading, setLoading] = useState(!initialRequest);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [conflict, setConflict] = useState(false);
  const [gone, setGone] = useState(false);
  const [deleteFocused, setDeleteFocused] = useState(false);
  const alive = useRef(true);
  const lock = useRef(false);
  const loadVersion = useRef(0);
  const pending = useRef<PendingDayDeletion | null>(initialRequest);
  const day = preview?.day;
  const frozen = loading || saving || !!pending.current;
  const actionDisabled = loading || saving || (!day && !pending.current) || conflict || gone;

  useEffect(() => {
    alive.current = true;
    const controller = new AbortController();
    if (!pending.current) void load(controller.signal);
    return () => { alive.current = false; loadVersion.current++; lock.current = false; controller.abort(); };
  }, [date]);

  async function load(signal?: AbortSignal) {
    if (lock.current || pending.current) return;
    const version = ++loadVersion.current;
    lock.current = true; setLoading(true); setError(''); setNotice('');
    try {
      const result = await api<DayDeletionPreview>(`/v1/days/${date}/deletion`, { signal, userId });
      if (!alive.current || signal?.aborted || version !== loadVersion.current) return;
      setPreview(result); setConflict(false); setGone(false);
      if (preview && result.day) setNotice('Aktueller Stand geladen. Bitte die Angaben prüfen und das Löschen erneut bestätigen.');
    } catch (reason) { if (alive.current && !signal?.aborted && version === loadVersion.current) setError(errorMessage(reason)); }
    finally { if (version === loadVersion.current) { lock.current = false; if (alive.current && !signal?.aborted) setLoading(false); } }
  }

  function close() { if (!lock.current && !pending.current) onClose(); }

  async function remove() {
    if (lock.current || actionDisabled || (!day && !pending.current)) return;
    const request = pending.current ?? { request_id: randomUUID(), day_id: day!.id, version: day!.version };
    lock.current = true; setSaving(true); setError(''); setNotice('');
    try {
      await prepareRequest(request);
      pending.current = request;
      if (!alive.current) return;
      const result = await api<DayDeletionResult>(`/v1/days/${date}`, { method: 'DELETE', body: request, userId });
      if (alive.current) { await onDeleted(result); pending.current = null; }
    } catch (reason) {
      if (!alive.current) return;
      if (reason instanceof APIError && [400, 404, 409].includes(reason.status)) {
        try { await releaseRequest(); } catch { setError('Der offene Löschvorgang konnte auf diesem Gerät nicht aktualisiert werden. Bitte erneut versuchen.'); return; }
        pending.current = null;
        if (reason.status === 409) {
          setConflict(true);
          setError('Der Tageschat wurde inzwischen geändert. Bitte den aktuellen Stand laden und das Löschen erneut bestätigen.');
        } else if (reason.status === 404) {
          setGone(true);
          setError('Dieser Tageschat wurde bereits gelöscht. Dein ungesendeter Entwurf auf diesem Gerät bleibt erhalten.');
          onRefresh();
        } else setError(errorMessage(reason));
      } else setError(errorMessage(reason));
    } finally { lock.current = false; if (alive.current) setSaving(false); }
  }

  return <Modal visible transparent animationType="none" onRequestClose={close} accessibilityLabel="Tageschat löschen">
    <View style={s.overlay}>
      <View style={s.sheet} accessibilityViewIsModal>
        <View style={s.header}>
          <View style={s.headingGroup}><Text style={s.date}>{dateLabel(date, true)}</Text>
            <Text role="heading" aria-level={2} style={s.title}>Tageschat löschen?</Text></View>
          <Button variant="ghost" icon="close" iconOnly label="Schließen" accessibilityLabel="Löschdialog schließen" disabled={frozen} onPress={close} />
        </View>
        <ScrollView style={s.scroll} contentContainerStyle={s.content}>
          {loading ? <Text accessibilityLiveRegion="polite" style={s.text}>Der aktuelle Tageschat wird geladen …</Text>
            : day && !gone ? <>
              <Text style={s.text}>Für diesen Tag werden dauerhaft entfernt:</Text>
              <View style={s.counts}>
                {[
                  ['Nachrichten', day.message_count], ['Einträge', day.entry_count], ['Fotos', day.photo_count],
                ].map(([label, count]) => <View key={label} style={s.count}><Text style={s.text}>{label}</Text><Text style={s.number}>{typeof count === 'number' ? count.toLocaleString('de-DE') : count}</Text></View>)}
              </View>
              <Text style={s.text}>Auch der Änderungsverlauf der Einträge wird gelöscht. Offene Auswertungen werden verworfen. Neue Antworten werden für diesen Tageschat nicht mehr gespeichert.</Text>
              {(hasDraft || hasPhotos) && <Text style={s.text}>Dein ungesendeter Entwurf{hasPhotos ? ' mit den ausgewählten Fotos' : ''} auf diesem Gerät wird ebenfalls verworfen.</Text>}
              <Text style={s.warning}>Das Löschen kann nicht rückgängig gemacht werden.</Text>
            </> : pending.current ? <Text style={s.text}>Eine bereits bestätigte Löschung ist noch offen. Bitte denselben Vorgang erneut versuchen, um das Ergebnis zu bestätigen.</Text> : preview && !gone && <>
              <Text style={s.text}>Für diesen Tag ist noch kein Tageschat gespeichert.</Text>
              {(hasDraft || hasPhotos) && <Text style={s.hint}>Dein ungesendeter Entwurf bleibt erhalten.</Text>}
              {!!preview.pending_photo_deletions && <Text style={s.hint}>Fotos eines bereits gelöschten Tageschats werden im Hintergrund entfernt.</Text>}
            </>}
        </ScrollView>
        <View style={s.footer}>
          <ErrorNotice text={error} />
          {!!notice && <Text accessibilityLiveRegion="polite" style={s.hint}>{notice}</Text>}
          {!!pending.current && !saving && <Text style={s.hint}>Das Ergebnis ist noch offen. Bitte denselben Vorgang erneut versuchen. Bis zur Bestätigung bleibt der Tageschat gesperrt.</Text>}
          {!pending.current && !gone && (conflict || (!preview && !!error)) && <Button secondary label={loading ? 'Stand wird geladen …' : 'Aktuellen Stand laden'} disabled={loading || saving} onPress={() => void load()} />}
          <View style={s.actions}>
            <Button secondary label={gone || (!loading && preview && !day) ? 'Schließen' : 'Abbrechen'} disabled={frozen} onPress={close} />
            {(day || pending.current) && !gone && <Pressable accessibilityRole="button" accessibilityLabel={pending.current && !saving ? 'Löschen erneut versuchen' : 'Tageschat löschen'}
              accessibilityState={{ disabled: actionDisabled }} disabled={actionDisabled} onPress={() => void remove()}
              onFocus={() => setDeleteFocused(true)} onBlur={() => setDeleteFocused(false)}
              style={({ pressed }) => [s.deleteButton, actionDisabled && s.disabled, pressed && s.pressed, deleteFocused && s.focus]}>
              <Text style={s.deleteText}>{saving ? 'Löschen …' : pending.current ? 'Erneut versuchen' : 'Tageschat löschen'}</Text>
            </Pressable>}
          </View>
        </View>
      </View>
    </View>
  </Modal>;
}
