import { makeAnalysisFramework } from '../test/fixtures';
import {
  EXPLANATION_UNAVAILABLE_TEXT,
  NO_ENTRIES_GUIDANCE_TEXT,
  QUOTA_EXHAUSTED_HISTORY_TEXT,
  QUOTA_UNAVAILABLE_TEXT,
  QUOTA_EXHAUSTED_TEXT,
  explanationUnavailableText,
  frameworkSourceText,
  hasFrameworkLabels,
} from './framework';

describe('analysis framework display helpers', () => {
  it('renders sources from code-supplied standards only', () => {
    const framework = makeAnalysisFramework();
    expect(hasFrameworkLabels(framework)).toBe(true);
    expect(frameworkSourceText(framework.labels[0])).toBe('ACG 2016');
  });

  it('treats an empty framework as absent', () => {
    expect(hasFrameworkLabels(undefined)).toBe(false);
    expect(hasFrameworkLabels({ labels: [] })).toBe(false);
  });

  it('shows quota exhaustion before other reasons', () => {
    expect(explanationUnavailableText('quota_exhausted', false)).toBe(QUOTA_EXHAUSTED_TEXT);
    expect(explanationUnavailableText('quota_exhausted', true)).toBe(QUOTA_EXHAUSTED_HISTORY_TEXT);
    expect(explanationUnavailableText('llm_timeout', true)).toBe(EXPLANATION_UNAVAILABLE_TEXT);
    expect(explanationUnavailableText('llm_timeout', false)).toBe(EXPLANATION_UNAVAILABLE_TEXT);
    expect(explanationUnavailableText('no_entries', true)).toBe(NO_ENTRIES_GUIDANCE_TEXT);
    expect(explanationUnavailableText('quota_unavailable', true)).toBe(QUOTA_UNAVAILABLE_TEXT);
  });
});
