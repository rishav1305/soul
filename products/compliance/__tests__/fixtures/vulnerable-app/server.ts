// Intentionally insecure — test fixture
const AWS_ACCESS_KEY = 'AKIAIOSFODNN7EXAMPLE';
const AWS_SECRET_KEY = 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY';
const GITHUB_TOKEN = 'ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef12';
const DB_PASSWORD = 'password = "super_secret_123"';
const PRIVATE_KEY = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHB7MhgHcTz6sE2I2yPB
-----END RSA PRIVATE KEY-----`;
const JWT_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U';
const SLACK_TOKEN = 'xoxb-123456789012-1234567890123-abcdefghijklmnopqrstuvwx';

export function getConnection() {
  return { host: 'db.example.com', password: DB_PASSWORD };
}
