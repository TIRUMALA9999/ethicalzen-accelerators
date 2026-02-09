/**
 * Background Violation Poller + SSE Broadcast
 * Polls the cloud API for new violations and pushes them to connected SSE clients.
 */

class ViolationPoller {
  constructor(apiClient, cacheStore = null) {
    this.apiClient = apiClient;
    this.cacheStore = cacheStore;
    this.clients = new Set();
    this.interval = null;
    this.lastTimestamp = new Date().toISOString();
    this.pollIntervalMs = parseInt(process.env.POLL_INTERVAL_MS) || 10000;
    this.isPolling = false;
  }

  start() {
    if (this.interval) return;
    console.log(`[Poller] Starting violation poll every ${this.pollIntervalMs}ms`);
    this.interval = setInterval(() => this.poll(), this.pollIntervalMs);
    this.poll(); // immediate first poll
  }

  stop() {
    if (this.interval) {
      clearInterval(this.interval);
      this.interval = null;
    }
  }

  addClient(res) {
    this.clients.add(res);
    res.on('close', () => this.clients.delete(res));
  }

  broadcast(event, data) {
    const payload = `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
    for (const client of this.clients) {
      try {
        client.write(payload);
      } catch {
        this.clients.delete(client);
      }
    }
  }

  async poll() {
    if (this.isPolling) return;
    this.isPolling = true;

    try {
      const result = await this.apiClient.getViolations({
        from: this.lastTimestamp,
        limit: 50
      });

      const violations = Array.isArray(result) ? result : (result.violations || result.data || []);

      if (violations.length > 0) {
        // Update last timestamp
        const newest = violations.reduce((max, v) =>
          v.timestamp > max ? v.timestamp : max, this.lastTimestamp);
        this.lastTimestamp = newest;

        // Cache violations
        if (this.cacheStore) {
          this.cacheStore.cacheViolations(violations);
        }

        // Broadcast to SSE clients
        for (const v of violations) {
          this.broadcast('violation', v);
        }

        this.broadcast('stats', {
          newCount: violations.length,
          totalClients: this.clients.size,
          lastPoll: new Date().toISOString()
        });
      }

      // Send heartbeat
      this.broadcast('heartbeat', { time: new Date().toISOString() });
    } catch (err) {
      console.warn(`[Poller] Poll failed: ${err.message}`);
      this.broadcast('error', { message: 'Poll failed', time: new Date().toISOString() });
    } finally {
      this.isPolling = false;
    }
  }

  getStats() {
    return {
      connectedClients: this.clients.size,
      lastPoll: this.lastTimestamp,
      pollIntervalMs: this.pollIntervalMs,
      running: !!this.interval
    };
  }
}

module.exports = { ViolationPoller };
