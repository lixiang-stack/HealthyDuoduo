import type { CareEvent } from './careEvents';
import { makeFeedEvent } from '../test/fixtures';
import { createRecordDraft, editRecordDraft, recordDraftErrors, recordDraftToBody } from './recordDraft';

const NOW = new Date(2026, 10, 23, 14, 5);

describe('record drafts', () => {
  it('defaults to the current local time and a diaper type', () => {
    const feed = createRecordDraft('feed', NOW);
    expect(feed).toMatchObject({
      type: 'feed',
      date: '2026-11-23',
      time: '14:05',
      note: '',
      amountMl: '',
      durationMin: '',
    });
    expect(feed.method).toBeUndefined();

    expect(createRecordDraft('diaper', NOW)).toMatchObject({
      type: 'diaper',
      diaperType: 'wet',
    });
    expect(createRecordDraft('sleep', NOW)).toMatchObject({ type: 'sleep', durationMin: '' });
    expect(createRecordDraft('solid', NOW)).toMatchObject({
      type: 'solid',
      food: '',
      amountG: '',
    });
  });

  it('keeps an edited event’s original local date across midnight', () => {
    const sleep: CareEvent = {
      id: 4,
      member_id: 1,
      type: 'sleep',
      started_at: new Date(2026, 10, 22, 23, 50).toISOString(),
      ended_at: new Date(2026, 10, 23, 6, 20).toISOString(),
      note: null,
      source: 'parent',
      duration_min: 390,
    };

    expect(editRecordDraft(sleep)).toMatchObject({
      date: '2026-11-22',
      time: '23:50',
      durationMin: '390',
    });
  });

  it('fills each type’s editable fields from the event', () => {
    const feed = makeFeedEvent({
      started_at: new Date(2026, 10, 23, 14, 5).toISOString(),
      note: '剩下 20 ml',
    });
    const solid: CareEvent = {
      id: 5,
      member_id: 1,
      type: 'solid',
      started_at: new Date(2026, 10, 23, 12, 0).toISOString(),
      ended_at: null,
      note: null,
      source: 'parent',
      food: '米粉',
      amount_g: 40,
    };

    expect(editRecordDraft(feed)).toMatchObject({
      method: 'formula',
      amountMl: '120',
      durationMin: '',
      note: '剩下 20 ml',
    });
    expect(editRecordDraft(solid)).toMatchObject({ food: '米粉', amountG: '40', note: '' });
  });
});

describe('record draft validation', () => {
  function validFeed() {
    return { ...createRecordDraft('feed', NOW), method: 'formula' as const };
  }

  it('requires a feeding method but allows empty amount and note', () => {
    expect(recordDraftErrors(createRecordDraft('feed', NOW))).toMatchObject({
      method: '请选择喂养方式',
    });
    expect(recordDraftErrors(validFeed())).toEqual({});
    expect(recordDraftErrors({ ...validFeed(), amountMl: '120', note: '任意备注' })).toEqual({});
  });

  it('rejects malformed times', () => {
    expect(recordDraftErrors({ ...validFeed(), time: '24:00' })).toMatchObject({
      time: '请填写 HH:MM 格式的时间',
    });
    expect(recordDraftErrors({ ...validFeed(), time: '9:5' })).toMatchObject({
      time: '请填写 HH:MM 格式的时间',
    });
    expect(recordDraftErrors({ ...validFeed(), time: '23:59' })).toEqual({});
  });

  it('rejects non-positive or non-numeric amounts and durations', () => {
    expect(recordDraftErrors({ ...validFeed(), amountMl: '0' })).toMatchObject({
      amountMl: '奶量需为正数',
    });
    expect(recordDraftErrors({ ...validFeed(), amountMl: '-5' })).toMatchObject({
      amountMl: '奶量需为正数',
    });
    expect(recordDraftErrors({ ...validFeed(), amountMl: 'abc' })).toMatchObject({
      amountMl: '奶量需为正数',
    });
    expect(
      recordDraftErrors({ ...validFeed(), method: 'breast', durationMin: '1.5' }),
    ).toMatchObject({ durationMin: '时长需为正整数分钟' });
    expect(recordDraftErrors({ ...validFeed(), method: 'breast', durationMin: '18' })).toEqual({});
  });

  it('requires solid food and validates its optional amount', () => {
    expect(recordDraftErrors(createRecordDraft('solid', NOW))).toMatchObject({
      food: '请填写食物名称',
    });
    expect(
      recordDraftErrors({ ...createRecordDraft('solid', NOW), food: '   ' }),
    ).toMatchObject({ food: '请填写食物名称' });
    expect(
      recordDraftErrors({ ...createRecordDraft('solid', NOW), food: '米粉', amountG: '0' }),
    ).toMatchObject({ amountG: '数量需为正数' });
    expect(
      recordDraftErrors({ ...createRecordDraft('solid', NOW), food: '米粉', amountG: '40' }),
    ).toEqual({});
  });

  it('allows an empty sleep duration but rejects invalid ones', () => {
    expect(recordDraftErrors(createRecordDraft('sleep', NOW))).toEqual({});
    expect(
      recordDraftErrors({ ...createRecordDraft('sleep', NOW), durationMin: '-1' }),
    ).toMatchObject({ durationMin: '时长需为正整数分钟' });
    expect(
      recordDraftErrors({ ...createRecordDraft('sleep', NOW), durationMin: '45' }),
    ).toEqual({});
  });
});

describe('record payloads', () => {
  it('writes formula amount, trims note and keeps feed as a point event', () => {
    const draft = {
      ...createRecordDraft('feed', NOW),
      method: 'formula' as const,
      amountMl: '120',
      durationMin: '99',
      note: '  剩余 20 ml  ',
    };

    expect(recordDraftToBody(draft)).toEqual({
      type: 'feed',
      started_at: new Date(2026, 10, 23, 14, 5).toISOString(),
      ended_at: null,
      note: '剩余 20 ml',
      method: 'formula',
      amount_ml: 120,
      duration_min: null,
    });
  });

  it('writes breastfeeding duration and ignores the formula amount', () => {
    const draft = {
      ...createRecordDraft('feed', NOW),
      method: 'breast' as const,
      amountMl: '120',
      durationMin: '18',
      note: '',
    };

    expect(recordDraftToBody(draft)).toMatchObject({
      method: 'breast',
      amount_ml: null,
      duration_min: 18,
      note: null,
    });
  });

  it('derives a sleep end from its duration and keeps an empty duration ongoing', () => {
    const base = createRecordDraft('sleep', NOW);

    expect(recordDraftToBody({ ...base, durationMin: '45' })).toMatchObject({
      type: 'sleep',
      started_at: new Date(2026, 10, 23, 14, 5).toISOString(),
      ended_at: new Date(2026, 10, 23, 14, 50).toISOString(),
      duration_min: 45,
    });
    expect(recordDraftToBody(base)).toMatchObject({ ended_at: null, duration_min: null });
  });

  it('trims solid food and writes optional grams', () => {
    expect(
      recordDraftToBody({
        ...createRecordDraft('solid', NOW),
        food: '  米粉 ',
        amountG: '40',
      }),
    ).toMatchObject({ type: 'solid', food: '米粉', amount_g: 40, ended_at: null });
  });

  it('writes the selected diaper type', () => {
    expect(
      recordDraftToBody({ ...createRecordDraft('diaper', NOW), diaperType: 'mixed' }),
    ).toMatchObject({ type: 'diaper', diaper_type: 'mixed', ended_at: null });
  });
});
