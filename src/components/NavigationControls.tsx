import { Component } from 'solid-js';

interface NavigationControlsProps {
  currentUrl: string;
  onNavigate: (url: string) => void;
  onBack: () => void;
  onForward: () => void;
  onReload: () => void;
  canGoBack: boolean;
  canGoForward: boolean;
}

const NavigationControls: Component<NavigationControlsProps> = (props) => {
  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const form = e.target as HTMLFormElement;
    const input = form.elements.namedItem('url') as HTMLInputElement;
    props.onNavigate(input.value);
  };

  return (
    <div class="navigation-controls">
      <button 
        onClick={props.onBack} 
        disabled={!props.canGoBack}
      >
        ←
      </button>
      <button 
        onClick={props.onForward} 
        disabled={!props.canGoForward}
      >
        →
      </button>
      <button onClick={props.onReload}>↻</button>
      <form onSubmit={handleSubmit}>
        <input 
          type="text" 
          name="url" 
          value={props.currentUrl}
          placeholder="Enter URL"
        />
        <button type="submit">Go</button>
      </form>
    </div>
  );
};

export default NavigationControls; 