'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { OctagonX, ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  BOT_STATUS_CHANGED,
  type KillSwitch,
  SCOPE_LABELS,
  fetchBotStatus,
  killActive,
  setKillSwitch,
} from '@/lib/bot';

const REFRESH_MS = 15000;
/** The second press must come within this window, or the button disarms. */
const ARM_WINDOW_MS = 5000;

/**
 * Global kill switch in the header. Activating takes two presses: the first
 * arms the button, the second (within a few seconds) fires. This blocks new
 * entries on every broker; exits and stop-losses keep working.
 */
export function KillSwitchButton() {
  const [switches, setSwitches] = useState<KillSwitch[] | null>(null);
  const [open, setOpen] = useState(false);
  const [armed, setArmed] = useState(false);
  const [closePositions, setClosePositions] = useState(false);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const disarmTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const refresh = useCallback(async () => {
    try {
      setSwitches((await fetchBotStatus()).kill_switches);
    } catch {
      setSwitches(null); // API unreachable: hide the control rather than show a wrong state
    }
  }, []);

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, REFRESH_MS);
    return () => clearInterval(timer);
  }, [refresh]);

  useEffect(
    () => () => {
      if (disarmTimer.current) {
        clearTimeout(disarmTimer.current);
      }
    },
    []
  );

  if (!switches) {
    return null;
  }

  const active = killActive(switches);
  const activeScopes = switches
    .filter(k => k.active)
    .map(k => SCOPE_LABELS[k.scope] ?? k.scope)
    .join('・');

  const disarm = () => {
    setArmed(false);
    if (disarmTimer.current) {
      clearTimeout(disarmTimer.current);
    }
  };

  const apply = async (next: boolean) => {
    setBusy(true);
    setError(null);
    try {
      if (next) {
        setSwitches(
          await setKillSwitch('global', true, closePositions, reason)
        );
      } else {
        // Release every active scope so "解除" really resumes trading.
        let latest = switches;
        for (const k of switches.filter(s => s.active)) {
          latest = await setKillSwitch(k.scope, false, false, '');
        }
        setSwitches(latest);
      }
      setOpen(false);
      setReason('');
      setClosePositions(false);
      window.dispatchEvent(new Event(BOT_STATUS_CHANGED));
    } catch (err) {
      setError(err instanceof Error ? err.message : '更新できませんでした');
    } finally {
      setBusy(false);
      disarm();
    }
  };

  const onActivatePress = () => {
    if (!armed) {
      setArmed(true);
      disarmTimer.current = setTimeout(() => setArmed(false), ARM_WINDOW_MS);
      return;
    }
    apply(true);
  };

  return (
    <div className="relative">
      <Button
        variant={active ? 'destructive' : 'outline'}
        size="sm"
        onClick={() => {
          setOpen(o => !o);
          disarm();
        }}
        aria-expanded={open}
      >
        {active ? (
          <OctagonX className="h-4 w-4 mr-1" />
        ) : (
          <ShieldCheck className="h-4 w-4 mr-1" />
        )}
        {active ? `停止中（${activeScopes}）` : 'Kill Switch'}
      </Button>

      {open && (
        <div className="absolute right-0 mt-2 w-80 rounded-md border bg-background p-4 shadow-lg z-50 space-y-3 text-sm">
          {active ? (
            <>
              <p className="font-medium text-red-600">
                新規の発注を停止しています
              </p>
              <ul className="text-muted-foreground space-y-1">
                {switches
                  .filter(k => k.active)
                  .map(k => (
                    <li key={k.scope}>
                      {SCOPE_LABELS[k.scope] ?? k.scope}
                      {k.close_positions && '（全決済）'}
                      {k.reason && ` — ${k.reason}`}
                    </li>
                  ))}
              </ul>
              <p className="text-muted-foreground">
                決済と損切りは引き続き動作します。
              </p>
              <Button
                size="sm"
                variant="outline"
                className="w-full"
                disabled={busy}
                onClick={() => apply(false)}
              >
                解除して取引を再開
              </Button>
            </>
          ) : (
            <>
              <p className="font-medium">全ブローカーの新規発注を止める</p>
              <p className="text-muted-foreground">
                決済と損切りは止まりません。解除するまで新しい建玉は作られません。
              </p>
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={closePositions}
                  onChange={e => setClosePositions(e.target.checked)}
                />
                保有中の建玉も成行で決済する
              </label>
              <input
                className="w-full rounded-md border px-2 py-1"
                placeholder="理由（任意）"
                value={reason}
                maxLength={200}
                onChange={e => setReason(e.target.value)}
              />
              <Button
                size="sm"
                variant="destructive"
                className="w-full"
                disabled={busy}
                onClick={onActivatePress}
              >
                {armed ? 'もう一度押すと発動します' : '停止する'}
              </Button>
            </>
          )}
          {error && <p className="text-red-600">{error}</p>}
        </div>
      )}
    </div>
  );
}
