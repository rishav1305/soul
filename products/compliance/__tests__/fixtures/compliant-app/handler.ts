// Proper error handling
async function riskyOperation() {
  try {
    await fetch('/api/data');
  } catch (err) {
    console.error('Operation failed:', err);
    throw new Error('Operation failed');
  }
}

function errorHandler(err: Error, req: any, res: any) {
  console.error(err);
  res.status(500).json({ error: 'Internal server error' });
}
