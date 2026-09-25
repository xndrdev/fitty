const apiURL = process.env.EXPO_PUBLIC_API_URL ?? 'http://127.0.0.1:8787';

export async function checkConnection(signal: AbortSignal): Promise<void> {
  const response = await fetch(`${apiURL.replace(/\/+$/, '')}/healthz`, { signal });

  if (!response.ok) {
    throw new Error('Server nicht erreichbar.');
  }

  const data: unknown = await response.json();
  if (
    typeof data !== 'object' ||
    data === null ||
    !('status' in data) ||
    data.status !== 'ok' ||
    !('service' in data) ||
    data.service !== 'fitty-api'
  ) {
    throw new Error('Unerwartete Serverantwort.');
  }
}
