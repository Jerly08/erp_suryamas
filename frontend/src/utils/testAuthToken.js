/**
 * Browser-console checks for the HttpOnly cookie auth flow.
 */

import { clearAuthTokens, getAuthHeaders, getAuthToken, isAuthenticated } from './authTokenUtils';

const legacyStorageKeys = ['token', 'authToken', 'access_token', 'refreshToken', 'user'];
const legacySessionTokenKey = 'token';

export function runAuthTokenTests() {
  console.log('Starting HttpOnly cookie auth utility checks...');

  try {
    getAuthToken();
    console.log('FAIL: getAuthToken should not expose a browser-readable JWT');
  } catch (error) {
    console.log('PASS: getAuthToken blocks browser-readable JWT access:', error.message);
  }

  const optionalToken = getAuthToken(false);
  console.log(optionalToken === null
    ? 'PASS: getAuthToken(false) returns null'
    : `FAIL: getAuthToken(false) returned ${optionalToken}`);

  const headers = getAuthHeaders();
  console.log(!headers.Authorization
    ? 'PASS: getAuthHeaders does not add Authorization'
    : 'FAIL: getAuthHeaders should not add Authorization');
  console.log(headers['Content-Type'] === 'application/json'
    ? 'PASS: getAuthHeaders includes Content-Type'
    : 'FAIL: getAuthHeaders is missing Content-Type');

  console.log(isAuthenticated() === false
    ? 'PASS: isAuthenticated does not infer auth from readable token storage'
    : 'FAIL: isAuthenticated should not use readable token storage');

  legacyStorageKeys.forEach((key) => {
    window.localStorage.setItem(key, 'legacy-test-value');
  });
  window.sessionStorage.setItem(legacySessionTokenKey, 'legacy-test-value');

  clearAuthTokens();

  const allCleared = legacyStorageKeys.every((key) => !window.localStorage.getItem(key))
    && !window.sessionStorage.getItem(legacySessionTokenKey);
  console.log(allCleared
    ? 'PASS: clearAuthTokens removes legacy visible auth storage'
    : 'FAIL: clearAuthTokens left legacy visible auth storage behind');

  console.log('HttpOnly cookie auth utility checks completed.');
}

if (typeof window !== 'undefined') {
  window.runAuthTokenTests = runAuthTokenTests;
  console.log('Auth token test utility loaded. Run window.runAuthTokenTests() to test.');
}
