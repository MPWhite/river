#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const binDir = path.join(__dirname, '..', 'bin');
const binaryName = 'river' + (process.platform === 'win32' ? '.exe' : '');
const binaryPath = path.join(binDir, binaryName);

// Clean up binary
if (fs.existsSync(binaryPath)) {
  try {
    fs.unlinkSync(binaryPath);
    console.log('River binary removed.');
  } catch (err) {
    console.error('Failed to remove binary:', err.message);
  }
}