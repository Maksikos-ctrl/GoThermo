import { describe, it, expect, beforeEach } from 'vitest';
import { titleFor, addUnseen, clearUnseen } from './notify';

describe('title badge', () => {
  beforeEach(() => clearUnseen());
  it('formats counts', () => {
    expect(titleFor(0)).toBe('GoThermo');
    expect(titleFor(3)).toBe('(3) GoThermo');
    expect(titleFor(150)).toBe('(99+) GoThermo');
  });
  it('counts up and clears', () => {
    addUnseen();
    addUnseen();
    expect(document.title).toBe('(2) GoThermo');
    clearUnseen();
    expect(document.title).toBe('GoThermo');
  });
});
