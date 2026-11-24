// Editable record-sheet draft: one flat shape for all four care types, with
// validation and conversion to the ticket-01 CareEvent body. Times are device-local
// HH:MM anchored to a local date so editing a cross-midnight event keeps its day.
import type {
  CareEvent,
  CareEventBody,
  CareEventType,
  DiaperType,
  FeedMethod,
} from './careEvents';
import { careEventDurationMinutes } from './careEvents';
import { localClockTime, localDateKey } from './careEventDisplay';

export type RecordDraft = {
  type: CareEventType;
  /** Device-local YYYY-MM-DD the HH:MM belongs to. */
  date: string;
  /** Device-local HH:MM. */
  time: string;
  note: string;
  method?: FeedMethod;
  amountMl: string;
  durationMin: string;
  diaperType: DiaperType;
  food: string;
  amountG: string;
};

export type RecordDraftErrors = Partial<
  Record<'time' | 'method' | 'amountMl' | 'durationMin' | 'food' | 'amountG', string>
>;

const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/;
const AMOUNT_PATTERN = /^\d+(\.\d+)?$/;
const MINUTES_PATTERN = /^\d+$/;

export function createRecordDraft(type: CareEventType, now: Date): RecordDraft {
  return {
    type,
    date: localDateKey(now),
    time: localClockTime(now),
    note: '',
    amountMl: '',
    durationMin: '',
    diaperType: 'wet',
    food: '',
    amountG: '',
  };
}

export function editRecordDraft(event: CareEvent): RecordDraft {
  const draft: RecordDraft = {
    type: event.type,
    date: localDateKey(event.started_at),
    time: localClockTime(event.started_at),
    note: event.note ?? '',
    amountMl: '',
    durationMin: '',
    diaperType: 'wet',
    food: '',
    amountG: '',
  };
  switch (event.type) {
    case 'feed':
      return {
        ...draft,
        method: event.method,
        amountMl: event.amount_ml === null ? '' : String(event.amount_ml),
        durationMin: event.duration_min === null ? '' : String(event.duration_min),
      };
    case 'diaper':
      return { ...draft, diaperType: event.diaper_type };
    case 'sleep': {
      const duration = careEventDurationMinutes(event);
      return { ...draft, durationMin: duration === null ? '' : String(duration) };
    }
    case 'solid':
      return {
        ...draft,
        food: event.food,
        amountG: event.amount_g === null ? '' : String(event.amount_g),
      };
  }
}

export function recordDraftErrors(draft: RecordDraft): RecordDraftErrors {
  const errors: RecordDraftErrors = {};
  if (!TIME_PATTERN.test(draft.time.trim())) {
    errors.time = '请填写 HH:MM 格式的时间';
  }
  if (draft.type === 'feed') {
    if (draft.method === undefined) {
      errors.method = '请选择喂养方式';
    }
    if (draft.method === 'formula' && !positiveAmount(draft.amountMl)) {
      if (draft.amountMl.trim() !== '') {
        errors.amountMl = '奶量需为正数';
      }
    }
    if (draft.method === 'breast' && !positiveMinutes(draft.durationMin)) {
      if (draft.durationMin.trim() !== '') {
        errors.durationMin = '时长需为正整数分钟';
      }
    }
  } else if (draft.type === 'sleep') {
    if (!positiveMinutes(draft.durationMin) && draft.durationMin.trim() !== '') {
      errors.durationMin = '时长需为正整数分钟';
    }
  } else if (draft.type === 'solid') {
    if (draft.food.trim() === '') {
      errors.food = '请填写食物名称';
    }
    if (!positiveAmount(draft.amountG) && draft.amountG.trim() !== '') {
      errors.amountG = '数量需为正数';
    }
  }
  return errors;
}

/** Call only after `recordDraftErrors` is empty. */
export function recordDraftToBody(draft: RecordDraft): CareEventBody {
  const started_at = localInstant(draft.date, draft.time.trim());
  const note = draft.note.trim() || null;
  switch (draft.type) {
    case 'feed': {
      const method = draft.method;
      if (method === undefined) {
        // Validation guarantees this; the guard narrows the type.
        throw new Error('feed method is required');
      }
      return {
        type: 'feed',
        started_at,
        ended_at: null,
        note,
        method,
        amount_ml: method === 'formula' ? optionalNumber(draft.amountMl) : null,
        duration_min: method === 'breast' ? optionalNumber(draft.durationMin) : null,
      };
    }
    case 'diaper':
      return {
        type: 'diaper',
        started_at,
        ended_at: null,
        note,
        diaper_type: draft.diaperType,
      };
    case 'sleep': {
      const duration = optionalNumber(draft.durationMin);
      return {
        type: 'sleep',
        started_at,
        ended_at:
          duration === null
            ? null
            : new Date(Date.parse(started_at) + duration * 60000).toISOString(),
        note,
        duration_min: duration,
      };
    }
    case 'solid':
      return {
        type: 'solid',
        started_at,
        ended_at: null,
        note,
        food: draft.food.trim(),
        amount_g: optionalNumber(draft.amountG),
      };
  }
}

function localInstant(dateKey: string, time: string): string {
  const [year, month, day] = dateKey.split('-').map(Number);
  const [hour, minute] = time.split(':').map(Number);
  return new Date(year, month - 1, day, hour, minute).toISOString();
}

function positiveAmount(value: string): boolean {
  const trimmed = value.trim();
  return AMOUNT_PATTERN.test(trimmed) && Number(trimmed) > 0;
}

function positiveMinutes(value: string): boolean {
  const trimmed = value.trim();
  return MINUTES_PATTERN.test(trimmed) && Number(trimmed) > 0;
}

function optionalNumber(value: string): number | null {
  return value.trim() === '' ? null : Number(value.trim());
}
