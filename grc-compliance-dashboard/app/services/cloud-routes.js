/**
 * Cloud API route configuration.
 * Paths are loaded from environment variables so the public source
 * does not reveal the backend's internal URL structure.
 * Set these in your .env file (see .env.example).
 */

module.exports = {
  health:           process.env.CLOUD_PATH_HEALTH           || '/health',
  violations:       process.env.CLOUD_PATH_VIOLATIONS       || '/v1/violations',
  evidence:         process.env.CLOUD_PATH_EVIDENCE         || '/v1/evidence',
  requests:         process.env.CLOUD_PATH_REQUESTS         || '/v1/requests',
  guardrails:       process.env.CLOUD_PATH_GUARDRAILS       || '/v1/guardrails',
  driftStatus:      process.env.CLOUD_PATH_DRIFT            || '/v1/drift-status',
  exportOscal:      process.env.CLOUD_PATH_OSCAL            || '/v1/export/oscal',
  exportStix:       process.env.CLOUD_PATH_STIX             || '/v1/export/stix',
  taxiiDiscovery:   process.env.CLOUD_PATH_TAXII_DISCOVERY  || '/taxii2/',
  taxiiCollections: process.env.CLOUD_PATH_TAXII_COLLECTIONS || '/taxii2/collections/',
  taxiiObjects:     process.env.CLOUD_PATH_TAXII_OBJECTS    || '/taxii2/collections',
};
