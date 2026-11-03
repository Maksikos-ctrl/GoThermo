// Google sign-in inside the browser: Google's own script opens its consent
// popup and hands back an access token; the server then verifies the token
// with Google and signs the person in (LoginWithGoogleToken).

const GSI_URL = 'https://accounts.google.com/gsi/client';

interface TokenResponse {
  access_token?: string;
  error?: string;
}

interface TokenClient {
  requestAccessToken: () => void;
}

interface GoogleGlobal {
  accounts: {
    oauth2: {
      initTokenClient: (cfg: {
        client_id: string;
        scope: string;
        callback: (r: TokenResponse) => void;
        error_callback?: (e: { type?: string }) => void;
      }) => TokenClient;
    };
  };
}

let loading: Promise<void> | null = null;

function loadScript(): Promise<void> {
  if ((window as unknown as { google?: GoogleGlobal }).google?.accounts) return Promise.resolve();
  if (!loading) {
    loading = new Promise<void>((resolve, reject) => {
      const s = document.createElement('script');
      s.src = GSI_URL;
      s.async = true;
      s.onload = () => resolve();
      s.onerror = () => {
        loading = null;
        s.remove();
        reject('Could not load Google sign-in. Check your connection or ad blocker.');
      };
      document.head.appendChild(s);
    });
  }
  return loading;
}

/** Opens Google's popup and resolves with an access token. */
export async function requestGoogleAccessToken(clientId: string): Promise<string> {
  await loadScript();
  const google = (window as unknown as { google: GoogleGlobal }).google;
  return new Promise<string>((resolve, reject) => {
    const client = google.accounts.oauth2.initTokenClient({
      client_id: clientId,
      scope: 'openid email profile',
      callback: (r) => {
        if (r.access_token) resolve(r.access_token);
        else reject(r.error === 'access_denied' ? 'Google sign-in was cancelled.' : 'Google sign-in failed.');
      },
      error_callback: (e) => {
        reject(e?.type === 'popup_closed' ? 'Google sign-in was cancelled.' : 'Google sign-in failed. Allow pop-ups for this site and try again.');
      },
    });
    client.requestAccessToken();
  });
}
