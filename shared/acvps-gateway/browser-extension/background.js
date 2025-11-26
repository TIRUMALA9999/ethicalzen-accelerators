// ACVPS Browser Extension - Background Service Worker (Fixed)

console.log('🔒 ACVPS Protocol Handler loaded');

// Listen for tab updates to detect acvps:// URLs in address bar
chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.url && changeInfo.url.startsWith('acvps://')) {
    console.log('🔒 ACVPS URL detected in address bar:', changeInfo.url);
    
    // Parse the URL
    const acvpsUrl = changeInfo.url.replace('acvps://', '');
    const isLocalhost = acvpsUrl.includes('localhost') || acvpsUrl.includes('127.0.0.1');
    
    // Convert to HTTP or HTTPS
    const targetUrl = isLocalhost 
      ? `http://${acvpsUrl}` 
      : `https://${acvpsUrl}`;
    
    console.log('→ Redirecting to:', targetUrl);
    
    // Redirect to target URL
    chrome.tabs.update(tabId, { url: targetUrl });
    
    // Store that this tab should have DC headers
    chrome.storage.local.set({ [`tab_${tabId}`]: true });
  }
});

// Listen for navigation events to intercept acvps:// before navigation
chrome.webNavigation.onBeforeNavigate.addListener((details) => {
  if (details.url && details.url.startsWith('acvps://')) {
    console.log('🔒 Intercepting ACVPS navigation:', details.url);
  }
});

// Handle messages from content script
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.type === 'GET_CONTRACT') {
    // Fetch contract from DC Control Plane (or return demo contract)
    fetchContract(request.domain)
      .then(contract => sendResponse({ success: true, contract }))
      .catch(error => sendResponse({ success: false, error: error.message }));
    return true; // Async response
  }
  
  if (request.type === 'LOG') {
    console.log('[Content Script]:', request.message);
  }
});

// Fetch contract from DC Control Plane (demo mode)
async function fetchContract(domain) {
  try {
    const response = await fetch(`http://localhost:8080/api/dc/contracts/lookup?domain=${domain}`);
    if (!response.ok) {
      throw new Error('Contract not found');
    }
    return await response.json();
  } catch (error) {
    console.log('Using demo contract (DC Control Plane not available)');
    // Return default demo contract
    return {
      id: `${domain}/demo/v1.0`,
      policy_digest: 'sha256-demo123',
      suite: 'S1'
    };
  }
}

// Update extension icon based on validation status
function updateIcon(tabId, status) {
  const icons = {
    valid: 'icons/icon-valid.png',
    invalid: 'icons/icon-invalid.png',
    default: 'icons/icon16.png'
  };
  
  chrome.action.setIcon({
    tabId: tabId,
    path: icons[status] || icons.default
  }).catch(() => {
    // Ignore icon errors if files don't exist
  });
}

console.log('✅ ACVPS Background worker ready');
