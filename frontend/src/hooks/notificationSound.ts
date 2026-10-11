let audioCtx: AudioContext | null = null;

function getAudioCtx(): AudioContext {
  if (!audioCtx) {
    const Ctx = window.AudioContext || (window as any).webkitAudioContext;
    audioCtx = new Ctx();
  }
  return audioCtx;
}

// A short two-note "ding" - pleasant, quick, doesn't loop or need an
// audio file. Volume is 0-1, same scale as the call ringtone's slider.
export function playNotificationSound(volume: number = 0.5) {
  if (volume <= 0) return;
  try {
    const ctx = getAudioCtx();
    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
    const now = ctx.currentTime;
    const gainValue = 0.2 * volume;

    const playNote = (freq: number, start: number, duration: number) => {
      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0, now + start);
      gain.gain.linearRampToValueAtTime(gainValue, now + start + 0.02);
      gain.gain.linearRampToValueAtTime(0, now + start + duration);
      gain.connect(ctx.destination);

      const osc = ctx.createOscillator();
      osc.type = 'sine';
      osc.frequency.value = freq;
      osc.connect(gain);
      osc.start(now + start);
      osc.stop(now + start + duration);
    };

    playNote(880, 0, 0.12);
    playNote(1175, 0.09, 0.15);
  } catch {
    // Audio can fail to init in some contexts - a missed notification
    // sound isn't worth surfacing an error for.
  }
}

// Plays a sample of the incoming-call ring (same double-beep and same
// volume formula as the real ringtone in useCall) so the settings slider
// gives immediate audible feedback instead of staying silent until an
// actual call happens.
export function playRingtonePreview(volume: number) {
  const gainValue = 0.3 * volume;
  if (gainValue <= 0) return;
  try {
    const ctx = getAudioCtx();
    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
    const now = ctx.currentTime;

    const playBeep = (start: number) => {
      const gain = ctx.createGain();
      gain.gain.setValueAtTime(gainValue, now + start);
      gain.gain.setValueAtTime(gainValue, now + start + 0.22);
      gain.gain.linearRampToValueAtTime(0, now + start + 0.25);
      gain.connect(ctx.destination);

      [800, 1000].forEach((freq) => {
        const osc = ctx.createOscillator();
        osc.type = 'sine';
        osc.frequency.value = freq;
        osc.connect(gain);
        osc.start(now + start);
        osc.stop(now + start + 0.25);
      });
    };

    playBeep(0);
    playBeep(0.4);
  } catch {
    // Same reasoning as the message sound: a failed preview isn't worth an error.
  }
}