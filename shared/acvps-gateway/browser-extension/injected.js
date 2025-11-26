// ACVPS Injected Script - Demo mode (works without backend)

(function() {
  'use strict';
  
  console.log('🔗 ACVPS fetch interceptor installed (DEMO MODE)');
  
  // Store original fetch
  const originalFetch = window.fetch;
  
  // Override fetch to add DC headers automatically
  window.fetch = function(resource, init = {}) {
    const url = typeof resource === 'string' ? resource : resource.url;
    
    // Check if this should use ACVPS
    if (url.startsWith('acvps://')) {
      console.log('🔒 ACVPS URL detected:', url);
      
      // Check if it's localhost and use HTTP instead of HTTPS
      const acvpsUrl = url.replace('acvps://', '');
      const isLocalhost = acvpsUrl.includes('localhost') || acvpsUrl.includes('127.0.0.1');
      const targetUrl = isLocalhost ? `http://${acvpsUrl}` : `https://${acvpsUrl}`;
      
      console.log('→ Converting to:', targetUrl);
      
      // Add DC headers (demo values - in production these come from blockchain)
      const headers = new Headers(init.headers || {});
      headers.set('X-DC-Id', 'demo-service/healthcare/us/v1.0');
      headers.set('X-DC-Digest', 'sha256-abc123demo');
      headers.set('X-DC-Suite', 'S1');
      headers.set('X-DC-Profile', 'balanced');
      headers.set('X-Protocol', 'acvps');
      
      console.log('✅ Added ACVPS headers:', {
        'X-DC-Id': headers.get('X-DC-Id'),
        'X-DC-Digest': headers.get('X-DC-Digest'),
        'X-DC-Suite': headers.get('X-DC-Suite'),
        'X-Protocol': headers.get('X-Protocol')
      });
      
      // Make the actual request with DC headers
      return originalFetch(targetUrl, {
        ...init,
        headers
      }).then(response => {
        console.log('✅ ACVPS request succeeded:', targetUrl, response.status);
        return response;
      }).catch(error => {
        console.error('❌ ACVPS request failed:', error);
        throw error;
      });
    }
    
    // Normal fetch for non-ACVPS URLs
    return originalFetch(resource, init);
  };
  
  // Also override XMLHttpRequest for completeness
  const OriginalXHR = window.XMLHttpRequest;
  window.XMLHttpRequest = function() {
    const xhr = new OriginalXHR();
    const originalOpen = xhr.open;
    
    xhr.open = function(method, url, ...args) {
      if (url.startsWith('acvps://')) {
        console.log('🔒 ACVPS XHR detected:', url);
        url = url.replace('acvps://', 'https://');
        xhr.setRequestHeader('X-Protocol', 'acvps');
        xhr.setRequestHeader('X-DC-Id', 'demo-service/healthcare/us/v1.0');
      }
      return originalOpen.apply(this, [method, url, ...args]);
    };
    
    return xhr;
  };
  
  console.log('✅ ACVPS protocol handlers installed (DEMO MODE - no backend required)');
  
})();
