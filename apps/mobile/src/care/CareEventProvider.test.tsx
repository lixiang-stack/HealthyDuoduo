import { render, screen } from '@testing-library/react-native';
import { useEffect, useState, type ReactNode } from 'react';
import { Text } from 'react-native';

import { WIDE_CARE_EVENT_RANGE } from '../test/fixtures';
import { CareEventProvider, useCareEventRepository } from './CareEventProvider';
import { CARE_EVENT_FIXTURES } from './careEventFixtures';

function CountEvents() {
  const repository = useCareEventRepository();
  const [count, setCount] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    // Check every owner used by the synthetic fixtures, so a future fixture for
    // another child cannot sneak into production unnoticed.
    const memberIds = [...new Set(CARE_EVENT_FIXTURES.map((event) => event.member_id))];
    void Promise.all(
      memberIds.map((memberId) =>
        repository.list({ member_id: memberId, ...WIDE_CARE_EVENT_RANGE }),
      ),
    ).then((pages) => {
      if (!cancelled) {
        setCount(pages.reduce((total, events) => total + events.length, 0));
      }
    });
    return () => {
      cancelled = true;
    };
  }, [repository]);

  return <Text testID="event-count">{count ?? 'loading'}</Text>;
}

function wrapper({ children }: { children: ReactNode }) {
  return <CareEventProvider>{children}</CareEventProvider>;
}

describe('CareEventProvider', () => {
  it('starts production with no synthetic care events', async () => {
    expect(CARE_EVENT_FIXTURES.length).toBeGreaterThan(0);

    await render(<CountEvents />, { wrapper });

    expect(await screen.findByTestId('event-count')).toHaveTextContent('0');
  });
});
