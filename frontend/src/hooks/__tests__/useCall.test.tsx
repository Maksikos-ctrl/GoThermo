import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useCall, mediaErrorMessage } from '../useCall';

class FakePC {
  connectionState = 'new';
  remoteDescription: unknown = null;
  onconnectionstatechange: (() => void) | null = null;
  addTrack() {}
  close() {}
  async setRemoteDescription(d: unknown) { this.remoteDescription = d; }
  async createAnswer() { return { type: 'answer', sdp: 'a' }; }
  async createOffer() { return { type: 'offer', sdp: 'o' }; }
  async setLocalDescription() {}
  async addIceCandidate() {}
}

let lastPC: FakePC | null = null;

class FakeAudioContext {
  state = 'running';
  currentTime = 0;
  destination = {};
  resume() { return Promise.resolve(); }
  createGain() { return { gain: { setValueAtTime() {}, linearRampToValueAtTime() {} }, connect() {} }; }
  createOscillator() { return { type: '', frequency: { value: 0 }, connect() {}, start() {}, stop() {} }; }
}

function install(getUserMedia: () => Promise<unknown>) {
  (window as any).AudioContext = FakeAudioContext;
  (globalThis as any).RTCPeerConnection = class extends FakePC { constructor() { super(); lastPC = this; } };
  (globalThis as any).RTCSessionDescription = function (d: unknown) { return d; };
  (globalThis as any).RTCIceCandidate = function (c: unknown) { return c; };
  Object.defineProperty(navigator, 'mediaDevices', { value: { getUserMedia }, configurable: true });
}

const stream = { getTracks: () => [{ stop() {} }], getAudioTracks: () => [] };

describe('useCall on the receiving side', () => {
  beforeEach(() => { lastPC = null; });
  afterEach(() => { delete (globalThis as any).RTCPeerConnection; });

  function setup() {
    const sendSignal = vi.fn();
    const onError = vi.fn();
    const hook = renderHook(() => useCall({ currentUser: 'bob', sendSignal, onError }));
    act(() => {
      hook.result.current.handleSignal('call_offer', { to: 'bob', from: 'ann', sdp: { type: 'offer', sdp: 'x' }, wantsVideo: true });
    });
    return { sendSignal, onError, ...hook };
  }

  it('shows "connecting" as soon as the call is accepted', async () => {
    install(async () => stream);
    const { result, sendSignal } = setup();
    expect(result.current.status).toBe('ringing');

    await act(async () => { await result.current.acceptCall(); });

    expect(result.current.status).toBe('connecting');
    expect(result.current.incomingCall).toBeNull();
    expect(sendSignal).toHaveBeenCalledWith('call_answer', expect.objectContaining({ to: 'ann' }));
  });

  it('becomes connected once the network path is up', async () => {
    install(async () => stream);
    const { result } = setup();
    await act(async () => { await result.current.acceptCall(); });
    act(() => { lastPC!.connectionState = 'connected'; lastPC!.onconnectionstatechange?.(); });
    expect(result.current.status).toBe('connected');
  });

  it('tells the person when the connection fails instead of just vanishing', async () => {
    install(async () => stream);
    const { result, onError } = setup();
    await act(async () => { await result.current.acceptCall(); });
    act(() => { lastPC!.connectionState = 'failed'; lastPC!.onconnectionstatechange?.(); });
    expect(onError).toHaveBeenCalledWith(expect.stringContaining('could not connect'));
    expect(result.current.status).toBe('idle');
  });

  it('explains a blocked microphone and declines the call', async () => {
    install(async () => { throw Object.assign(new Error('x'), { name: 'NotAllowedError' }); });
    const { result, onError, sendSignal } = setup();
    await act(async () => { await result.current.acceptCall(); });
    expect(onError).toHaveBeenCalledWith(expect.stringContaining('Microphone is blocked'));
    expect(sendSignal).toHaveBeenCalledWith('call_reject', expect.objectContaining({ to: 'ann' }));
    expect(result.current.status).toBe('idle');
  });
});

describe('mediaErrorMessage', () => {
  it('maps the usual failures', () => {
    expect(mediaErrorMessage({ name: 'NotFoundError' })).toBe('No microphone found.');
    expect(mediaErrorMessage({ name: 'NotReadableError' })).toMatch(/busy/);
    expect(mediaErrorMessage({ name: 'Weird' })).toBe('Could not start the call.');
  });
});
