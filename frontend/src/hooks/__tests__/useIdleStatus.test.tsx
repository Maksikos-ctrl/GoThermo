import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, fireEvent } from '@testing-library/react';
import { useIdleStatus, IDLE_AWAY_MS } from '../useIdleStatus';
import type { StatusType } from '../../types';

const minutes = (n: number) => n * 60 * 1000;

describe('useIdleStatus', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  function setup(status: StatusType = 'online', inCall = false) {
    const onGoAway = vi.fn();
    const onReturn = vi.fn();
    const hook = renderHook(
      (props: { status: StatusType; inCall: boolean }) =>
        useIdleStatus({ enabled: true, currentStatus: props.status, inCall: props.inCall, onGoAway, onReturn }),
      { initialProps: { status, inCall } },
    );
    return { onGoAway, onReturn, ...hook };
  }

  it('waits ten minutes, not seconds, before going away', () => {
    expect(IDLE_AWAY_MS).toBe(minutes(10));
    const { onGoAway } = setup();
    act(() => { vi.advanceTimersByTime(minutes(2)); });
    expect(onGoAway).not.toHaveBeenCalled();
    act(() => { vi.advanceTimersByTime(minutes(9)); });
    expect(onGoAway).toHaveBeenCalledTimes(1);
  });

  it('activity keeps the person online', () => {
    const { onGoAway } = setup();
    for (let i = 0; i < 4; i++) {
      act(() => { vi.advanceTimersByTime(minutes(8)); });
      act(() => { fireEvent.mouseMove(window); });
    }
    expect(onGoAway).not.toHaveBeenCalled();
  });

  it('a call never counts as idle', () => {
    const { onGoAway } = setup('online', true);
    act(() => { vi.advanceTimersByTime(minutes(60)); });
    expect(onGoAway).not.toHaveBeenCalled();
  });

  it('only "online" turns into away (someone already away or offline is left alone)', () => {
    const { onGoAway } = setup('away');
    act(() => { vi.advanceTimersByTime(minutes(60)); });
    expect(onGoAway).not.toHaveBeenCalled();
  });

  it('comes back when the person returns', () => {
    const { onGoAway, onReturn, rerender } = setup();
    act(() => { vi.advanceTimersByTime(minutes(11)); });
    expect(onGoAway).toHaveBeenCalledTimes(1);
    rerender({ status: 'away', inCall: false });
    act(() => { fireEvent.keyDown(window); });
    expect(onReturn).toHaveBeenCalledTimes(1);
  });
});
