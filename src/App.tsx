import { Component } from 'solid-js';
import { createSignal } from 'solid-js';
import NavigationControls from './components/NavigationControls';
import WebView from './components/WebView';

const App: Component = () => {
  const [currentUrl, setCurrentUrl] = createSignal('');
  const [history, setHistory] = createSignal<string[]>([]);
  const [historyIndex, setHistoryIndex] = createSignal(-1);

  const navigate = (url: string) => {
    setCurrentUrl(url);
    setHistory([...history().slice(0, historyIndex() + 1), url]);
    setHistoryIndex(historyIndex() + 1);
  };

  const goBack = () => {
    if (historyIndex() > 0) {
      setHistoryIndex(historyIndex() - 1);
      setCurrentUrl(history()[historyIndex() - 1]);
    }
  };

  const goForward = () => {
    if (historyIndex() < history().length - 1) {
      setHistoryIndex(historyIndex() + 1);
      setCurrentUrl(history()[historyIndex() + 1]);
    }
  };

  const reload = () => {
    setCurrentUrl(currentUrl());
  };

  return (
    <div class="web-navigator">
      <NavigationControls 
        currentUrl={currentUrl()}
        onNavigate={navigate}
        onBack={goBack}
        onForward={goForward}
        onReload={reload}
        canGoBack={historyIndex() > 0}
        canGoForward={historyIndex() < history().length - 1}
      />
      <WebView url={currentUrl()} />
    </div>
  );
};

export default App; 