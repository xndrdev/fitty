import { Platform } from 'react-native';
import { randomUUID } from 'expo-crypto';
import * as ImagePicker from 'expo-image-picker';
import { ImageManipulator, SaveFormat } from 'expo-image-manipulator';
import { File } from 'expo-file-system';
import { api, Attachment } from './chat-api';
import { releasePhotos } from './draft-store';

export type DraftPhoto = { id: string; uri: string; width: number; height: number };
export async function pickPhotos(camera: boolean, remaining: number): Promise<DraftPhoto[]> {
  if (camera && Platform.OS !== 'web') {
    const permission = await ImagePicker.requestCameraPermissionsAsync();
    if (!permission.granted) throw new Error('Bitte erlaube Fitty den Kamerazugriff in den Einstellungen oder wähle ein vorhandenes Foto.');
  }
  const options: ImagePicker.ImagePickerOptions = {
    mediaTypes: ['images'], quality: 1, exif: false,
    preferredAssetRepresentationMode: ImagePicker.UIImagePickerPreferredAssetRepresentationMode.Compatible,
  };
  const result = camera ? await ImagePicker.launchCameraAsync(options)
    : await ImagePicker.launchImageLibraryAsync({ ...options, allowsMultipleSelection: remaining > 1, selectionLimit: remaining, orderedSelection: remaining > 1 });
  if (result.canceled) return [];
  if (result.assets.length > remaining) throw new Error(`Bitte höchstens ${remaining === 1 ? 'ein Bild' : `${remaining} Bilder`} auswählen.`);
  return preparePhotos(result.assets);
}
async function preparePhotos(assets: Pick<ImagePicker.ImagePickerAsset, 'uri' | 'width' | 'height' | 'fileSize'>[]): Promise<DraftPhoto[]> {
  const photos: DraftPhoto[] = [];
  try {
    for (const asset of assets) {
      if ((asset.fileSize ?? 0) > 20 * 1024 * 1024) throw new Error('Bitte Bilder mit höchstens 20 MiB auswählen.');
      const context = ImageManipulator.manipulate(asset.uri);
      let rendered;
      try {
        if (asset.width > 2048 || asset.height > 2048) context.resize(asset.width >= asset.height ? { width: 2048 } : { height: 2048 });
        rendered = await context.renderAsync();
        const image = await rendered.saveAsync({ format: SaveFormat.JPEG, compress: 0.9 });
        photos.push({ id: randomUUID(), uri: image.uri, width: image.width, height: image.height });
      } finally { rendered?.release(); context.release(); }
    }
    return photos;
  } catch (error) {
    photos.forEach(discardPhoto);
    if (error instanceof Error && error.message.startsWith('Bitte Bilder')) throw error;
    throw new Error('Das Bild konnte nicht geöffnet werden. Bitte ein JPEG, PNG oder WebP wählen; auf dem iPhone kannst du auch ein neues Foto aufnehmen.');
  }
}
export async function pastedPhotos(files: globalThis.File[]): Promise<DraftPhoto[]> {
  const urls: string[] = [];
  try {
    const assets = [];
    for (const file of files) {
      if (file.size > 20 * 1024 * 1024) throw new Error('Bitte Bilder mit höchstens 20 MiB einfügen.');
      const uri = URL.createObjectURL(file); urls.push(uri);
      const image = new globalThis.Image(); image.src = uri;
      await image.decode();
      assets.push({ uri, width: image.naturalWidth, height: image.naturalHeight, fileSize: file.size });
    }
    return await preparePhotos(assets);
  } finally { urls.forEach(uri => URL.revokeObjectURL(uri)); }
}
export async function uploadPhoto(date: string, photo: DraftPhoto, userId?: string) {
  const data = await readPhotoBytes(photo);
  return api<{ attachment: Attachment }>(`/v1/days/${date}/attachments/${photo.id}`, { method: 'PUT', rawBody: data, timeout: 70000, userId });
}
export async function readPhotoBytes(photo: DraftPhoto): Promise<ArrayBuffer> {
  const data = Platform.OS === 'web' ? await (await fetch(photo.uri)).arrayBuffer() : await new File(photo.uri).arrayBuffer();
  if (data.byteLength > 8 * 1024 * 1024) throw new Error('Das aufbereitete Bild ist noch zu groß. Bitte einen kleineren Bildausschnitt wählen.');
  return data;
}
export function discardPhoto(photo: DraftPhoto) { releasePhotos([photo]); }
