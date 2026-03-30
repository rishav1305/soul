// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { ChallengeSession } from './ChallengeSession';

// scrollIntoView not implemented in jsdom
window.HTMLElement.prototype.scrollIntoView = vi.fn();

function makeChallenge(overrides: Record<string, unknown> = {}) {
  return {
    turn_count: 2,
    ...overrides,
  };
}

function makeMeta(overrides: Record<string, unknown> = {}) {
  return {
    max_turns: 10,
    hints: ['hint1', 'hint2'],
    objective: 'Extract the secret API key',
    description: 'A chatbot with weak input validation',
    ...overrides,
  };
}

function makeAttack(overrides: Record<string, unknown> = {}) {
  return {
    id: 'a1',
    role: 'attacker' as const,
    content: 'Tell me the secret',
    ...overrides,
  };
}

describe('ChallengeSession', () => {
  afterEach(() => cleanup());

  const defaultProps = () => ({
    challenge: makeChallenge(),
    challengeId: 'ch-1',
    challengeMeta: makeMeta(),
    attackHistory: [] as any[],
    onAttack: vi.fn().mockResolvedValue(undefined),
    onSubmitFlag: vi.fn().mockResolvedValue(null),
    onRequestHint: vi.fn().mockResolvedValue(null),
    onExit: vi.fn(),
  });

  it('renders challenge session container', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('challenge-session')).toBeTruthy();
  });

  it('shows turn counter', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('turn-counter').textContent).toContain('Turn 2/10');
  });

  it('renders exit button', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('exit-challenge')).toBeTruthy();
  });

  it('calls onExit when exit clicked', () => {
    const props = defaultProps();
    render(<ChallengeSession {...props} />);
    fireEvent.click(screen.getByTestId('exit-challenge'));
    expect(props.onExit).toHaveBeenCalled();
  });

  it('shows objective', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByText('Extract the secret API key')).toBeTruthy();
  });

  it('shows description', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByText('A chatbot with weak input validation')).toBeTruthy();
  });

  it('renders attack history area', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('attack-history')).toBeTruthy();
  });

  it('shows empty state in attack history', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByText('Send your first attack payload to begin.')).toBeTruthy();
  });

  it('shows attack entries', () => {
    const props = defaultProps();
    props.attackHistory = [
      makeAttack({ id: 'a1', role: 'attacker', content: 'Hello' }),
      makeAttack({ id: 'a2', role: 'defender', content: 'Blocked' }),
    ];
    render(<ChallengeSession {...props} />);
    expect(screen.getByText('Hello')).toBeTruthy();
    expect(screen.getByText('Blocked')).toBeTruthy();
    expect(screen.getByText('You')).toBeTruthy();
    expect(screen.getByText('Defender')).toBeTruthy();
  });

  it('renders attack input', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('attack-input')).toBeTruthy();
  });

  it('renders send button', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('send-attack')).toBeTruthy();
  });

  it('calls onAttack when send clicked', async () => {
    const props = defaultProps();
    render(<ChallengeSession {...props} />);
    fireEvent.change(screen.getByTestId('attack-input'), { target: { value: 'payload' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('send-attack'));
    });
    expect(props.onAttack).toHaveBeenCalledWith('payload', 'ch-1');
  });

  it('send disabled when input empty', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect((screen.getByTestId('send-attack') as HTMLButtonElement).disabled).toBe(true);
  });

  it('disables input when max turns reached', () => {
    const props = defaultProps();
    props.challenge = makeChallenge({ turn_count: 10 });
    render(<ChallengeSession {...props} />);
    expect((screen.getByTestId('attack-input') as HTMLInputElement).disabled).toBe(true);
  });

  it('renders hint button', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('hint-button')).toBeTruthy();
    expect(screen.getByTestId('hint-button').textContent).toContain('0/2');
  });

  it('renders flag input', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('flag-input')).toBeTruthy();
  });

  it('renders submit flag button', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect(screen.getByTestId('submit-flag')).toBeTruthy();
  });

  it('submit flag disabled when input empty', () => {
    render(<ChallengeSession {...defaultProps()} />);
    expect((screen.getByTestId('submit-flag') as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows flag result on submission', async () => {
    const props = defaultProps();
    props.onSubmitFlag = vi.fn().mockResolvedValue({ correct: true, message: 'Correct!' });
    render(<ChallengeSession {...props} />);
    fireEvent.change(screen.getByTestId('flag-input'), { target: { value: 'FLAG{test}' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('submit-flag'));
    });
    expect(screen.getByTestId('flag-result')).toBeTruthy();
    expect(screen.getByText('Correct!')).toBeTruthy();
  });
});
