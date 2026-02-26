// Properly secured — test fixture
const DB_PASSWORD = process.env.DB_PASSWORD;
const API_KEY = process.env.API_KEY;

export function getConnection() {
  return { host: process.env.DB_HOST, password: DB_PASSWORD };
}
