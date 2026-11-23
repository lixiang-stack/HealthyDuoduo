import AsyncStorage from '@react-native-async-storage/async-storage';
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { Pressable, Text, View } from 'react-native';

import {
  CURRENT_CHILD_STORAGE_KEY,
  CURRENT_MEMBER_STORAGE_KEY,
} from './currentMemberStore';
import { CurrentMemberProvider, useCurrentMember } from './CurrentMemberContext';

function Probe({ name }: { name: string }) {
  const {
    currentMemberId,
    currentChildId,
    childReady,
    selectMember,
    setCurrentChild,
  } = useCurrentMember();
  return (
    <View>
      <Text testID={`${name}-member`}>{currentMemberId ?? 'none'}</Text>
      <Text testID={`${name}-child`}>
        {childReady ? (currentChildId ?? 'none') : 'loading'}
      </Text>
      <Pressable testID={`${name}-member-2`} onPress={() => selectMember(2)} />
      <Pressable testID={`${name}-child-1`} onPress={() => setCurrentChild(1)} />
      <Pressable testID={`${name}-child-3`} onPress={() => setCurrentChild(3)} />
    </View>
  );
}

beforeEach(async () => {
  jest.clearAllMocks();
  await AsyncStorage.clear();
});

describe('CurrentMemberProvider', () => {
  it('restores the persisted current child', async () => {
    await AsyncStorage.setItem(CURRENT_CHILD_STORAGE_KEY, '7');

    await render(
      <CurrentMemberProvider>
        <Probe name="a" />
      </CurrentMemberProvider>,
    );

    expect(await screen.findByText('7')).toBeTruthy();
    expect(screen.getByTestId('a-child')).toHaveTextContent('7');
  });

  it('keeps two device-local context instances independent and stores member / child separately', async () => {
    await render(
      <View>
        <CurrentMemberProvider>
          <Probe name="a" />
        </CurrentMemberProvider>
        <CurrentMemberProvider>
          <Probe name="b" />
        </CurrentMemberProvider>
      </View>,
    );
    await waitFor(() => expect(screen.getByTestId('a-child')).toHaveTextContent('none'));
    await waitFor(() => expect(screen.getByTestId('b-child')).toHaveTextContent('none'));

    await fireEvent.press(screen.getByTestId('a-child-1'));
    await fireEvent.press(screen.getByTestId('b-child-3'));
    await fireEvent.press(screen.getByTestId('a-member-2'));

    // The two providers stand in for two devices; the shared AsyncStorage mock is a
    // harness limitation, so assert each instance's in-memory choice plus the
    // separate storage slots that prevent member / child clobbering.
    await waitFor(() => expect(screen.getByTestId('a-child')).toHaveTextContent('1'));
    await waitFor(() => expect(screen.getByTestId('b-child')).toHaveTextContent('3'));
    expect(screen.getByTestId('a-member')).toHaveTextContent('2');
    expect(screen.getByTestId('b-member')).toHaveTextContent('none');
    expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY, '1');
    expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_CHILD_STORAGE_KEY, '3');
    expect(AsyncStorage.setItem).toHaveBeenCalledWith(CURRENT_MEMBER_STORAGE_KEY, '2');
    expect(AsyncStorage.setItem).not.toHaveBeenCalledWith(CURRENT_MEMBER_STORAGE_KEY, '1');
  });

  it('falls back to unselected instead of hanging when local storage cannot be read', async () => {
    const getItem = jest
      .spyOn(AsyncStorage, 'getItem')
      .mockRejectedValue(new Error('storage unavailable'));

    await render(
      <CurrentMemberProvider>
        <Probe name="a" />
      </CurrentMemberProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('a-child')).toHaveTextContent('none'));
    expect(screen.getByTestId('a-member')).toHaveTextContent('none');
    getItem.mockRestore();
  });
});
