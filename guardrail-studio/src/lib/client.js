/**
 * EthicalZen Guardrail Client
 * 
 * A thin API client for interacting with EthicalZen's guardrail services.
 * This client provides a clean interface without exposing any internal implementation details.
 * 
 * @example
 * ```javascript
 * import { EthicalZen } from '@ethicalzen/guardrail-sdk';
 * 
 * const client = new EthicalZen({ apiKey: 'sk-...' });
 * const result = await client.evaluate('financial_advice_smart', 'Should I buy Bitcoin?');
 * console.log(result.decision); // 'block'
 * ```
 */

const DEFAULT_API_BASE = 'https://api.ethicalzen.ai/v1';

/**
 * EthicalZen API Client
 */
export class EthicalZen {
    /**
     * Create a new EthicalZen client
     * @param {Object} config - Configuration options
     * @param {string} config.apiKey - Your EthicalZen API key
     * @param {string} [config.apiBase] - Optional custom API base URL
     */
    constructor({ apiKey, apiBase = DEFAULT_API_BASE }) {
        if (!apiKey) {
            throw new Error('API key is required. Get one at https://ethicalzen.ai/signup');
        }
        this.apiKey = apiKey;
        this.apiBase = apiBase.replace(/\/$/, ''); // Remove trailing slash
    }

    /**
     * Evaluate an input against a guardrail
     * @param {string} guardrailId - The ID of the guardrail to evaluate against
     * @param {string} input - The text input to evaluate
     * @returns {Promise<EvaluationResult>} The evaluation result
     */
    async evaluate(guardrailId, input) {
        const response = await this._request('POST', '/evaluate', {
            guardrail: guardrailId,
            input
        });
        return response;
    }

    /**
     * Design a new guardrail from a natural language description
     * @param {Object} params - Design parameters
     * @param {string} params.description - Natural language description of what to block
     * @param {string[]} [params.safeExamples] - Examples of allowed content
     * @param {string[]} [params.unsafeExamples] - Examples of blocked content
     * @returns {Promise<GuardrailConfig>} The generated guardrail configuration
     */
    async design({ description, safeExamples = [], unsafeExamples = [] }) {
        const response = await this._request('POST', '/guardrails/design', {
            naturalLanguage: description,
            safeExamples,
            unsafeExamples
        });
        return response;
    }

    /**
     * List available guardrails
     * @returns {Promise<Guardrail[]>} List of available guardrails
     */
    async listGuardrails() {
        const response = await this._request('GET', '/guardrails');
        return response.guardrails || [];
    }

    /**
     * Get a specific guardrail by ID
     * @param {string} guardrailId - The guardrail ID
     * @returns {Promise<Guardrail>} The guardrail details
     */
    async getGuardrail(guardrailId) {
        const response = await this._request('GET', `/guardrails/${guardrailId}`);
        return response;
    }

    /**
     * Test a guardrail with multiple inputs
     * @param {string} guardrailId - The guardrail ID
     * @param {Object} testData - Test data
     * @param {string[]} testData.safeInputs - Inputs expected to be allowed
     * @param {string[]} testData.unsafeInputs - Inputs expected to be blocked
     * @returns {Promise<TestResult>} Test results with accuracy metrics
     */
    async test(guardrailId, { safeInputs = [], unsafeInputs = [] }) {
        const response = await this._request('POST', `/guardrails/${guardrailId}/test`, {
            safeInputs,
            unsafeInputs
        });
        return response;
    }

    /**
     * Export a guardrail configuration
     * @param {string} guardrailId - The guardrail ID
     * @param {string} [format='yaml'] - Export format ('yaml' or 'json')
     * @returns {Promise<string>} The exported configuration
     */
    async export(guardrailId, format = 'yaml') {
        const response = await this._request('GET', `/guardrails/${guardrailId}/export?format=${format}`);
        return response;
    }

    /**
     * Internal method to make API requests
     * @private
     */
    async _request(method, path, body = null) {
        const url = `${this.apiBase}${path}`;
        const options = {
            method,
            headers: {
                'X-API-Key': this.apiKey,
                'Content-Type': 'application/json',
                'User-Agent': 'ethicalzen-sdk/1.0.0'
            }
        };

        if (body && method !== 'GET') {
            options.body = JSON.stringify(body);
        }

        try {
            const response = await fetch(url, options);
            const data = await response.json();

            if (!response.ok) {
                throw new EthicalZenError(
                    data.error || `Request failed with status ${response.status}`,
                    response.status,
                    data
                );
            }

            return data;
        } catch (error) {
            if (error instanceof EthicalZenError) {
                throw error;
            }
            throw new EthicalZenError(`Network error: ${error.message}`, 0, null);
        }
    }
}

/**
 * Custom error class for EthicalZen API errors
 */
export class EthicalZenError extends Error {
    constructor(message, status, data) {
        super(message);
        this.name = 'EthicalZenError';
        this.status = status;
        this.data = data;
    }
}

// Type definitions for better IDE support
/**
 * @typedef {Object} EvaluationResult
 * @property {'allow'|'block'|'review'} decision - The guardrail decision
 * @property {number} score - Confidence score (0-1)
 * @property {string} guardrailId - The guardrail that was evaluated
 */

/**
 * @typedef {Object} GuardrailConfig
 * @property {string} id - Unique guardrail identifier
 * @property {string} name - Human-readable name
 * @property {string} description - What this guardrail does
 * @property {Object} calibration - Calibration parameters
 * @property {number} calibration.tAllow - Threshold for allowing content
 * @property {number} calibration.tBlock - Threshold for blocking content
 */

/**
 * @typedef {Object} Guardrail
 * @property {string} id - Unique identifier
 * @property {string} name - Human-readable name
 * @property {string} type - Guardrail type
 * @property {number} accuracy - Current accuracy percentage
 */

/**
 * @typedef {Object} TestResult
 * @property {number} accuracy - Overall accuracy (0-1)
 * @property {number} precision - Precision score (0-1)
 * @property {number} recall - Recall score (0-1)
 * @property {number} f1 - F1 score (0-1)
 * @property {Object[]} results - Individual test results
 */

// Default export
export default EthicalZen;

