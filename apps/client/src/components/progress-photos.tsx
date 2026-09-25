import { useEffect, useRef, useState } from 'react';
import { Image, Modal, Platform, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { api, APIError, dateLabel, errorMessage, todayIn } from '../lib/chat-api';
import { discardPhoto, DraftPhoto, pickPhotos, readPhotoBytes } from '../lib/photos';
import { mergeProgressPhotos, progressDateError, progressDateLabel, ProgressPhoto, ProgressPhotoPage, ProgressView, progressViewLabel, progressViews } from '../lib/progress-photos';
import { styles as s } from '../styles/progress-photos';
import { Button, ErrorNotice } from './ui';

type PendingUpload = { photo: DraftPhoto; date: string; view: ProgressView; bytes: ArrayBuffer };
type Comparison = { a: ProgressPhoto | null; b: ProgressPhoto | null };

export function ProgressPhotos({ userId, timeZone, onBusy }: {
  userId: string; timeZone: string; onBusy: (busy: boolean) => void;
}) {
  const [photos, setPhotos] = useState<ProgressPhoto[]>([]);
  const [nextBefore, setNextBefore] = useState<string | null>(null);
  const [photosEnabled, setPhotosEnabled] = useState<boolean | null>(null);
  const [pendingDeletions, setPendingDeletions] = useState(0);
  const [filter, setFilter] = useState<ProgressView | ''>('');
  const [loading, setLoading] = useState(false);
  const [listError, setListError] = useState('');
  const [draft, setDraft] = useState<DraftPhoto | null>(null);
  const [date, setDate] = useState(() => todayIn(timeZone));
  const [view, setView] = useState<ProgressView>('front');
  const [dateError, setDateError] = useState('');
  const [focused, setFocused] = useState(false);
  const [picking, setPicking] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [discarding, setDiscarding] = useState(false);
  const [deletePhoto, setDeletePhoto] = useState<ProgressPhoto | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteUncertain, setDeleteUncertain] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [comparison, setComparison] = useState<Comparison>({ a: null, b: null });
  const [enlarged, setEnlarged] = useState<ProgressPhoto | null>(null);
  const pending = useRef<PendingUpload | null>(null);
  const draftRef = useRef<DraftPhoto | null>(null);
  const mutationLock = useRef(false);
  const alive = useRef(true);
  const listRequest = useRef<AbortController | null>(null);
  const busyCallback = useRef(onBusy);
  busyCallback.current = onBusy;
  const busy = !!draft || picking || saving || !!deletePhoto || !!enlarged;
  const controlsLocked = busy || loading;

  useEffect(() => { busyCallback.current(busy); }, [busy]);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false; listRequest.current?.abort();
      if (draftRef.current) discardPhoto(draftRef.current);
      busyCallback.current(false);
    };
  }, []);
  useEffect(() => {
    void loadPhotos(filter);
    return () => listRequest.current?.abort();
  }, [userId, filter]);
  useEffect(() => {
    if (Platform.OS !== 'web' || (!draft && !deleteUncertain)) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [draft, deleteUncertain]);

  async function loadPhotos(selectedFilter: ProgressView | '', before?: string) {
    listRequest.current?.abort();
    const controller = new AbortController(); listRequest.current = controller;
    setLoading(true); setListError('');
    const query = [selectedFilter && `view=${selectedFilter}`, before && `before=${encodeURIComponent(before)}`].filter(Boolean).join('&');
    try {
      const result = await api<ProgressPhotoPage>(`/v1/progress-photos${query ? `?${query}` : ''}`, { userId, signal: controller.signal });
      if (!alive.current || controller.signal.aborted) return;
      setPhotos(previous => before ? mergeProgressPhotos(previous, result.photos) : result.photos);
      setNextBefore(result.next_before); setPhotosEnabled(result.photos_enabled); setPendingDeletions(result.pending_deletions);
    } catch (reason) {
      if (alive.current && !controller.signal.aborted) setListError(errorMessage(reason));
    } finally { if (alive.current && !controller.signal.aborted) setLoading(false); }
  }

  async function choosePhoto(camera: boolean) {
    if (controlsLocked || mutationLock.current || !photosEnabled) return;
    mutationLock.current = true; setPicking(true); setError(''); setNotice('');
    try {
      const selected = await pickPhotos(camera, 1);
      if (!alive.current) { selected.forEach(discardPhoto); return; }
      if (selected[0]) {
        draftRef.current = selected[0]; setDraft(selected[0]); setDate(todayIn(timeZone)); setDateError('');
      }
    } catch (reason) { if (alive.current) setError(errorMessage(reason)); }
    finally { mutationLock.current = false; if (alive.current) setPicking(false); }
  }

  function clearDraft() {
    if (pending.current || mutationLock.current) return;
    if (draftRef.current) discardPhoto(draftRef.current);
    draftRef.current = null; setDraft(null); setDiscarding(false); setDateError(''); setError('');
  }

  async function savePhoto() {
    if (mutationLock.current || !draft) return;
    if (!pending.current) {
      const validation = progressDateError(date.trim(), todayIn(timeZone));
      setDateError(validation);
      if (validation) return;
    }
    mutationLock.current = true; setSaving(true); setError(''); setNotice('');
    try {
      if (!pending.current) pending.current = { photo: draft, date: date.trim(), view, bytes: await readPhotoBytes(draft) };
      const upload = pending.current;
      const result = await api<{ photo: ProgressPhoto }>(`/v1/progress-photos/${upload.photo.id}?date=${upload.date}&view=${upload.view}`, {
        userId, method: 'PUT', rawBody: upload.bytes, timeout: 70000,
      });
      if (!alive.current) return;
      pending.current = null; discardPhoto(upload.photo); draftRef.current = null; setDraft(null);
      setNotice(`Foto vom ${dateLabel(result.photo.date)} gespeichert.${filter && filter !== result.photo.view ? ' Wähle die passende Perspektive oder „Alle“, um es im Verlauf zu sehen.' : ''}`);
      if (!filter || filter === result.photo.view) setPhotos(previous => [result.photo, ...previous.filter(photo => photo.id !== result.photo.id)]);
      void loadPhotos(filter);
    } catch (reason) {
      if (!alive.current) return;
      if (reason instanceof APIError && [400, 401, 403, 404, 409, 413, 415, 422].includes(reason.status)) pending.current = null;
      setError(errorMessage(reason));
    } finally { mutationLock.current = false; if (alive.current) setSaving(false); }
  }

  function askToDelete(photo: ProgressPhoto) {
    if (controlsLocked || mutationLock.current) return;
    setDeletePhoto(photo); setDeleteError(''); setDeleteUncertain(false); setNotice('');
  }

  async function removePhoto() {
    if (!deletePhoto || mutationLock.current) return;
    mutationLock.current = true; setDeleting(true); setDeleteError('');
    try {
      try { await api(`/v1/progress-photos/${deletePhoto.id}`, { userId, method: 'DELETE' }); }
      catch (reason) { if (!(reason instanceof APIError && reason.status === 404)) throw reason; }
      if (!alive.current) return;
      const id = deletePhoto.id;
      setPhotos(previous => previous.filter(photo => photo.id !== id));
      setComparison(previous => ({ a: previous.a?.id === id ? null : previous.a, b: previous.b?.id === id ? null : previous.b }));
      setDeletePhoto(null); setDeleteUncertain(false); setNotice('Fortschrittsfoto entfernt.');
      void loadPhotos(filter);
    } catch (reason) {
      if (!alive.current) return;
      setDeleteUncertain(!(reason instanceof APIError && [400, 401, 403, 409].includes(reason.status)));
      setDeleteError(errorMessage(reason));
    } finally { mutationLock.current = false; if (alive.current) setDeleting(false); }
  }

  function selectPhoto(slot: keyof Comparison, photo: ProgressPhoto) {
    if (controlsLocked) return;
    const other = slot === 'a' ? 'b' : 'a';
    setComparison(previous => ({ ...previous, [slot]: previous[slot]?.id === photo.id ? null : photo, [other]: previous[other]?.id === photo.id ? null : previous[other] }));
  }

  const comparisonReady = !!comparison.a && !!comparison.b;
  return <View style={s.screen} testID="progress-photos">
    <View style={s.form}>
      <Text role="heading" aria-level={1} style={s.heading}>Deine Fortschrittsfotos</Text>
      <Text style={s.hint}>Halte Veränderungen in deinem Tempo fest. Mit ähnlicher Perspektive, Beleuchtung und Entfernung lassen sich Aufnahmen leichter vergleichen.</Text>
      <Text style={s.hint}>Die Fotos sind privat in deinem Konto gespeichert. Sie erscheinen nicht im Tageschat und werden nicht an die KI geschickt.</Text>
    </View>
    <View style={s.section}>
      <Text role="heading" aria-level={2} style={s.subheading}>Eine neue Aufnahme</Text>
      {photosEnabled === false && <Text style={s.hint}>Der Fotospeicher ist derzeit nicht verfügbar. Neue Aufnahmen können noch nicht gespeichert werden.</Text>}
      {!draft && <View style={s.actions}>
        <Button label={picking ? 'Foto wird vorbereitet …' : 'Foto auswählen'} disabled={controlsLocked || !photosEnabled} onPress={() => void choosePhoto(false)} />
        <Button secondary label="Foto aufnehmen" disabled={controlsLocked || !photosEnabled} onPress={() => void choosePhoto(true)} />
      </View>}
      {draft && <View style={s.form}>
        <Image source={{ uri: draft.uri }} style={s.draftImage} resizeMode="contain" accessibilityLabel="Ausgewähltes Fortschrittsfoto, noch nicht gespeichert" />
        <Text style={s.hint}>Prüfe Datum und Perspektive, bevor du speicherst. Diese Auswahl bleibt nur in der geöffneten Ansicht erhalten.</Text>
        <View style={s.field}>
          <Text style={s.label}>Aufnahmedatum</Text>
          <TextInput accessibilityLabel="Aufnahmedatum" accessibilityHint="Datum als JJJJ-MM-TT, zum Beispiel 2026-09-17" aria-invalid={!!dateError}
            value={date} onChangeText={text => { if (!saving && !pending.current) { setDate(text); setDateError(''); } }} editable={!saving && !pending.current}
            autoCorrect={false} autoCapitalize="none" maxLength={10} placeholder="JJJJ-MM-TT" placeholderTextColor="#687061"
            onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} style={[s.input, !!dateError && s.invalid, focused && s.focus]} />
          {!!dateError && <Text accessibilityRole="alert" style={s.error}>{dateError}</Text>}
        </View>
        <View style={s.field}>
          <Text style={s.label}>Perspektive</Text>
          <View style={s.actions}>{progressViews.map(option => <Button key={option.value} variant="chip" label={option.label} accessibilityLabel={`Aufnahme: ${option.label}`}
            selected={view === option.value} disabled={saving || !!pending.current} onPress={() => setView(option.value)} />)}</View>
        </View>
        {!!pending.current && !saving && <Text accessibilityLiveRegion="polite" style={s.hint}>Die Bestätigung steht noch aus. Versuche denselben Upload erneut. Foto, Datum und Perspektive bleiben bis dahin gesperrt.</Text>}
        <View style={s.actions}>
          <Button label={saving ? 'Foto wird gespeichert …' : pending.current ? 'Upload erneut versuchen' : 'Fortschrittsfoto speichern'} disabled={saving} onPress={() => void savePhoto()} />
          {!pending.current && <Button secondary label="Auswahl entfernen" disabled={saving} onPress={() => setDiscarding(true)} />}
        </View>
      </View>}
      <ErrorNotice text={error} />
      {!!notice && <Text accessibilityLiveRegion="polite" style={s.success}>{notice}</Text>}
    </View>
    <View style={s.section}>
      <Text role="heading" aria-level={2} style={s.subheading}>Dein Vergleich</Text>
      <Text style={s.hint}>{comparisonReady ? 'Beide Aufnahmen werden vollständig angezeigt. Tippe auf ein Foto, um es zu vergrößern.' : 'Wähle im Verlauf zwei verschiedene Fotos als A und B aus. Du kannst die Perspektiven getrennt filtern.'}</Text>
      {(comparison.a || comparison.b) && <View style={s.comparison}>
        {(['a', 'b'] as const).map(slot => {
          const photo = comparison[slot];
          return <View style={s.comparisonItem} key={slot}>
            <Text style={s.label}>Foto {slot.toUpperCase()}</Text>
            {photo ? <>
              <View style={s.comparisonCaption}><Text style={s.small}>{progressDateLabel(photo.date)}</Text><Text style={s.small}>{progressViewLabel(photo.view)}</Text></View>
              <PrivatePhoto photo={photo} userId={userId} comparison onOpen={() => setEnlarged(photo)} disabled={controlsLocked} />
              <Button secondary label={`${slot.toUpperCase()} lösen`} accessibilityLabel={`Foto ${slot.toUpperCase()} aus Vergleich entfernen`} disabled={controlsLocked}
                onPress={() => setComparison(previous => ({ ...previous, [slot]: null }))} />
            </> : <Text style={s.small}>Noch kein Foto gewählt</Text>}
          </View>;
        })}
      </View>}
      {comparisonReady && comparison.a!.view !== comparison.b!.view && <Text style={s.hint}>Diese Aufnahmen zeigen unterschiedliche Perspektiven.</Text>}
    </View>
    <View style={s.form}>
      <Text role="heading" aria-level={2} style={s.subheading}>Dein Verlauf</Text>
      <View style={s.actions}>
        <Button variant="chip" label="Alle" accessibilityLabel="Alle Perspektiven" selected={!filter} disabled={controlsLocked} onPress={() => setFilter('')} />
        {progressViews.map(option => <Button key={option.value} variant="chip" label={option.label} accessibilityLabel={`Perspektive filtern: ${option.label}`}
          selected={filter === option.value} disabled={controlsLocked} onPress={() => setFilter(option.value)} />)}
      </View>
      {pendingDeletions > 0 && <Text accessibilityLiveRegion="polite" style={s.hint}>Entfernte Fotos sind im Verlauf ausgeblendet. Der Fotospeicher wird noch bereinigt.</Text>}
      <ErrorNotice text={listError} />
      {!loading && !listError && !photos.length && <Text style={s.hint}>{filter ? 'Für diese Perspektive gibt es noch keine Aufnahmen.' : 'Dein Verlauf beginnt mit deiner ersten Aufnahme.'}</Text>}
      <View style={s.grid}>{photos.map(photo => <View key={photo.id} style={[s.card, (comparison.a?.id === photo.id || comparison.b?.id === photo.id) && s.selectedCard]}>
        <Text style={s.label}>{dateLabel(photo.date, true)}</Text>
        <Text style={s.small}>{progressViewLabel(photo.view)}</Text>
        <PrivatePhoto photo={photo} userId={userId} onOpen={() => setEnlarged(photo)} disabled={controlsLocked} />
        <View style={s.actions}>
          <Button variant="chip" label="Foto A" accessibilityLabel={`Als Foto A: ${photo.date}, ${progressViewLabel(photo.view)}`} selected={comparison.a?.id === photo.id} disabled={controlsLocked} onPress={() => selectPhoto('a', photo)} />
          <Button variant="chip" label="Foto B" accessibilityLabel={`Als Foto B: ${photo.date}, ${progressViewLabel(photo.view)}`} selected={comparison.b?.id === photo.id} disabled={controlsLocked} onPress={() => selectPhoto('b', photo)} />
          <Button variant="ghost" label="Entfernen" accessibilityLabel={`Foto entfernen: ${photo.date}, ${progressViewLabel(photo.view)}`} disabled={controlsLocked} onPress={() => askToDelete(photo)} />
        </View>
      </View>)}</View>
      {loading && <Text accessibilityLiveRegion="polite" style={s.hint}>Aufnahmen werden geladen …</Text>}
      <View style={s.actions}>
        {nextBefore && <Button secondary label="Ältere Fotos laden" disabled={controlsLocked} onPress={() => void loadPhotos(filter, nextBefore)} />}
        <Button secondary label={listError ? 'Verlauf erneut laden' : 'Verlauf aktualisieren'} disabled={controlsLocked} onPress={() => void loadPhotos(filter)} />
      </View>
    </View>
    <Modal visible={discarding} transparent animationType="none" onRequestClose={() => setDiscarding(false)}>
      <View style={s.modalBackdrop} accessibilityViewIsModal><View style={s.modalCard}>
        <Text role="heading" aria-level={2} style={s.subheading}>Auswahl entfernen?</Text>
        <Text style={s.hint}>Dieses Foto wurde noch nicht gespeichert. Du kannst es später erneut auswählen.</Text>
        <View style={s.actions}>
          <Button secondary label="Foto behalten" onPress={() => setDiscarding(false)} />
          <Button label="Auswahl jetzt entfernen" onPress={clearDraft} />
        </View>
      </View></View>
    </Modal>
    <Modal visible={!!deletePhoto} transparent animationType="none" onRequestClose={() => { if (!deleting && !deleteUncertain) setDeletePhoto(null); }}>
      <View style={s.modalBackdrop} accessibilityViewIsModal><ScrollView style={s.modalCard} contentContainerStyle={s.form}>
        <Text role="heading" aria-level={2} style={s.subheading}>Foto entfernen?</Text>
        {deletePhoto && <Text style={s.label}>{dateLabel(deletePhoto.date, true)} · {progressViewLabel(deletePhoto.view)}</Text>}
        <Text style={s.hint}>Das Foto wird aus deinem Verlauf und dem Vergleich entfernt. Dies lässt sich nicht rückgängig machen.</Text>
        {deleteUncertain && <Text style={s.hint}>Die Bestätigung steht noch aus. Wiederhole das Entfernen, um den gespeicherten Stand zu bestätigen.</Text>}
        <ErrorNotice text={deleteError} />
        <View style={s.actions}>
          {!deleteUncertain && <Button secondary label="Foto behalten" disabled={deleting} onPress={() => setDeletePhoto(null)} />}
          <Button label={deleting ? 'Foto wird entfernt …' : deleteUncertain ? 'Entfernen erneut versuchen' : 'Foto endgültig entfernen'} disabled={deleting} onPress={() => void removePhoto()} />
        </View>
      </ScrollView></View>
    </Modal>
    <Modal visible={!!enlarged} animationType="none" onRequestClose={() => setEnlarged(null)}>
      <View style={s.fullScreen} accessibilityViewIsModal>
        <Button label="Foto schließen" onPress={() => setEnlarged(null)} />
        {enlarged && <><Text style={s.label}>{dateLabel(enlarged.date, true)} · {progressViewLabel(enlarged.view)}</Text><PrivatePhoto photo={enlarged} userId={userId} full /></>}
      </View>
    </Modal>
  </View>;
}

function PrivatePhoto({ photo, userId, comparison = false, full = false, onOpen, disabled = false }: {
  photo: ProgressPhoto; userId: string; comparison?: boolean; full?: boolean; onOpen?: () => void; disabled?: boolean;
}) {
  const [uri, setURI] = useState('');
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    setURI(''); setError('');
    async function load() {
      try {
        const result = await api<{ path: string; expires_in: number }>(`/v1/progress-photos/${photo.id}/url`, { userId, signal: controller.signal });
        if (controller.signal.aborted) return;
        setURI(`${process.env.EXPO_PUBLIC_SUPABASE_URL}${result.path}`); setError('');
        timer = setTimeout(() => void load(), Math.max(30, result.expires_in - 60) * 1000);
      } catch (reason) { if (!controller.signal.aborted) setError(errorMessage(reason)); }
    }
    void load();
    return () => { controller.abort(); clearTimeout(timer); };
  }, [photo.id, userId, revision]);
  const label = `Fortschrittsfoto vom ${progressDateLabel(photo.date)}, ${progressViewLabel(photo.view)}`;
  if (error || !uri) return <View style={s.imagePlaceholder}>
    <Text style={s.small}>{error || 'Foto wird geladen …'}</Text>
    {!!error && <Button secondary label="Bild erneut laden" onPress={() => setRevision(value => value + 1)} />}
  </View>;
  const image = <Image source={{ uri }} style={full ? s.fullImage : comparison ? s.comparisonImage : s.thumbnail}
    resizeMode="contain" accessibilityLabel={label} onError={() => setError('Das Foto konnte nicht geladen werden.')} />;
  if (full) return image;
  return <Pressable accessibilityRole="button" accessibilityLabel={`${label} vergrößern`} accessibilityState={{ disabled }}
    disabled={disabled} onPress={onOpen} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} style={[s.imageButton, focused && s.focus]}>{image}</Pressable>;
}
