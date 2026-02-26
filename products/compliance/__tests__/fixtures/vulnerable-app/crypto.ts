import crypto from 'crypto';

// Weak hashing
const hash = crypto.createHash('md5').update('password').digest('hex');
const sha1Hash = crypto.createHash('sha1').update('data').digest('hex');

// ECB mode
const cipher = crypto.createCipheriv('aes-128-ecb', Buffer.alloc(16), null);

// Hardcoded IV
const iv = Buffer.from('1234567890123456');
const cipher2 = crypto.createCipheriv('aes-256-cbc', Buffer.alloc(32), iv);
