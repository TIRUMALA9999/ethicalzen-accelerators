// ACVPS Popup Script

document.addEventListener('DOMContentLoaded', () => {
  // Get current tab info
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs[0]) {
      const url = new URL(tabs[0].url);
      document.getElementById('current-page').textContent = url.hostname;
      
      // Check if contract exists
      checkContract(url.hostname);
    }
  });
  
  // Test ACVPS URL button
  document.getElementById('test-acvps').addEventListener('click', () => {
    chrome.tabs.create({ url: 'acvps://api.example.com/test' });
  });
  
  // Open dashboard button
  document.getElementById('open-dashboard').addEventListener('click', () => {
    chrome.tabs.create({ url: 'http://localhost:8000/dashboard.html' });
  });
});

async function checkContract(domain) {
  try {
    const response = await fetch(`http://localhost:8080/api/dc/contracts/lookup?domain=${domain}`);
    if (response.ok) {
      const contract = await response.json();
      document.getElementById('contract-status').textContent = contract.id || 'Found';
      document.getElementById('status').classList.add('active');
    } else {
      document.getElementById('contract-status').textContent = 'Not found';
    }
  } catch (error) {
    document.getElementById('contract-status').textContent = 'None';
  }
}

