import { useCallback, useEffect, useRef, useState } from 'react';
import { Pressable, ScrollView, Text, TextInput, useWindowDimensions, View } from 'react-native';
import { api, dateLabel, Day, DayDeletionResult, DayPage, errorMessage, moveDate, Profile, todayIn } from '../lib/chat-api';
import { supabase } from '../lib/supabase';
import { styles as s } from '../styles/app';
import { styles as deletionStyles } from '../styles/day-delete';
import { Brand, Button, ErrorNotice, Field } from './ui';
import { Chat } from './chat';
import { TargetsForm } from './targets-form';
import { ProgressPhotos } from './progress-photos';
import { useDrafts } from '../hooks/use-drafts';
import { styles as draftStyles } from '../styles/drafts';
import { DayDelete } from './day-delete';

export function Workspace({ email, userId }: { email: string; userId: string }) {
  const { width } = useWindowDimensions();
  const wide = width >= 900;
  const [profile, setProfile] = useState<Profile | null>(null);
  const [profileError, setProfileError] = useState('');
  const [revision, setRevision] = useState(0);
  const [date, setDate] = useState('');
  const [dateInput, setDateInput] = useState('');
  const [dateError, setDateError] = useState('');
  const [datePickerOpen, setDatePickerOpen] = useState(false);
  const [focusedControl, setFocusedControl] = useState('');
  const [pane, setPane] = useState<'chat' | 'history' | 'profile' | 'progress'>('chat');
  const [working, setBusy] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [chatRevision, setChatRevision] = useState(0);
  const [deletedDate, setDeletedDate] = useState('');
  const [pendingPhotoDeletions, setPendingPhotoDeletions] = useState(0);
  const [days, setDays] = useState<Day[]>([]);
  const [next, setNext] = useState<string | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState('');
  const [focusedDay, setFocusedDay] = useState('');
  const drafts = useDrafts(userId);
  const currentDraft = drafts.get(date);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [logoutError, setLogoutError] = useState('');
  const busy = working || deleteOpen || discardOpen || !currentDraft.loaded;
  useEffect(() => { if (date) void drafts.load(date); }, [date]);
  useEffect(() => { if (currentDraft.value.deletion) setDeleteOpen(true); }, [date, currentDraft.value.deletion?.request_id]);
  const alive = useRef(true);
  const historyVersion = useRef(0);
  const [, tick] = useState(0);
  useEffect(() => {
    alive.current = true;
    const timer = setInterval(() => tick(n => n + 1), 60000);
    return () => { alive.current = false; clearInterval(timer); };
  }, []);
  const loadDays = useCallback(async (before?: string) => {
    const version = ++historyVersion.current;
    setHistoryLoading(true); setHistoryError('');
    try {
      const result = await api<DayPage>(`/v1/days${before ? `?before=${before}` : ''}`);
      if (alive.current && version === historyVersion.current) { setDays(previous => before ? [...previous, ...result.days] : result.days); setNext(result.next_before); }
    } catch (error) { if (alive.current && version === historyVersion.current) setHistoryError(errorMessage(error)); }
    finally { if (alive.current && version === historyVersion.current) setHistoryLoading(false); }
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setProfileError('');
    api<Profile>('/v1/profile', { signal: controller.signal }).then(result => {
      if (controller.signal.aborted) return;
      setProfile(result); setDate(previous => previous || todayIn(result.time_zone));
    }).catch(error => { if (!controller.signal.aborted) setProfileError(errorMessage(error)); });
    void loadDays();
    return () => controller.abort();
  }, [revision, loadDays]);
  useEffect(() => { setDateInput(date); setDateError(''); }, [date]);

  function openDate(value: string) {
    if (busy) return;
    const parsed = new Date(`${value}T12:00:00Z`);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || Number.isNaN(parsed.valueOf()) || parsed.getUTCFullYear() < 1900 || parsed.toISOString().slice(0, 10) !== value) {
      setDateError('Bitte ein gültiges Datum als JJJJ-MM-TT eingeben.'); return;
    }
    setDate(value); setDateInput(value); setDateError(''); setDatePickerOpen(false); setPane('chat'); setDeletedDate('');
  }
  async function logout() {
    setBusy(true); setLogoutError('');
    try {
      await drafts.flushAll();
      const { error } = await supabase!.auth.signOut({ scope: 'local' });
      if (error) setLogoutError('Abmelden fehlgeschlagen. Bitte erneut versuchen.');
    } catch { setLogoutError('Bitte zuerst den offenen Entwurf speichern oder den Entwurfskonflikt im Tageschat klären.'); }
    finally { if (alive.current) setBusy(false); }
  }
  function refreshDay() { setChatRevision(value => value + 1); void loadDays(); }
  async function deleted(result: DayDeletionResult) {
    await drafts.complete(date);
    setDeleteOpen(false); setDeletedDate(date); setPendingPhotoDeletions(result.pending_photo_deletions); refreshDay();
  }
  async function discardDraft() {
    setBusy(true);
    try {
      const photos = currentDraft.value.photos;
      await drafts.clear(date); setDiscardOpen(false);
      photos.forEach(photo => { void api(`/v1/attachments/${photo.id}`, { method: 'DELETE', userId }).catch(() => {}); });
    } catch { /* The draft stays visible, with its storage error. */ }
    finally { if (alive.current) setBusy(false); }
  }
  const hasDraft = !!(currentDraft.value.text || currentDraft.value.photos.length || currentDraft.value.pending);
  const draftNotice = <ScrollView style={draftStyles.notice} contentContainerStyle={s.field} keyboardShouldPersistTaps="handled">
    {!currentDraft.loaded ? <><Text style={s.small}>Entwurf wird geladen …</Text><ErrorNotice text={currentDraft.error} />
      {!!currentDraft.error && <Button secondary label="Entwurf erneut laden" onPress={() => void drafts.load(date)} />}</>
      : currentDraft.error ? <><ErrorNotice text={currentDraft.error} />
        {currentDraft.conflict ? <><Text style={s.small}>„Gespeicherten Entwurf übernehmen“ ersetzt den Text hier. „Meinen Entwurf speichern“ ersetzt den gespeicherten Entwurf.</Text>
          <View style={s.row}><Button secondary label="Gespeicherten Entwurf übernehmen" disabled={working} onPress={() => void drafts.reload(date)} />
            {!currentDraft.value.pending && !currentDraft.value.deletion && <Button secondary label="Meinen Entwurf speichern" disabled={working} onPress={() => void drafts.keepLocal(date)} />}</View></>
          : <Button secondary label="Entwurf erneut speichern" disabled={working} onPress={() => void drafts.flush(date).catch(() => {})} />}</>
      : currentDraft.task || currentDraft.committing ? <Text style={s.small}>Entwurf wird auf diesem Gerät gespeichert …</Text>
      : currentDraft.value.pending ? <Text style={s.small}>Dieser Sendevorgang ist noch offen. Mit „Erneut senden“ wird dieselbe Nachricht geprüft und gegebenenfalls gespeichert.</Text>
      : hasDraft && <Text style={s.small}>Entwurf auf diesem Gerät gespeichert.</Text>}
    {hasDraft && currentDraft.loaded && !currentDraft.value.pending && !currentDraft.value.deletion && !deleteOpen && <>
      {discardOpen ? <><Text style={s.small}>Text und Fotoauswahl für diesen Tag auf diesem Gerät verwerfen?</Text><View style={s.row}>
        <Button secondary label="Entwurf behalten" disabled={working} onPress={() => setDiscardOpen(false)} />
        <Button secondary label="Entwurf endgültig verwerfen" disabled={working || currentDraft.conflict} onPress={() => void discardDraft()} />
      </View></> : <Button variant="ghost" label="Entwurf verwerfen" disabled={working} onPress={() => setDiscardOpen(true)} />}
    </>}
  </ScrollView>;
  const today = profile ? todayIn(profile.time_zone) : '';
  const initials = (profile?.display_name || 'Fitty').split(/\s+/).slice(0, 2).map(word => word[0]).join('').toUpperCase();
  const history = <>
    <ErrorNotice text={drafts.listError} />
    {!!drafts.listError && <Button secondary label="Entwürfe erneut laden" onPress={() => void drafts.refreshList()} />}
    {!!drafts.infos.length && <View style={s.field}><Text style={s.eyebrow}>Entwürfe auf diesem Gerät</Text>
      {drafts.infos.map(info => <Button key={info.date} variant="ghost" label={`${dateLabel(info.date)}${info.hasPending ? ' · Vorgang offen' : ''}`}
        accessibilityLabel={`Entwurf vom ${dateLabel(info.date, true)}`} disabled={busy} onPress={() => openDate(info.date)} />)}
    </View>}
    <ErrorNotice text={historyError} />
    {!!historyError && <Button secondary label="Verlauf erneut laden" disabled={historyLoading} onPress={() => void loadDays()} />}
    {!days.length && !historyLoading && !historyError && <Text style={s.subtitle}>Deine Tage erscheinen hier, sobald du eine Nachricht speicherst.</Text>}
    {days.map(day => <Pressable key={day.date} accessibilityRole="button" accessibilityLabel={`${dateLabel(day.date, true)}, ${day.message_count} Nachrichten`} accessibilityState={{ selected: day.date === date, disabled: busy }} disabled={busy} onPress={() => openDate(day.date)}
      onFocus={() => setFocusedDay(day.date)} onBlur={() => setFocusedDay('')}
      style={({ pressed }) => [s.historyItem, day.date === date && s.historySelected, busy && s.disabled, pressed && s.pressed, focusedDay === day.date && s.focus]}>
      <View style={s.between}><Text style={s.historyDate}>{day.date === today ? 'Heute' : new Intl.DateTimeFormat('de-DE', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(`${day.date}T12:00:00Z`))}</Text>
        <Text style={s.historyCount}>{day.message_count} {day.message_count === 1 ? 'Nachricht' : 'Nachrichten'}</Text></View>
      <Text numberOfLines={2} style={s.small}>{day.preview}</Text>
    </Pressable>)}
    {historyLoading && <Text style={s.small}>Verlauf wird geladen …</Text>}
    {next && <Button secondary label="Ältere Tage laden" disabled={busy || historyLoading} onPress={() => void loadDays(next)} />}
  </>;

  return <View style={s.app}>
    {wide && <View style={s.sidebar}>
      <Brand /><View style={s.sidebarNav}>
        <Button icon="calendar" label="Heutiger Tag" secondary={pane !== 'chat' || date !== today} disabled={busy || !today} onPress={() => openDate(today)} />
        <Button variant="ghost" icon="user" label="Ziele & Vorlieben" selected={pane === 'profile'} disabled={busy} onPress={() => setPane('profile')} />
        <Button variant="ghost" icon="photo" label="Fortschrittsfotos" selected={pane === 'progress'} disabled={busy} onPress={() => setPane('progress')} />
      </View>
      <View style={s.historySection}><Text style={s.eyebrow}>Dein Verlauf</Text><ScrollView style={s.fill} contentContainerStyle={s.historyList}>{history}</ScrollView></View>
      <View style={s.sidebarBottom}>
        <Pressable accessibilityRole="button" accessibilityLabel="Mein Profil" disabled={busy} accessibilityState={{ disabled: busy, selected: pane === 'profile' }} onPress={() => setPane('profile')}
          onFocus={() => setFocusedControl('profile')} onBlur={() => setFocusedControl('')} style={({ pressed }) => [s.profileLink, pressed && s.pressed, focusedControl === 'profile' && s.focus]}>
          <View style={s.avatar}><Text style={s.avatarText}>{initials}</Text></View>
          <View style={s.flexible}><Text numberOfLines={1} style={s.label}>{profile?.display_name || 'Mein Profil'}</Text><Text numberOfLines={1} style={s.small}>{email}</Text></View>
        </Pressable>
        <Button variant="ghost" icon="logout" iconOnly label="Abmelden" disabled={busy} onPress={() => void logout()} />
      </View>
    </View>}
    <View style={s.main}>
      {!!logoutError && <View style={s.field}><ErrorNotice text={logoutError} /></View>}
      {!wide && <View style={s.mobileHeader}>
        <Pressable accessibilityRole="button" accessibilityLabel="Tageschat" disabled={busy} accessibilityState={{ disabled: busy, selected: pane === 'chat' }} onPress={() => setPane('chat')}
          onFocus={() => setFocusedControl('brand')} onBlur={() => setFocusedControl('')} style={[s.brandLink, focusedControl === 'brand' && s.focus]}><Brand /></Pressable>
        <View style={s.inline}>
        <Button variant="ghost" icon="history" selected={pane === 'history'} label="Verlauf" disabled={busy} onPress={() => setPane('history')} />
        <Button variant="ghost" icon="photo" iconOnly selected={pane === 'progress'} label="Fortschrittsfotos" disabled={busy} onPress={() => setPane('progress')} />
        <Button variant="ghost" icon="user" iconOnly selected={pane === 'profile'} label="Profil" disabled={busy} onPress={() => setPane('profile')} />
      </View></View>}
      {profileError ? <View style={s.pageScroll}><ErrorNotice text={profileError} /><Button label="Erneut laden" onPress={() => setRevision(n => n + 1)} />{!wide && <Button secondary label="Abmelden" onPress={() => void logout()} />}</View>
        : !profile || !date ? <View style={s.pageScroll}><Text style={s.subtitle}>Dein Tagebuch wird geladen …</Text></View>
        : pane === 'profile' ? <ProfileForm userId={userId} profile={profile} email={email} onSaved={setProfile} onLogout={logout} setBusy={setBusy} onBack={() => setPane('chat')} />
        : pane === 'progress' ? <ScrollView style={s.fill} contentContainerStyle={s.pageScroll} keyboardShouldPersistTaps="handled"><ProgressPhotos userId={userId} timeZone={profile.time_zone} onBusy={setBusy} /></ScrollView>
        : pane === 'history' ? <ScrollView contentContainerStyle={s.pageScroll}><Text role="heading" aria-level={1} style={s.title}>Dein Verlauf</Text>{history}</ScrollView>
        : <>
          <View style={[s.topBar, wide && s.topBarWide]}>
            <View style={s.headerRow}><View style={s.dayHeading}>{wide && <Text style={s.eyebrow}>Dein Tag</Text>}
              <Text role="heading" aria-level={1} accessibilityLabel={dateLabel(date, true)} style={[s.heading, wide && s.headingWide]}>{width < 360
                ? new Intl.DateTimeFormat('de-DE', { weekday: 'short', day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }).format(new Date(`${date}T12:00:00Z`))
                : dateLabel(date, true)}</Text></View>
              <View style={[s.row, deletionStyles.dayActions]}>
                <Button secondary icon="left" iconOnly label="Vorheriger Tag" disabled={busy || date === '1900-01-01'} onPress={() => openDate(moveDate(date, -1))} />
                <Button secondary label="Heute" selected={date === today} disabled={busy} onPress={() => openDate(today)} />
                <Button secondary icon="right" iconOnly label="Nächster Tag" disabled={busy || date === '9999-12-31'} onPress={() => openDate(moveDate(date, 1))} />
                <Button variant="ghost" icon="calendar" iconOnly label="Datum wählen" expanded={datePickerOpen} disabled={busy} onPress={() => setDatePickerOpen(value => !value)} />
                <Button variant="ghost" label="Tageschat löschen" disabled={busy} onPress={() => { setDatePickerOpen(false); setDeleteOpen(true); }} />
              </View>
            </View>
            {datePickerOpen && <View style={s.field}><Text style={s.label}>Zu einem Datum springen</Text><View style={s.row}>
              <TextInput accessibilityLabel="Datum (JJJJ-MM-TT)" value={dateInput} onChangeText={setDateInput} editable={!busy} autoFocus maxLength={10} placeholder="JJJJ-MM-TT"
                onFocus={() => setFocusedControl('date')} onBlur={() => setFocusedControl('')} style={[s.dateInput, focusedControl === 'date' && s.focus]} onSubmitEditing={() => openDate(dateInput)} />
              <Button label="Öffnen" disabled={busy} onPress={() => openDate(dateInput)} />
              <Button variant="ghost" label="Schließen" disabled={busy} onPress={() => { setDatePickerOpen(false); setDateError(''); }} />
            </View><ErrorNotice text={dateError} /></View>}
            {deletedDate === date && <Text accessibilityLiveRegion="polite" style={s.success}>Tageschat gelöscht.</Text>}
          </View>
          <Chat key={`${date}:${chatRevision}`} userId={userId} date={date} timeZone={profile.time_zone} draft={currentDraft.value.text}
            externalBusy={deleteOpen || discardOpen || !currentDraft.loaded || !!currentDraft.value.deletion}
            sendBlocked={currentDraft.conflict || !!currentDraft.error} draftNotice={draftNotice}
            initialPendingPhotoDeletions={deletedDate === date ? pendingPhotoDeletions : 0}
            setDraft={value => drafts.setText(date, value)} photos={currentDraft.value.photos} setPhotos={value => drafts.setPhotos(date, value)}
            pending={currentDraft.value.pending} prepareSend={pending => drafts.prepareSend(date, pending)}
            completeSend={() => drafts.complete(date)} rejectSend={() => drafts.releaseSend(date)}
            onTargets={() => setPane('profile')} setBusy={setBusy} onSaved={() => { setDeletedDate(''); void loadDays(); }} />
          {deleteOpen && <DayDelete key={date} userId={userId} date={date} hasDraft={!!currentDraft.value.text} hasPhotos={!!currentDraft.value.photos.length}
            initialRequest={currentDraft.value.deletion} prepareRequest={request => drafts.prepareDeletion(date, request)}
            releaseRequest={() => drafts.releaseDeletion(date)} onClose={() => setDeleteOpen(false)} onDeleted={deleted} onRefresh={refreshDay} />}

        </>}
    </View>
  </View>;
}

function ProfileForm({ userId, profile, email, onSaved, onLogout, setBusy, onBack }: {
  userId: string; profile: Profile; email: string; onSaved: (profile: Profile) => void; onLogout: () => Promise<void>; setBusy: (value: boolean) => void; onBack: () => void;
}) {
  const [value, setValue] = useState(profile);
  const [saving, setSaving] = useState(false);
  const [targetsBusy, setTargetsBusy] = useState(false);
  const busy = saving || targetsBusy;
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  function field(key: keyof Profile, content: string) { setValue(previous => ({ ...previous, [key]: content })); setSaved(false); }
  async function save() {
    if (busy) return;
    try { new Intl.DateTimeFormat('de-DE', { timeZone: value.time_zone }).format(new Date()); }
    catch { setError('Bitte eine gültige Zeitzone verwenden, zum Beispiel Europe/Berlin.'); return; }
    setSaving(true); setBusy(true); setError(''); setSaved(false);
    try { const result = await api<Profile>('/v1/profile', { method: 'PUT', body: value, userId }); setValue(result); onSaved(result); setSaved(true); }
    catch (error) { setError(errorMessage(error)); }
    finally { setSaving(false); setBusy(false); }
  }
  return <ScrollView contentContainerStyle={s.pageScroll} keyboardShouldPersistTaps="handled">
    <Text style={s.eyebrow}>Ganz persönlich</Text><Text role="heading" aria-level={1} style={s.title}>Dein Profil</Text>
    <Text style={s.subtitle}>Deine Ziele und Vorlieben helfen Fitty, Antworten und Essensvorschläge auf deinen Alltag abzustimmen.</Text>
    <Text style={s.small}>{email}</Text>
    <Field label="Dein Name" value={value.display_name} onChangeText={text => field('display_name', text)} maxLength={80} editable={!busy} autoComplete="name" />
    <Field label="Zeitzone" value={value.time_zone} onChangeText={text => field('time_zone', text)} autoCapitalize="none" editable={!busy} placeholder="Europe/Berlin" />
    <Text style={s.small}>Bestimmt, welcher Tag als „Heute“ geöffnet wird. Bereits gespeicherte Tage bleiben unverändert.</Text>
    <Field label="Deine Ziele" value={value.goals} onChangeText={text => field('goals', text)} maxLength={2000} multiline editable={!busy} placeholder="Was möchtest du erreichen?" />
    <Field label="Ernährung und Vorlieben" value={value.preferences} onChangeText={text => field('preferences', text)} maxLength={2000} multiline editable={!busy} placeholder="Zum Beispiel vegetarisch, Unverträglichkeiten oder Lieblingsessen" />
    <ErrorNotice text={error} />{saved && <Text accessibilityLiveRegion="polite" style={s.success}>Profil gespeichert.</Text>}
    <Button label={saving ? 'Speichern …' : 'Profil speichern'} disabled={busy} onPress={() => void save()} />
    <TargetsForm userId={userId} timeZone={profile.time_zone} disabled={saving} onBusy={value => { setTargetsBusy(value); setBusy(value); }} />
    <Button secondary label="Zurück zum Tageschat" disabled={busy} onPress={onBack} />
    <Button secondary label="Abmelden" disabled={busy} onPress={() => void onLogout()} />
  </ScrollView>;
}
