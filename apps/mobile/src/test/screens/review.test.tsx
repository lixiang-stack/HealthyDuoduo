import { fireEvent, screen, waitFor } from '@testing-library/react-native';

import { resolveApiClient } from '../../api/resolveClient';
import ReviewScreen from '../../app/review/[report_id]';
import { createFakeApiClient, TEST_BASE_URL, type FakeApiOptions } from '../fakeApi';
import { makeFlagCounts, makeReportDetail, makeReportItem, makeReviewResult } from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockUseLocalSearchParams } from '../routerMock';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

const lowConfidenceItem = makeReportItem({
  item_id: 1,
  name: '总胆红素',
  value: '25.00',
  ref_range: '2-21',
  flag: 'high',
  low_confidence: true,
  review_status: 'unreviewed',
});

function useReviewApi(item = lowConfidenceItem) {
  const report = makeReportDetail({
    status: 'partial',
    unreviewed_low_confidence_count: 1,
    flag_counts: makeFlagCounts({ high: 1 }),
    items: [item],
  });

  const patch = jest.fn(async (options?: FakeApiOptions) => {
    const body = (options?.body ?? {}) as Record<string, unknown>;
    return makeReviewResult({
      ...item,
      value: typeof body.value === 'string' ? body.value : item.value,
      review_status: body.action === 'correct' ? 'corrected' : 'confirmed',
    });
  });

  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({
      GET: { '/reports/{report_id}': () => report },
      PATCH: { '/reports/{report_id}/items/{item_id}': patch },
    }),
    baseUrl: TEST_BASE_URL,
  });

  return { patch };
}

beforeEach(() => {
  jest.clearAllMocks();
  mockUseLocalSearchParams.mockReturnValue({ report_id: 'r1' });
});

describe('ReviewScreen', () => {
  it('confirms a low-confidence item and clears the pending list', async () => {
    const { patch } = useReviewApi();

    await renderScreen(<ReviewScreen />);

    expect(await screen.findByTestId('review-item-1')).toHaveTextContent(/低置信/);
    await fireEvent.press(screen.getByTestId('review-confirm-1'));

    await waitFor(() =>
      expect(patch).toHaveBeenCalledWith(
        expect.objectContaining({ body: expect.objectContaining({ action: 'confirm' }) }),
      ),
    );
    expect(await screen.findByText('没有需要核对的低置信项。')).toBeTruthy();
  });

  it('edits a value and submits a correction', async () => {
    const { patch } = useReviewApi();

    await renderScreen(<ReviewScreen />);

    await screen.findByTestId('review-item-1');
    await fireEvent.press(screen.getByTestId('review-correct-1'));
    await fireEvent.changeText(screen.getByTestId('review-field-value-1'), '30.00');
    await fireEvent.press(screen.getByTestId('review-save-1'));

    await waitFor(() =>
      expect(patch).toHaveBeenCalledWith(
        expect.objectContaining({
          body: expect.objectContaining({ action: 'correct', value: '30.00' }),
        }),
      ),
    );
    expect(await screen.findByText('没有需要核对的低置信项。')).toBeTruthy();
  });

  it('offers backfill for an actionable unknown item', async () => {
    useReviewApi(
      makeReportItem({
        item_id: 3,
        name: '血清淀粉样蛋白A',
        value: null,
        ref_range: '0-10',
        flag: 'unknown',
        flag_reason: 'ocr_miss',
        low_confidence: false,
      }),
    );

    await renderScreen(<ReviewScreen />);

    expect(await screen.findByTestId('review-item-3')).toBeTruthy();
    expect(screen.getByTestId('review-reason-3')).toHaveTextContent(/OCR/);
    expect(screen.getByTestId('review-backfill-3')).toBeTruthy();
  });
});
