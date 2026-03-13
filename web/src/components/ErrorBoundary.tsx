import { Component } from 'react';
import type { ReactNode, ErrorInfo } from 'react';
import { reportError } from '../lib/telemetry';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError('ErrorBoundary', error);
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div data-testid="error-boundary" className="flex items-center justify-center h-screen bg-deep text-fg">
          <div className="text-center space-y-4">
            <h1 className="text-xl font-semibold">Something went wrong</h1>
            <p className="text-fg-secondary text-sm max-w-md">
              {this.state.error?.message || 'An unexpected error occurred'}
            </p>
            <button
              data-testid="error-retry"
              onClick={this.handleRetry}
              className="px-4 py-2 bg-soul text-deep rounded-lg hover:bg-soul/85 transition-colors cursor-pointer"
            >
              Try Again
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
