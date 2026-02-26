import bcrypt from 'bcrypt';
import crypto from 'crypto';

// Strong hashing
const hash = await bcrypt.hash('password', 12);

// Proper encryption
const iv = crypto.randomBytes(16);
const cipher = crypto.createCipheriv('aes-256-gcm', crypto.randomBytes(32), iv);
