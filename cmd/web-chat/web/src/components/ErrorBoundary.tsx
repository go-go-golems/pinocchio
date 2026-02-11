import React from 'react';
import { logError } from '../utils/logger';

type ErrorBoundaryProps = {
  children: React.ReactNode;
};

type ErrorBoundaryState = {
  hasError: boolean;
};

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: unknown, info: unknown) {
    logError('render error', error, { scope: 'ErrorBoundary', extra: { info } });
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ padding: 16 }}>
          <div className="card">
            <div className="cardHeader">
              <div className="cardHeaderTitle">Something went wrong</div>
            </div>
            <div className="cardBody">
              <div className="pill">Check the console for details.</div>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
