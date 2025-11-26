/**
 * Database Abstraction Layer
 * Supports both SQLite (local) and PostgreSQL (production)
 */

const DB_TYPE = process.env.DB_TYPE || 'sqlite';

let db = null;

function initDB() {
    if (db) return db;

    if (DB_TYPE === 'sqlite') {
        const Database = require('better-sqlite3');
        const dbPath = process.env.SQLITE_PATH || './data/metrics.db';
        db = new Database(dbPath);
        db.pragma('journal_mode = WAL'); // Better concurrency
        console.log(`✅ Connected to SQLite: ${dbPath}`);
    } else if (DB_TYPE === 'postgres') {
        const { Pool } = require('pg');
        db = new Pool({
            host: process.env.PG_HOST,
            port: process.env.PG_PORT,
            user: process.env.PG_USER,
            password: process.env.PG_PASSWORD,
            database: process.env.PG_DATABASE,
            max: 20,
        });
        console.log('✅ Connected to PostgreSQL pool');
    }

    return db;
}

/**
 * Execute a query (abstracted for SQLite vs PostgreSQL)
 */
async function query(sql, params = []) {
    const database = initDB();

    if (DB_TYPE === 'sqlite') {
        // SQLite uses different param style
        const stmt = database.prepare(sql);
        
        if (sql.trim().toUpperCase().startsWith('SELECT')) {
            return stmt.all(...params);
        } else {
            const info = stmt.run(...params);
            return { rowCount: info.changes, lastInsertRowid: info.lastInsertRowid };
        }
    } else {
        // PostgreSQL
        const result = await database.query(sql, params);
        return result.rows || result;
    }
}

/**
 * Execute a single query and return first row
 */
async function queryOne(sql, params = []) {
    const database = initDB();

    if (DB_TYPE === 'sqlite') {
        const stmt = database.prepare(sql);
        return stmt.get(...params);
    } else {
        const result = await database.query(sql, params);
        return result.rows[0];
    }
}

/**
 * Begin a transaction
 */
async function beginTransaction() {
    const database = initDB();
    
    if (DB_TYPE === 'sqlite') {
        database.prepare('BEGIN').run();
    } else {
        await database.query('BEGIN');
    }
}

/**
 * Commit a transaction
 */
async function commitTransaction() {
    const database = initDB();
    
    if (DB_TYPE === 'sqlite') {
        database.prepare('COMMIT').run();
    } else {
        await database.query('COMMIT');
    }
}

/**
 * Rollback a transaction
 */
async function rollbackTransaction() {
    const database = initDB();
    
    if (DB_TYPE === 'sqlite') {
        database.prepare('ROLLBACK').run();
    } else {
        await database.query('ROLLBACK');
    }
}

module.exports = {
    initDB,
    query,
    queryOne,
    beginTransaction,
    commitTransaction,
    rollbackTransaction,
    getDB: () => db,
};

