/**
 * frontend/src/lib/types — barrel re-export for backward compatibility.
 *
 * Formerly frontend/src/lib/types.ts (571 lines, 53 exports).
 * Types are now organized by domain. Import from sub-modules directly
 * for new code; this barrel preserves existing `import { X } from 'lib/types'` imports.
 */
export * from './common';
export * from './user';
export * from './agent';
export * from './repo';
export * from './cloud';
export * from './pentest';
export * from './threat-model';
export * from './autopilot';
export * from './api';
