import { useEffect, useState } from 'react';
import { Image, Modal, Pressable, Text, View } from 'react-native';
import { api, Attachment, errorMessage } from '../lib/chat-api';
import { styles as s } from '../styles/app';
import { Button } from './ui';

export function ChatPhoto({ attachment, index }: { attachment: Attachment; index: number }) {
  const [uri, setURI] = useState('');
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const [open, setOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    async function load() {
      try {
        const result = await api<{ path: string }>(`/v1/attachments/${attachment.id}/url`, { signal: controller.signal });
        if (controller.signal.aborted) return;
        setURI(`${process.env.EXPO_PUBLIC_SUPABASE_URL}${result.path}`); setError('');
        timer = setTimeout(() => void load(), 9 * 60 * 1000);
      } catch (error) { if (!controller.signal.aborted) setError(errorMessage(error)); }
    }
    void load();
    return () => { controller.abort(); clearTimeout(timer); };
  }, [attachment.id, revision]);
  return <View style={s.photoItem}>
    {uri && !error ? <Pressable accessibilityRole="button" accessibilityLabel={`Foto ${index + 1} vergrößern`} onPress={() => setOpen(true)}
      onFocus={() => setFocused(true)} onBlur={() => setFocused(false)} style={focused && s.focus}>
      <Image source={{ uri }} style={s.photoThumbnail} accessibilityLabel={`Angehängtes Foto ${index + 1}`} resizeMode="cover" onError={() => setError('Bild konnte nicht geladen werden.')} />
    </Pressable> : <Text style={s.small}>{error || 'Bild wird geladen …'}</Text>}
    {!!error && <Button secondary label="Bild erneut laden" onPress={() => setRevision(value => value + 1)} />}
    <Modal visible={open} transparent animationType="none" onRequestClose={() => setOpen(false)}>
      <View style={s.photoModal} accessibilityViewIsModal>
        <Button label="Bild schließen" onPress={() => setOpen(false)} />
        {!!uri && <Image source={{ uri }} style={s.photoFull} resizeMode="contain" accessibilityLabel={`Angehängtes Foto ${index + 1}, vergrößert`} />}
      </View>
    </Modal>
  </View>;
}
