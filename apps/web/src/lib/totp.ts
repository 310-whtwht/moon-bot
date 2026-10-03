// RFC 6238 TOTP (SHA-1, 6 digits, 30 s) compatible with Google / Microsoft
// Authenticator. Uses Web Crypto so it works in both Node and Edge runtimes.

const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

export function base32Decode(input: string): Uint8Array<ArrayBuffer> {
  const clean = input.toUpperCase().replace(/[\s=-]/g, '');
  let bits = 0;
  let value = 0;
  const out: number[] = [];
  for (const char of clean) {
    const idx = BASE32_ALPHABET.indexOf(char);
    if (idx === -1) {
      throw new Error('Invalid base32 character in TOTP secret');
    }
    value = (value << 5) | idx;
    bits += 5;
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return new Uint8Array(out);
}

async function hotp(
  key: Uint8Array<ArrayBuffer>,
  counter: number
): Promise<string> {
  const msg = new ArrayBuffer(8);
  const view = new DataView(msg);
  view.setUint32(0, Math.floor(counter / 2 ** 32));
  view.setUint32(4, counter >>> 0);

  const cryptoKey = await crypto.subtle.importKey(
    'raw',
    key,
    { name: 'HMAC', hash: 'SHA-1' },
    false,
    ['sign']
  );
  const mac = new DataView(await crypto.subtle.sign('HMAC', cryptoKey, msg));
  // Dynamic truncation (RFC 4226 §5.3)
  const offset = mac.getUint8(mac.byteLength - 1) & 0x0f;
  const code = (mac.getUint32(offset) & 0x7fffffff) % 1_000_000;
  return code.toString().padStart(6, '0');
}

/**
 * Verifies a 6-digit code, allowing `window` steps of clock drift either way.
 */
export async function verifyTotp(
  secretBase32: string,
  code: string,
  now: number = Date.now(),
  window = 1
): Promise<boolean> {
  if (!/^\d{6}$/.test(code)) {
    return false;
  }
  const key = base32Decode(secretBase32);
  const counter = Math.floor(now / 1000 / 30);
  for (let i = -window; i <= window; i++) {
    if ((await hotp(key, counter + i)) === code) {
      return true;
    }
  }
  return false;
}
