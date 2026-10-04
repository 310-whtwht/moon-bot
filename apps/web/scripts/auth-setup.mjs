// Generates the admin login settings for Vercel / .env:
//   npm run auth:setup
// Prints ADMIN_EMAIL, ADMIN_PASSWORD_HASH, ADMIN_TOTP_SECRET and an otpauth://
// URI to register in Google / Microsoft Authenticator. Nothing is stored.
import { randomBytes } from 'node:crypto';
import { createInterface } from 'node:readline';
import { stdin, stdout } from 'node:process';
import bcrypt from 'bcryptjs';

const BASE32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

function base32Encode(bytes) {
  let bits = 0;
  let value = 0;
  let out = '';
  for (const byte of bytes) {
    value = (value << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      out += BASE32_ALPHABET[(value >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits > 0) {
    out += BASE32_ALPHABET[(value << (5 - bits)) & 31];
  }
  return out;
}

// Read line by line so it works both interactively and with piped input.
const rl = createInterface({ input: stdin });
const lines = rl[Symbol.asyncIterator]();
async function ask(prompt) {
  stdout.write(prompt);
  const { value } = await lines.next();
  return value ?? '';
}
const email = (await ask('管理者のメールアドレス: ')).trim();
const password = await ask('パスワード（12文字以上推奨）: ');
rl.close();

if (!email || password.length < 8) {
  console.error('メールアドレスと8文字以上のパスワードを入力してください。');
  process.exit(1);
}

const hash = await bcrypt.hash(password, 12);
const totpSecret = base32Encode(randomBytes(20));
const label = encodeURIComponent(`moon-bot:${email}`);
const uri = `otpauth://totp/${label}?secret=${totpSecret}&issuer=moon-bot&algorithm=SHA1&digits=6&period=30`;

console.log(`
以下を apps/web/.env.prod に貼り付け、npm run env:push で Vercel（Production）に反映してください。
（AUTH_SECRET は空のままにすると env:push が自動で生成します）

ADMIN_EMAIL=${email}
ADMIN_PASSWORD_HASH='${hash}'
ADMIN_TOTP_SECRET=${totpSecret}

認証アプリには次の URI（またはシークレット）を登録してください:
${uri}
`);
