import { useEffect, useRef, useState, type ReactNode } from 'react';
import { AppState, Image, Keyboard, Platform, ScrollView, Text, TextInput, useWindowDimensions, View } from 'react-native';
import { randomUUID } from 'expo-crypto';
import { api, APIError, Entry, errorMessage, Message, MessagePage, Summary } from '../lib/chat-api';
import { styles as s } from '../styles/app';
import { Button, ErrorNotice, FittyMark } from './ui';
import { DaySummary } from './day-summary';
import { DraftPhoto, discardPhoto, pastedPhotos, pickPhotos, uploadPhoto } from '../lib/photos';
import { ChatPhoto } from './chat-photo';
import { EntryEditor, EntryEditorMode } from './entry-editor';
import { FormScrollView, useKeyboardToolbar } from './keyboard-layout';
import { ComposerActions, type ComposerAction } from './composer-actions';
import type { useFavorites } from '../hooks/use-favorites';
import { favoriteMatches, insertFavorite, type Favorite, type FavoriteMatch } from '../lib/favorites';
import { FavoriteSuggestions, FavoritesSheet } from './favorites';

import type { PendingMessage, StoredDraft } from '../lib/draft-store.types';
const analysisErrors: Record<string, string> = {
  refused: 'Fitty konnte diese Nachricht nicht beantworten.',
  incomplete: 'Die Antwort war unvollständig. Es wurde nichts gebucht.',
  invalid_result: 'Die Auswertung war nicht eindeutig genug. Es wurde nichts gebucht.',
  configuration: 'Die KI-Verbindung ist nicht verfügbar. Bitte die Einrichtung prüfen.',
  timeout: 'Die Auswertung hat zu lange gedauert.',
  interrupted: 'Die Auswertung wurde unterbrochen.',
};
const statusLabels = {
  saved: 'gespeichert', queued: 'wartet auf Auswertung', processing: 'wird ausgewertet',
  completed: 'ausgewertet', failed: 'Auswertung fehlgeschlagen', disabled: 'noch nicht ausgewertet', skipped: 'ohne Auswertung',
};
function mergeMessages(previous: Message[], incoming: Message[]) {
  const all = new Map(previous.map(message => [message.id, message]));
  incoming.forEach(message => all.set(message.id, message));
  return [...all.values()].sort((a, b) => BigInt(a.id) < BigInt(b.id) ? -1 : 1);
}

export function Chat({ favorites, userId, date, timeZone, draft, setDraft, photos, setPhotos, pending, prepareSend, completeSend, rejectSend, draftNotice, sendBlocked, setBusy, onSaved, onTargets, onDiscardDraft, externalBusy = false, initialPendingPhotoDeletions = 0 }: {
  userId: string; date: string; timeZone: string; draft: string; setDraft: (value: string) => void;
  photos: DraftPhoto[]; setPhotos: (value: DraftPhoto[]) => void;
  pending: PendingMessage | null; prepareSend: (pending: PendingMessage) => Promise<StoredDraft>; completeSend: () => Promise<void>; rejectSend: () => Promise<unknown>;
  draftNotice?: ReactNode; sendBlocked?: boolean; setBusy: (value: boolean) => void; onSaved: () => void; onTargets: () => void;
  favorites: ReturnType<typeof useFavorites>;
  externalBusy?: boolean; initialPendingPhotoDeletions?: number; onDiscardDraft?: () => void;
}) {
  const { width } = useWindowDimensions();
  const wide = width >= 900;
  useKeyboardToolbar(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [next, setNext] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [olderLoading, setOlderLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [sendError, setSendError] = useState('');
  const [sending, setSending] = useState(false);
  const [picking, setPicking] = useState(false);
  const [uploadStatus, setUploadStatus] = useState('');
  const [actionBusy, setActionBusy] = useState('');
  const [revision, setRevision] = useState(0);
  const [inputFocused, setInputFocused] = useState(false);
  const [actionsOpen, setActionsOpen] = useState(false);
  const [favoritesOpen, setFavoritesOpen] = useState(false);
  const [selection, setSelection] = useState({ start: draft.length, end: draft.length });
  const [completion, setCompletion] = useState<{ text: string; caret: number } | null>(null);
  const [editor, setEditor] = useState<{ entry: Entry; mode: EntryEditorMode } | null>(null);
  const scroll = useRef<ScrollView>(null);
  const input = useRef<TextInput>(null);
  const alive = useRef(true);
  const scrollToBottom = useRef(true);
  const sendLock = useRef(false);
  const refresh = useRef<() => void>(() => {});
  const savedCallback = useRef(onSaved);
  savedCallback.current = onSaved;
  const paginated = useRef(false);
  const dayIdentity = useRef<string | null | undefined>(undefined);
  const dayGeneration = useRef(0);
  const requestGeneration = useRef(0);

  useEffect(() => {
    if (!completion || draft !== completion.text) return;
    // Focus after the suggestion has unmounted, then put the caret immediately
    // after the inserted portion so existing trailing text stays editable.
    input.current?.focus();
    if (Platform.OS === 'web') {
      (input.current as unknown as HTMLTextAreaElement | null)?.setSelectionRange(completion.caret, completion.caret);
    } else {
      input.current?.setSelection(completion.caret, completion.caret);
    }
  }, [completion, draft]);

  useEffect(() => {
    if (Platform.OS !== 'web') return;
    const element = input.current as unknown as HTMLTextAreaElement | null;
    const paste = (event: ClipboardEvent) => {
      const files = Array.from(event.clipboardData?.files ?? []).filter(file => file.type.startsWith('image/'));
      if (!files.length) return;
      event.preventDefault();
      if (externalBusy || sending || picking || editor || pending || !summary?.photos_enabled) return;
      if (files.length + photos.length > 4) { setSendError('Du kannst höchstens vier Bilder pro Nachricht einfügen.'); return; }
      setPicking(true); setBusy(true); setSendError('');
      void pastedPhotos(files).then(selected => {
        if (alive.current) setPhotos([...photos, ...selected]); else selected.forEach(discardPhoto);
      }).catch(() => { if (alive.current) setSendError('Das eingefügte Bild konnte nicht geöffnet werden. Bitte ein JPEG, PNG oder WebP mit höchstens 20 MiB verwenden.'); })
        .finally(() => { if (alive.current) { setPicking(false); setBusy(false); } });
    };
    element?.addEventListener('paste', paste);
    return () => element?.removeEventListener('paste', paste);
  }, [date, editor, externalBusy, photos, pending, picking, sending, summary?.photos_enabled, setBusy, setPhotos]);

  useEffect(() => {
    alive.current = true;
    ++requestGeneration.current;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let fetching = false;
    let lastReply = '';
    setLoading(true); setOlderLoading(false);
    async function poll() {
      if (controller.signal.aborted || fetching) return;
      fetching = true;
      clearTimeout(timer);
      let retrySoon = false;
      try {
        const [page, state] = await Promise.all([
          api<MessagePage>(`/v1/days/${date}/messages`, { signal: controller.signal }),
          api<Summary>(`/v1/days/${date}/summary`, { signal: controller.signal }),
        ]);
        if (controller.signal.aborted) return;
        const identity = page.day_id ?? null;
        if (identity !== (state.day?.id ?? null)) { retrySoon = true; return; }
        const sameDay = dayIdentity.current === identity;
        if (!sameDay) {
          const previouslyLoaded = dayIdentity.current !== undefined;
          dayIdentity.current = identity; ++dayGeneration.current; paginated.current = false;
          setOlderLoading(false); scrollToBottom.current = true; lastReply = '';
          if (previouslyLoaded) savedCallback.current();
        }
        const states = new Map(state.analysis.map(item => [item.message_id, item]));
        setMessages(previous => (sameDay ? mergeMessages(previous, page.messages) : page.messages).map(message => {
          const job = states.get(message.id);
          return job ? { ...message, analysis_status: job.status, analysis_error: job.error, analysis_attempts: job.attempts } : message;
        }));
        setSummary(state);
        if (!paginated.current) setNext(page.next_before);
        setLoadError('');
        const latestReply = [...page.messages].reverse().find(message => message.role === 'assistant')?.id ?? '';
        if (lastReply && latestReply !== lastReply) { scrollToBottom.current = true; savedCallback.current(); }
        lastReply = latestReply || 'none';
      } catch (error) { if (!controller.signal.aborted) setLoadError(errorMessage(error)); }
      finally {
        fetching = false;
        if (!controller.signal.aborted) { setLoading(false); timer = setTimeout(() => void poll(), retrySoon ? 150 : AppState.currentState === 'active' ? 2500 : 10000); }
      }
    }
    refresh.current = () => void poll();
    const subscription = AppState.addEventListener('change', state => { if (state === 'active') void poll(); });
    void poll();
    return () => { alive.current = false; ++requestGeneration.current; clearTimeout(timer); controller.abort(); subscription.remove(); refresh.current = () => {}; };
  }, [date, revision]);

  async function older() {
    if (!next || loading || olderLoading || externalBusy) return;
    const generation = requestGeneration.current;
    const currentDay = dayGeneration.current;
    const identity = dayIdentity.current;
    setOlderLoading(true);
    try {
      const result = await api<MessagePage>(`/v1/days/${date}/messages?before=${next}`);
      if (!alive.current || generation !== requestGeneration.current || currentDay !== dayGeneration.current) return;
      if ((result.day_id ?? null) !== identity) { refresh.current(); return; }
      paginated.current = true; scrollToBottom.current = false; setMessages(previous => mergeMessages(previous, result.messages)); setNext(result.next_before);
    } catch (error) { if (alive.current && generation === requestGeneration.current && currentDay === dayGeneration.current) setLoadError(errorMessage(error)); }
    finally { if (alive.current && generation === requestGeneration.current && currentDay === dayGeneration.current) setOlderLoading(false); }
  }
  async function send() {
    if (externalBusy || sendBlocked || sendLock.current || picking || editor || (!draft.trim() && !photos.length) || loading || loadError) return;
    const currentDay = dayGeneration.current;
    sendLock.current = true; setSending(true); setBusy(true); setSendError('');
    try {
      const saved = await prepareSend(pending ?? { client_id: randomUUID(), content: draft.trim(), attachment_ids: photos.map(photo => photo.id) });
      if (!alive.current) return;
      const item = saved.pending!;
      for (const [index, photo] of saved.photos.entries()) {
        setUploadStatus(`Bild ${index + 1} von ${photos.length} wird hochgeladen …`);
        await uploadPhoto(date, photo, userId);
      }
      setUploadStatus('');
      const result = await api<{ message: Message }>(`/v1/days/${date}/messages`, { method: 'POST', body: item, userId });
      if (!alive.current) return;
      scrollToBottom.current = true;
      if (currentDay === dayGeneration.current) setMessages(previous => mergeMessages(previous, [result.message]));
      await completeSend(); onSaved(); refresh.current();
    } catch (error) {
      if (error instanceof APIError && [400, 409].includes(error.status)) { try { await rejectSend(); } catch { /* Keep the durable retry body when storage is unavailable. */ } }
      if (alive.current) setSendError(errorMessage(error));
    } finally {
      sendLock.current = false;
      if (alive.current) { setSending(false); setBusy(false); setUploadStatus(''); setTimeout(() => input.current?.focus(), 0); }
    }
  }
  async function selectPhotos(camera: boolean) {
    if (externalBusy || sendLock.current || picking || editor || pending || photos.length >= 4) return;
    Keyboard.dismiss();
    setPicking(true); setBusy(true); setSendError('');
    try { const selected = await pickPhotos(camera, 4 - photos.length); if (alive.current) setPhotos([...photos, ...selected]); else selected.forEach(discardPhoto); }
    catch (error) { if (alive.current) setSendError(errorMessage(error)); }
    finally { if (alive.current) { setPicking(false); setBusy(false); } }
  }
  async function removePhoto(photo: DraftPhoto) {
    if (externalBusy || sendLock.current || picking || editor || pending) return;
    setPicking(true); setBusy(true); setSendError('');
    try {
      setPhotos(photos.filter(item => item.id !== photo.id));
      // Uploads only start after a durable pending request exists. Any old
      // unbound upload is also covered by the server's 24-hour cleanup.
      void api(`/v1/attachments/${photo.id}`, { method: 'DELETE', userId }).catch(() => {});
    } catch (error) { if (alive.current) setSendError(errorMessage(error)); }
    finally { if (alive.current) { setPicking(false); setBusy(false); } }
  }
  async function analysisAction(message: Message, action: 'retry' | 'skip') {
    if (externalBusy || actionBusy || editor) return;
    setActionBusy(message.id); setSendError('');
    try {
      await api(`/v1/messages/${message.id}/analysis/${action}`, { method: 'POST' });
      if (alive.current) refresh.current();
    } catch (error) { if (alive.current) setSendError(errorMessage(error)); }
    finally { if (alive.current) setActionBusy(''); }
  }

  function appendDraft(text: string) {
    if (externalBusy || sending || picking || editor || pending) return;
    const value = draft ? `${draft}\n${text}` : text;
    if (value.length > 8000) { setSendError('Bitte kürze deinen Entwurf, bevor du etwas ergänzt.'); return; }
    setDraft(value); setSendError(''); setTimeout(() => input.current?.focus(), 0);
  }
  function chooseAction(action: ComposerAction) {
    if (!alive.current) return;
    if (action === 'photos' || action === 'camera') { void selectPhotos(action === 'camera'); return; }
    if (action === 'discard') { onDiscardDraft?.(); return; }
    if (action === 'favorites') { Keyboard.dismiss(); setFavoritesOpen(true); return; }
    appendDraft({ meal: 'Ich habe gegessen: ', activity: 'Meine Bewegung heute: ', restaurant: 'Ich gehe heute ins Restaurant. Worauf sollte ich achten? ' }[action]);
  }
  function chooseFavorite(favorite: Favorite, match?: FavoriteMatch) {
    if (externalBusy || sending || picking || editor || pending) return;
    const value = insertFavorite(draft, favorite, match);
    if (value.length > 8000) { setSendError('Bitte kürze deinen Entwurf, bevor du einen Favoriten ergänzt.'); return; }
    const caret = match ? value.length - (draft.length - match.end) : value.length;
    setDraft(value); setCompletion({ text: value, caret }); setSelection({ start: caret, end: caret }); setSendError('');
  }
  const matches = favorites.ready && !externalBusy && !sending && !picking && !editor && !pending && !actionsOpen && !favoritesOpen && draft !== completion?.text
    ? favoriteMatches(favorites.favorites, draft, selection) : [];
  function correct(entry: Entry) {
    appendDraft(`Korrektur zu „${entry.label}“${entry.amount ? ` (${entry.amount})` : ''}: `);
  }
  function openEditor(entry: Entry, mode: EntryEditorMode) {
    if (externalBusy || sendLock.current || picking || editor || pending || actionBusy || loading || loadError || summary?.pending_count) return;
    Keyboard.dismiss(); setEditor({ entry: { ...entry }, mode }); setBusy(true);
  }
  function closeEditor() { setEditor(null); setBusy(false); }
  useEffect(() => {
    if (Platform.OS !== 'web') return;
    const element = input.current as unknown as HTMLTextAreaElement | null;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !event.isComposing) { event.preventDefault(); void send(); }
    };
    element?.addEventListener('keydown', onKey);
    return () => element?.removeEventListener('keydown', onKey);
  });

  return <View style={s.fill}>
    <ScrollView ref={scroll} style={s.fill} contentContainerStyle={[s.messages, wide && s.messagesWide]} keyboardShouldPersistTaps="handled" keyboardDismissMode={Platform.OS === 'web' ? 'none' : Platform.OS === 'ios' ? 'interactive' : 'on-drag'}
      onContentSizeChange={() => { if (scrollToBottom.current && messages.length) { scroll.current?.scrollToEnd({ animated: false }); scrollToBottom.current = false; } }}>
      {summary && <DaySummary summary={summary} onTargets={externalBusy || sending || picking || editor ? undefined : onTargets} onCorrect={externalBusy || sending || picking || editor || pending ? undefined : correct}
        favorites={favorites.favorites} favoriteBusy={favorites.busy}
        onFavorite={externalBusy || sending || picking || editor || loading || loadError || !favorites.ready ? undefined : entry => void favorites.toggle(entry)}
        onEdit={externalBusy || sending || picking || editor || pending || actionBusy || loading || loadError || summary.pending_count ? undefined : entry => openEditor(entry, 'edit')}
        onDelete={externalBusy || sending || picking || editor || pending || actionBusy || loading || loadError || summary.pending_count ? undefined : entry => openEditor(entry, 'delete')} />}
      {!!favorites.error && <View style={s.field}><ErrorNotice text={favorites.error} /><Button variant="ghost" label="Favoriten erneut laden" onPress={favorites.reload} /></View>}
      {!!(summary?.pending_photo_deletions ?? initialPendingPhotoDeletions) && <Text accessibilityLiveRegion="polite" style={s.small}>Fotos werden im Hintergrund entfernt.</Text>}
      {summary && !summary.ai_enabled && <Text style={s.small}>Die KI ist noch nicht verbunden. Deine Nachrichten bleiben als Tagebuch gespeichert und können später ausgewertet werden.</Text>}
      <View style={s.divider}><View style={s.dividerLine} /><Text style={s.eyebrow}>Gespräch</Text><View style={s.dividerLine} /></View>
      {next && <Button secondary label="Ältere Nachrichten laden" disabled={externalBusy || loading || olderLoading || sending} onPress={() => void older()} />}
      {(loading || olderLoading) && <Text style={s.small}>Nachrichten werden geladen …</Text>}
      <ErrorNotice text={loadError} />{!!loadError && <Button secondary label="Nachrichten erneut laden" disabled={externalBusy} onPress={() => setRevision(n => n + 1)} />}
      {!loading && !loadError && !messages.length && <View style={s.empty}>
        <FittyMark />
        <Text role="heading" aria-level={2} style={[s.title, s.emptyTitle]}>Was gehört zu deinem Tag?</Text>
        <Text style={[s.subtitle, s.emptyText]}>Dein Essen, deine Bewegung oder eine kleine Frage. Schreib einfach los – ein Foto geht auch.</Text>
      </View>}
      {messages.map(message => <View key={message.id} testID="chat-message" style={[s.message, wide && s.messageWide, message.role === 'assistant' && s.assistantMessage]}>
        {message.role === 'assistant' && <View style={s.inline}><FittyMark small /><Text style={s.assistantLabel}>Fitty</Text></View>}
        {!!message.attachments?.length && <View style={s.row}>{message.attachments.map((attachment, index) => <ChatPhoto key={attachment.id} attachment={attachment} index={index} />)}</View>}
        {!!message.content && <Text selectable style={s.messageText}>{message.content}</Text>}
        <Text style={s.messageTime}>{new Intl.DateTimeFormat('de-DE', { timeZone, hour: '2-digit', minute: '2-digit' }).format(new Date(message.created_at))} · {message.role === 'assistant' ? 'gespeichert' : statusLabels[message.analysis_status]}</Text>
        {message.analysis_status === 'failed' && <Text style={s.error}>{analysisErrors[message.analysis_error] ?? 'Fitty ist gerade nicht erreichbar. Deine Nachricht ist gespeichert.'}</Text>}
        {message.role === 'user' && ['failed', 'disabled', 'saved'].includes(message.analysis_status) && <View style={s.row}>
          {summary?.ai_enabled && message.analysis_attempts < 3 && <Button secondary label={actionBusy === message.id ? 'Bitte warten …' : message.analysis_status === 'failed' ? 'Erneut auswerten' : 'Auswerten'} disabled={externalBusy || !!actionBusy || !!editor} onPress={() => void analysisAction(message, 'retry')} />}
          {message.analysis_status === 'failed' && <Button secondary label="Ohne Auswertung lassen" disabled={externalBusy || !!actionBusy || !!editor} onPress={() => void analysisAction(message, 'skip')} />}
          {message.analysis_attempts >= 3 && <Text style={s.small}>Nach drei Versuchen gestoppt. Bitte die Angabe prüfen und gegebenenfalls eine neue Nachricht schreiben.</Text>}
        </View>}
      </View>)}
      {!!summary?.pending_count && <View accessibilityLiveRegion="polite" style={s.analysisNotice}>
        <Text style={s.label}>Fitty wertet deinen Tag aus …</Text>
        <Text style={s.small}>Du kannst weiterschreiben. Bei einer fehlgeschlagenen Nachricht bitte zuerst erneut auswerten oder ohne Auswertung fortfahren.</Text>
      </View>}
    </ScrollView>
    <FormScrollView style={s.composerFooter} testID="chat-composer"><View style={[s.composerArea, wide && s.composerWide]}>
      {draftNotice}
      <ErrorNotice text={sendError} />
      {!!photos.length && <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={s.photoStrip} keyboardShouldPersistTaps="handled">
          {photos.map((photo, index) => <View key={photo.id} style={s.photoDraft} testID="draft-photo">
            <Image source={{ uri: photo.uri }} style={s.photoDraftPreview} accessibilityLabel={`Ausgewähltes Foto ${index + 1}`} />
            <Button variant="ghost" icon="close" iconOnly label="Entfernen" accessibilityLabel={`Foto ${index + 1} entfernen`} disabled={externalBusy || sending || picking || !!editor || !!pending} onPress={() => void removePhoto(photo)} />
          </View>)}
      </ScrollView>}
      {picking && <Text accessibilityLiveRegion="polite" style={s.small}>Bilder werden vorbereitet …</Text>}
      {!!uploadStatus && <Text accessibilityLiveRegion="polite" style={s.small}>{uploadStatus}</Text>}
      <FavoriteSuggestions matches={matches} onChoose={match => chooseFavorite(match.favorite, match)} />
      <View style={[s.composer, inputFocused && s.composerFocused]}>
        <Button variant="ghost" icon="plus" iconOnly round label="Hinzufügen" accessibilityLabel="Nachricht ergänzen" expanded={actionsOpen}
          disabled={externalBusy || sending || picking || !!editor || !!pending} onPress={() => { Keyboard.dismiss(); setActionsOpen(true); }} />
        <TextInput ref={input} accessibilityLabel="Nachricht" placeholder="Erzähl von deinem Tag …" placeholderTextColor="#687061" value={draft} onChangeText={value => { setDraft(value); setCompletion(null); }}
          onSelectionChange={event => setSelection(event.nativeEvent.selection)}
          onFocus={() => setInputFocused(true)} onBlur={() => setInputFocused(false)} editable={!externalBusy && !sending && !editor && !pending} multiline {...(Platform.OS === 'web' ? { rows: 1 } : {})} maxLength={8000} style={s.composerInput} />
        <Button icon={pending ? 'history' : 'send'} iconOnly round loading={sending} label={sending ? 'Speichern …' : pending ? 'Erneut senden' : 'Senden'}
          disabled={externalBusy || sendBlocked || sending || picking || !!editor || (!draft.trim() && !photos.length) || loading || !!loadError} onPress={() => void send()} />
      </View>
      {draft.length >= 7800 && <Text accessibilityLiveRegion="polite" style={s.small}>{(8000 - draft.length).toLocaleString('de-DE')} Zeichen übrig</Text>}
    </View></FormScrollView>
    <ComposerActions visible={actionsOpen} photosEnabled={!!summary?.photos_enabled} photoLimitReached={photos.length >= 4} canDiscard={!!onDiscardDraft}
      onClose={() => setActionsOpen(false)} onAction={chooseAction} />
    <FavoritesSheet visible={favoritesOpen} model={favorites} onClose={() => setFavoritesOpen(false)} onChoose={chooseFavorite} />
    {editor && <EntryEditor date={date} entry={editor.entry} mode={editor.mode} analysisBusy={!!summary?.pending_count}
      onClose={closeEditor} onRefresh={() => refresh.current()}
      onSaved={() => { closeEditor(); refresh.current(); onSaved(); }} />}
  </View>;
}
