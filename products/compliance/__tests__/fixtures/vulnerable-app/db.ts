import mysql from 'mysql';

const connection = mysql.createConnection({ host: 'localhost' });

// SQL injection — string concatenation
function getUser(userId: string) {
  const query = "SELECT * FROM users WHERE id = '" + userId + "'";
  return connection.query(query);
}

function searchUsers(name: string) {
  return connection.query(`SELECT * FROM users WHERE name = '${name}'`);
}
