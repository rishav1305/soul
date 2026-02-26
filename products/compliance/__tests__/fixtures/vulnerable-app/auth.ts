import express from 'express';

const app = express();

// No auth middleware — routes are unprotected
app.get('/api/users', (req, res) => {
  res.json({ users: [] });
});

app.post('/api/admin/delete', (req, res) => {
  res.json({ deleted: true });
});

// No session timeout
app.use(require('express-session')({ secret: 'keyboard cat' }));
