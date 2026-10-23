import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// jsdom has no scrollIntoView / requestAnimationFrame-driven layout.
Element.prototype.scrollIntoView = function () {};

afterEach(() => cleanup());
