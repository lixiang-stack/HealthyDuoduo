import { fireEvent, screen } from '@testing-library/react-native';
import { Image } from 'react-native';

import { resolveApiClient } from '../../api/resolveClient';
import ReportDetailScreen from '../../app/report/[report_id]';
import { createFakeApiClient, TEST_BASE_URL } from '../fakeApi';
import { makeFlagCounts, makeReportDetail, makeReportItem } from '../fixtures';
import { renderScreen } from '../renderScreen';
import { mockUseLocalSearchParams } from '../routerMock';

jest.mock('../../api/resolveClient', () => ({
  resolveApiClient: jest.fn(),
}));

const mockedResolveApiClient = jest.mocked(resolveApiClient);

function useReport(detail: ReturnType<typeof makeReportDetail>) {
  mockedResolveApiClient.mockResolvedValue({
    client: createFakeApiClient({
      GET: {
        '/reports/{report_id}': () => detail,
      },
    }),
    baseUrl: TEST_BASE_URL,
  });
}

beforeEach(() => {
  jest.clearAllMocks();
  mockUseLocalSearchParams.mockReturnValue({ report_id: 'r1' });
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe('ReportDetailScreen', () => {
  it('renders the report header, grouped items and the pending review entry', async () => {
    useReport(
      makeReportDetail({
        status: 'partial',
        unreviewed_low_confidence_count: 1,
        flag_counts: makeFlagCounts({ high: 1 }),
        items: [
          makeReportItem({ item_id: 1, name: '丙氨酸转氨酶', flag: 'normal' }),
          makeReportItem({
            item_id: 2,
            name: '总胆红素',
            value: '25.00',
            ref_range: '2-21',
            flag: 'high',
            low_confidence: true,
          }),
        ],
      }),
    );

    await renderScreen(<ReportDetailScreen />);

    expect(await screen.findByTestId('report-card-r1')).toBeTruthy();
    expect(screen.getByTestId('report-short-id')).toHaveTextContent('报告 r1');
    expect(screen.getByTestId('report-panel-group-liver')).toBeTruthy();
    expect(screen.getByText('丙氨酸氨基转移酶')).toBeTruthy();
    expect(screen.getByText('总胆红素')).toBeTruthy();
    expect(screen.getByTestId('report-pending-entry')).toHaveTextContent(/逐项核对 1 项/);
    expect(screen.getByTestId('report-manage-reassign')).toBeTruthy();
    expect(screen.getByTestId('report-manage-delete')).toBeTruthy();
  });

  it('renders a failed report with its reason and re-upload entry', async () => {
    useReport(
      makeReportDetail({
        status: 'failed',
        fail_reason: '图像无法识别',
        panels: [],
        items: [],
      }),
    );

    await renderScreen(<ReportDetailScreen />);

    expect(await screen.findByText('图像无法识别')).toBeTruthy();
    expect(screen.getByTestId('report-image')).toBeTruthy();
    expect(screen.getByText(/重新拍照上传/)).toBeTruthy();
    expect(screen.queryByTestId('report-items-title')).toBeNull();
  });

  it('opens the full-screen viewer from the report image', async () => {
    jest
      .spyOn(Image, 'getSize')
      .mockImplementation(((_url: string, success: (width: number, height: number) => void) =>
        success(1000, 1500)) as typeof Image.getSize);
    useReport(
      makeReportDetail({
        status: 'failed',
        fail_reason: '图像无法识别',
        panels: [],
        items: [],
      }),
    );

    await renderScreen(<ReportDetailScreen />);

    fireEvent.press(await screen.findByTestId('report-image-open'));
    const root = await screen.findByTestId('image-viewer-root');
    fireEvent(root, 'layout', {
      nativeEvent: { layout: { width: 390, height: 844 } },
    });

    expect(await screen.findByTestId('image-viewer-image')).toBeTruthy();
  });

  it('renders narrative sections without an items section', async () => {
    useReport(
      makeReportDetail({
        report_id: 'r1',
        report_type: '影像',
        panels: ['imaging'],
        items: [],
        sections: [
          {
            kind: 'findings',
            text: '肝脏大小形态正常。',
            ocr_lines: [],
          },
          {
            kind: 'impression',
            text: '未见明显异常。',
            ocr_lines: [],
          },
        ],
      }),
    );

    await renderScreen(<ReportDetailScreen />);

    expect(await screen.findByTestId('report-section-0')).toHaveTextContent(/检查所见/);
    expect(screen.getByTestId('report-section-0')).toHaveTextContent(/肝脏大小形态正常。/);
    expect(screen.getByTestId('report-section-1')).toHaveTextContent(/超声提示/);
    expect(screen.queryByTestId('report-items-title')).toBeNull();
  });

  it('shows the load error and retries the report request', async () => {
    let failing = true;
    mockedResolveApiClient.mockResolvedValue({
      client: createFakeApiClient({
        GET: {
          '/reports/{report_id}': () => {
            if (failing) {
              throw new Error('backend down');
            }
            return makeReportDetail({ report_id: 'r1' });
          },
        },
      }),
      baseUrl: TEST_BASE_URL,
    });

    await renderScreen(<ReportDetailScreen />);

    expect(await screen.findByText('加载失败:backend down')).toBeTruthy();

    failing = false;
    await fireEvent.press(screen.getByText('重试'));

    expect(await screen.findByTestId('report-card-r1')).toBeTruthy();
  });
});
