// Empty catch blocks
async function riskyOperation() {
  try {
    await fetch('/api/data');
  } catch (err) {
  }
}

// Stack trace exposed
function errorHandler(err: Error, req: any, res: any) {
  res.status(500).json({ error: err.message, stack: err.stack });
}
