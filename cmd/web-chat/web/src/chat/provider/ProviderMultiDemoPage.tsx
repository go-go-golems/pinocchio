import '../../webchat/styles/theme-default.css';
import '../../webchat/styles/webchat.css';
import { ProviderMultiDemoInstance } from './ProviderMultiDemoInstance';

export function ProviderMultiDemoPage() {
  return (
    <div data-pwchat="" data-part="root" data-theme="default" data-fullscreen="true">
      <header data-part="header">
        <h1>ChatProvider multi-instance smoke</h1>
      </header>
      <main data-part="main" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, padding: 16 }}>
        <ProviderMultiDemoInstance name="left" prompt="hello from left provider" />
        <ProviderMultiDemoInstance name="right" prompt="hello from right provider" />
      </main>
    </div>
  );
}
