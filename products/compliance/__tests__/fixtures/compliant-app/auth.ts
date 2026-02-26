import express from 'express';
import { authenticateToken } from './middleware.js';

const app = express();

// Auth middleware on routes
app.get('/api/users', authenticateToken, (req, res) => {
  res.json({ users: [] });
});

app.post('/api/admin/delete', authenticateToken, (req, res) => {
  res.json({ deleted: true });
});
