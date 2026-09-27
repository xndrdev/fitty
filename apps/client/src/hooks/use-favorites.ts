import { useEffect, useRef, useState } from 'react';
import { api, errorMessage, type Entry } from '../lib/chat-api';
import { favoriteForEntry, type Favorite } from '../lib/favorites';

export function useFavorites(userId: string) {
  const [favorites, setFavorites] = useState<Favorite[]>([]);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const [revision, setRevision] = useState(0);
  const generation = useRef(0);
  const locked = useRef(false);
  useEffect(() => {
    const current = ++generation.current;
    const controller = new AbortController();
    setReady(false); setError('');
    api<{ favorites: Favorite[] }>('/v1/favorites', { userId, signal: controller.signal }).then(result => {
      if (generation.current === current) { setFavorites(result.favorites); setReady(true); }
    }).catch(error => { if (generation.current === current) setError(errorMessage(error)); });
    return () => { ++generation.current; controller.abort(); };
  }, [userId, revision]);

  async function change(id: string, entry?: Entry) {
    if (locked.current || !ready) return;
    locked.current = true; setBusy(id); setError('');
    const current = generation.current;
    try {
      if (entry) {
        const result = await api<{ favorite: Favorite }>(`/v1/favorites/${id}`, { method: 'PUT', body: { entry_version: entry.version }, userId });
        if (generation.current === current) setFavorites(previous => [result.favorite, ...previous.filter(item => item.id !== id)]);
      } else {
        await api(`/v1/favorites/${id}`, { method: 'DELETE', userId });
        if (generation.current === current) setFavorites(previous => previous.filter(item => item.id !== id));
      }
    } catch (error) {
      // A lost response can still mean the bookmark was saved. Reload before
      // allowing another toggle rather than guessing the resulting state.
      if (generation.current === current) { setError(errorMessage(error)); setReady(false); }
    } finally {
      locked.current = false;
      if (generation.current === current) setBusy('');
    }
  }
  return {
    favorites, ready, error, busy,
    reload: () => { if (!locked.current) setRevision(value => value + 1); },
    remove: (favorite: Favorite) => change(favorite.id),
    toggle: (entry: Entry) => {
      const favorite = favoriteForEntry(favorites, entry);
      return favorite ? change(favorite.id) : change(entry.id, entry);
    },
  };
}
