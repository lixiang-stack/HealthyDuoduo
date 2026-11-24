import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { Pressable, Text, View } from 'react-native';

import { DomainNavigationProvider, useDomainNavigation } from './DomainNavigationContext';
import type { Domain } from './domainRoutes';
import { mockPathname, mockRouter } from '../test/routerMock';

function Probe({ active }: { active: Domain }) {
  const { state, navigateFromDomain } = useDomainNavigation();
  return (
    <View>
      <Text testID="last-care">{state.lastPaths.care}</Text>
      <Text testID="last-health">{state.lastPaths.health}</Text>
      <Pressable testID="to-care" onPress={() => navigateFromDomain(active, 'care')} />
      <Pressable testID="to-health" onPress={() => navigateFromDomain(active, 'health')} />
    </View>
  );
}

async function renderAt(pathname: string, active: Domain) {
  mockPathname.current = pathname;
  const result = await render(
    <DomainNavigationProvider>
      <Probe active={active} />
    </DomainNavigationProvider>,
  );
  // Wait for the async preference load + persist effect so it cannot overlap the next act.
  await waitFor(() => expect(AsyncStorage.setItem).toHaveBeenCalled());
  return result;
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
  mockPathname.current = '/';
});

describe('DomainNavigationProvider', () => {
  it('preserves each domain secondary tab across appbar switches', async () => {
    const { rerender } = await renderAt('/care/today', 'care');

    mockPathname.current = '/care/stats';
    await rerender(
      <DomainNavigationProvider>
        <Probe active="care" />
      </DomainNavigationProvider>,
    );
    expect(await screen.findByText('/care/stats')).toBeTruthy();

    await fireEvent.press(await screen.findByTestId('to-health'));
    expect(mockRouter.replace).toHaveBeenLastCalledWith('/health/reports');

    mockPathname.current = '/health/trends';
    await rerender(
      <DomainNavigationProvider>
        <Probe active="health" />
      </DomainNavigationProvider>,
    );

    await fireEvent.press(await screen.findByTestId('to-care'));
    expect(mockRouter.replace).toHaveBeenLastCalledWith('/care/stats');
  });

  it('returns to the domain root when pressing the active domain', async () => {
    await renderAt('/care/stats', 'care');

    await fireEvent.press(await screen.findByTestId('to-care'));

    expect(mockRouter.replace).toHaveBeenLastCalledWith('/care/today');
  });
});
