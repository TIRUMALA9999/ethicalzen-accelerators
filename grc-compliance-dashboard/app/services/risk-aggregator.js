/**
 * Risk Aggregator
 * Computes overall and per-framework risk scores from violations and evidence.
 */

const FRAMEWORKS = {
  nist_ai_rmf: 'NIST AI Risk Management Framework',
  iso_42001: 'ISO/IEC 42001 AI Management System',
  nist_csf: 'NIST Cybersecurity Framework 2.0'
};

class RiskAggregator {

  computeRisk(violations = [], evidence = [], driftAlerts = []) {
    const now = Date.now();
    const h24 = 24 * 3600 * 1000;

    // Filter to last 24h
    const recent = violations.filter(v =>
      new Date(v.timestamp).getTime() > now - h24
    );

    // Violation frequency score (0-40 points)
    const freqScore = Math.min(40, recent.length * 2);

    // Severity score (0-30 points) — blocked = high severity
    const blocked = recent.filter(v => v.status === 'blocked').length;
    const sevScore = recent.length > 0
      ? Math.min(30, (blocked / recent.length) * 30)
      : 0;

    // Drift score (0-30 points) — active drift alerts
    const activeDrift = Array.isArray(driftAlerts)
      ? driftAlerts.filter(d => d.status === 'active').length
      : 0;
    const driftScore = Math.min(30, activeDrift * 10);

    const overall = Math.round(freqScore + sevScore + driftScore);

    return {
      overall: Math.min(100, overall),
      zone: this.getZone(overall),
      breakdown: {
        violationFrequency: Math.round(freqScore),
        severity: Math.round(sevScore),
        drift: Math.round(driftScore)
      },
      violations24h: recent.length,
      blocked24h: blocked,
      blockRate: recent.length > 0 ? (blocked / recent.length * 100).toFixed(1) : '0.0',
      activeDriftAlerts: activeDrift,
      frameworks: this.computeFrameworkRisk(violations),
      computedAt: new Date().toISOString()
    };
  }

  computeFrameworkRisk(violations = []) {
    const result = {};
    for (const [key, name] of Object.entries(FRAMEWORKS)) {
      // Simple heuristic: framework risk = violation density mapped to 0-100
      const count = violations.length;
      const score = Math.min(100, Math.round(count * 1.5));
      result[key] = {
        name,
        score,
        zone: this.getZone(score),
        controlsCovered: 0,  // Will be enriched when compliance data available
        totalControls: 0
      };
    }
    return result;
  }

  getZone(score) {
    if (score <= 30) return 'low';
    if (score <= 60) return 'moderate';
    if (score <= 80) return 'high';
    return 'critical';
  }

  getZoneColor(zone) {
    const colors = { low: '#10b981', moderate: '#f59e0b', high: '#f97316', critical: '#ef4444' };
    return colors[zone] || '#94a3b8';
  }
}

module.exports = { RiskAggregator, FRAMEWORKS };
