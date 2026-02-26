import mysql from 'mysql';
const connection = mysql.createConnection({ host: 'localhost' });

// Parameterized queries
function getUser(userId: string) {
  return connection.query('SELECT * FROM users WHERE id = ?', [userId]);
}
