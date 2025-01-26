import { Component, createEffect } from 'solid-js';

interface WebViewProps {
  url: string;
}

const WebView: Component<WebViewProps> = (props) => {
  const fetchPage = async (url: string) => {
    if (!url) return;
    
    try {
      const response = await fetch(`http://localhost:8080/render?url=${encodeURIComponent(url)}`);
      const html = await response.text();
      const container = document.getElementById('web-view-container');
      if (container) {
        container.innerHTML = html;
      }
    } catch (error) {
      console.error('Error fetching page:', error);
    }
  };

  createEffect(() => {
    fetchPage(props.url);
  });

  return (
    <div id="web-view-container" class="web-view">
    </div>
  );
};

export default WebView; 