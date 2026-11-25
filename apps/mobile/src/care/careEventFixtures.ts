// Synthetic care-event fixtures for tests and explicit previews only. Production
// injects the HTTP repository, so these must never be shown as real records. They
// cover all four types plus the boundaries the contract must keep: null note, null
// optional amounts, and a sleep spanning midnight.
import type { CareEvent } from './careEvents';

export const CARE_EVENT_FIXTURES: CareEvent[] = [
  {
    id: 1,
    member_id: 1,
    type: 'feed',
    started_at: '2026-11-23T14:05:00+08:00',
    ended_at: null,
    note: '奶粉 120 ml',
    source: 'parent',
    method: 'formula',
    amount_ml: 120,
    duration_min: null,
  },
  {
    id: 2,
    member_id: 1,
    type: 'feed',
    started_at: '2026-11-23T09:10:00+08:00',
    ended_at: '2026-11-23T09:28:00+08:00',
    note: null,
    source: 'parent',
    method: 'breast',
    amount_ml: null,
    duration_min: 18,
  },
  {
    id: 3,
    member_id: 1,
    type: 'diaper',
    started_at: '2026-11-23T08:20:00+08:00',
    ended_at: null,
    note: null,
    source: 'parent',
    diaper_type: 'wet',
  },
  {
    id: 4,
    member_id: 1,
    type: 'sleep',
    started_at: '2026-11-22T23:50:00+08:00',
    ended_at: '2026-11-23T06:20:00+08:00',
    note: null,
    source: 'parent',
    duration_min: 390,
  },
  {
    id: 5,
    member_id: 1,
    type: 'solid',
    started_at: '2026-11-23T12:00:00+08:00',
    ended_at: null,
    note: '第一次尝试',
    source: 'parent',
    food: '米粉',
    amount_g: null,
  },
];
