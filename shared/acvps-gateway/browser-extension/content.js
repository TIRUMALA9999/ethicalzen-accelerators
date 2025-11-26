// ACVPS Content Script - Intercepts fetch and XHR requests

(function() {
  'use strict';
  
  console.log('🔗 ACVPS Content Script injected');
  console.log('🏷️  Extension Version: 1.2.0 - ADDRESS BAR SUPPORT');
  console.log('📅 Last Updated: Oct 12, 2025 4:40 PM');
  
  // Inject script into page context to intercept fetch/XHR
  const script = document.createElement('script');
  script.src = chrome.runtime.getURL('injected.js');
  script.onload = function() {
    this.remove();
  };
  (document.head || document.documentElement).appendChild(script);
  
  // Listen for messages from injected script
  window.addEventListener('message', async (event) => {
    if (event.source !== window) return;
    
    if (event.data.type === 'ACVPS_FETCH_REQUEST') {
      const url = event.data.url;
      console.log('Intercepted fetch request:', url);
      
      // Check if this should use ACVPS
      if (url.startsWith('acvps://') || event.data.useACVPS) {
        // Get contract for this domain
        const domain = new URL(url.replace('acvps://', 'https://')).hostname;
        
        chrome.runtime.sendMessage(
          { type: 'GET_CONTRACT', domain },
          (response) => {
            if (response.success) {
              // Send contract back to page
              window.postMessage({
                type: 'ACVPS_CONTRACT',
                requestId: event.data.requestId,
                contract: response.contract
              }, '*');
            }
          }
        );
      }
    }
  });
  
  // Visual indicator that ACVPS is active
  function showACVPSIndicator() {
    if (document.getElementById('acvps-indicator')) return;
    
    const indicator = document.createElement('div');
    indicator.id = 'acvps-indicator';
    indicator.innerHTML = `
      <div style="
        position: fixed;
        top: 10px;
        right: 10px;
        background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        color: white;
        padding: 8px 16px;
        border-radius: 20px;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto;
        font-size: 12px;
        font-weight: 600;
        box-shadow: 0 4px 12px rgba(0,0,0,0.2);
        z-index: 999999;
        display: flex;
        align-items: center;
        gap: 8px;
      ">
        <span style="font-size: 16px;">🔒🔗</span>
        <span>ACVPS Protected</span>
      </div>
    `;
    document.body.appendChild(indicator);
    
    // Fade out after 3 seconds
    setTimeout(() => {
      indicator.style.transition = 'opacity 0.5s';
      indicator.style.opacity = '0';
      setTimeout(() => indicator.remove(), 500);
    }, 3000);
  }
  
  // Show indicator when page is ACVPS-enabled
  if (window.location.protocol === 'https:') {
    // Check if this page came from acvps://
    chrome.storage.local.get([`tab_${chrome.devtools?.tabId}`], (result) => {
      if (result[`tab_${chrome.devtools?.tabId}`]) {
        showACVPSIndicator();
      }
    });
  }
  
})();

